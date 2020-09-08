package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ServeOptions has options for Serve()
type ServeOptions struct {
	locker       *remoteLocker
	PORT         string
	notifierURL  string
	jar          string
	shouldWatch  bool
	watchURL     string
	shouldListen bool
	hasJob       bool
}

// Serve creates the web server
func Serve(options ServeOptions, job Job) {
	metrics := createIpedMetrics()
	router := mux.NewRouter()
	endpoints := []string{}

	router.HandleFunc("/healthz", healthz).Methods("GET")
	endpoints = append(endpoints, "/healthz")

	router.HandleFunc("/readiness", readiness(options)).Methods("GET")
	endpoints = append(endpoints, "/readiness")

	router.Handle("/metrics", promhttp.Handler())
	endpoints = append(endpoints, "/metrics")

	if options.shouldListen {
		router.HandleFunc("/start",
			func(w http.ResponseWriter, r *http.Request) {
				defer r.Body.Close()
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Header().Set("Access-Control-Allow-Methods", "POST")
				w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")
			}).Methods("OPTIONS")

		router.HandleFunc("/start",
			listenForJobs(options.jar, options.locker, options.notifierURL, metrics)).Methods("POST")
		endpoints = append(endpoints, "/start")
	}

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	srv := &http.Server{Addr: fmt.Sprintf(":%s", options.PORT), Handler: router}
	srv.Shutdown(ctx)
	go func() {
		log.Println("Listening on port", options.PORT)
		log.Println("Endpoints:")
		for _, x := range endpoints {
			log.Println(x)
		}
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			// unexpected error. port in use?
			log.Fatalf("ListenAndServe(): %v", err)
		}
	}()

	if options.hasJob {
		log.Printf("using single job: %s\n", job.EvidencePath)
		processPayloads(ctx, []Job{job}, options.jar, options.locker, options.notifierURL, metrics)
		cancel()
	}

	if options.shouldWatch {
		log.Printf("watching URL: %s\n", options.watchURL)
		go watch(ctx, options.watchURL, options.jar, options.locker, options.notifierURL, metrics)
	}
	<-ctx.Done()
}

func healthz(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	w.Write([]byte("ok"))
}

func readiness(options ServeOptions) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if options.locker.EvidencePath != "" {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	}
}

func listenForJobs(jar string, locker *remoteLocker, notifierURL string, metrics ipedMetrics) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		w.Header().Set("Access-Control-Allow-Origin", "*")
		var payload Job
		err := json.NewDecoder(r.Body).Decode(&payload)
		if err != nil {
			http.Error(w, fmt.Sprintf("could not parse JSON: %s", err.Error()), http.StatusBadRequest)
			return
		}
		params := ipedParams{
			jar:             jar,
			evidence:        payload.EvidencePath,
			output:          payload.OutputPath,
			profile:         payload.Profile,
			additionalArgs:  payload.AdditionalArgs,
			additionalPaths: payload.AdditionalPaths,
		}
		result := make(chan error)
		go func() {
			result <- runIped(params, locker, notifierURL, metrics)
		}()
		select {
		case err = <-result:
		case <-time.After(5 * time.Second):
			err = nil
		}
		if err != nil {
			http.Error(w, fmt.Sprintf("error : %s", err.Error()), http.StatusBadRequest)
			return
		}
		w.Write([]byte("{\"status\":\"started\"}"))
	}
}

func watch(ctx context.Context, watchURL, jar string, locker *remoteLocker, notifierURL string, metrics ipedMetrics) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			r, err := http.Get(watchURL)
			if err != nil {
				log.Fatalf("could not watch URL: %v\n", err)
			}
			defer r.Body.Close()
			var payloads []Job
			b, err := ioutil.ReadAll(r.Body)
			if err != nil {
				log.Fatalf("could not read watch body: %v\n", err)
			}
			err = json.Unmarshal(b, &payloads)
			if err != nil {
				log.Fatalf("could not parse JSON: %v; data: %v\n", err, string(b))
			}
			if len(payloads) < 1 {
				time.Sleep(5 * time.Second)
				continue
			}
			ctxPayloads, cancelPayload := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancelPayload()
			processPayloads(ctxPayloads, payloads, jar, locker, notifierURL, metrics)
			return
		}
	}
}

func processPayloads(ctx context.Context, payloads []Job, jar string, locker *remoteLocker, notifierURL string, metrics ipedMetrics) {
	var err error
	for _, payload := range payloads {
		select {
		case <-ctx.Done():
			if err != nil {
				log.Fatalf("error: %v\n", err)
			}
			return
		default:
			params := ipedParams{
				jar:             jar,
				evidence:        payload.EvidencePath,
				output:          payload.OutputPath,
				profile:         payload.Profile,
				additionalArgs:  payload.AdditionalArgs,
				additionalPaths: payload.AdditionalPaths,
			}
			err = runIped(params, locker, notifierURL, metrics)
			if err != nil {
				fmt.Printf("error: %v\n", err)
			}
		}
	}
}

type Job struct {
	EvidencePath    string `json:"evidencePath,omitempty"`
	OutputPath      string `json:"outputPath,omitempty"`
	Profile         string `json:"profile,omitempty"`
	AdditionalArgs  string `json:"additionalArgs,omitempty"`
	AdditionalPaths string `json:"additionalPaths,omitempty"`
}
