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
	locker      *remoteLocker
	PORT        string
	watchURL    string
	isWatching  bool
	jar         string
	notifierURL string
}

// Serve creates the web server
func Serve(options ServeOptions) {
	if options.isWatching {
		router := WatchRouter(options.locker)
		go http.ListenAndServe(fmt.Sprintf(":%s", options.PORT), router)

		ctxWatch, ctxCancel := context.WithTimeout(context.Background(), 1*time.Hour)
		defer ctxCancel()
		watch(ctxWatch, options.watchURL, options.jar, options.locker, options.notifierURL)
	} else {
		router := LazyRouter(options.locker, options.notifierURL, options.jar)
		log.Println("Listening on port", options.PORT)
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", options.PORT), router))
	}
}

// WatchRouter returns a router that actively looks for jobs
func WatchRouter(locker *remoteLocker) *mux.Router {
	router := mux.NewRouter()

	router.HandleFunc("/healthz",
		func(w http.ResponseWriter, r *http.Request) {
			defer r.Body.Close()
			w.Write([]byte("ok"))
		}).Methods("GET")

	router.HandleFunc("/readiness",
		func(w http.ResponseWriter, r *http.Request) {
			defer r.Body.Close()
			if locker.EvidencePath != "" {
				http.Error(w, "not ready", http.StatusServiceUnavailable)
				return
			}
			w.Write([]byte("ok"))
		}).Methods("GET")

	router.Handle("/metrics", promhttp.Handler())

	return router
}

// LazyRouter returns a router that receives jobs via web API. It uses WatchRouter
func LazyRouter(locker *remoteLocker, notifierURL string, jar string) *mux.Router {
	router := WatchRouter(locker)

	router.HandleFunc("/start",
		listen(jar, locker, notifierURL)).Methods("POST")

	router.HandleFunc("/start",
		func(w http.ResponseWriter, r *http.Request) {
			defer r.Body.Close()
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST")
			w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")
		}).Methods("OPTIONS")

	router.HandleFunc("/swagger.json",
		func(w http.ResponseWriter, r *http.Request) {
			defer r.Body.Close()
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Write([]byte(generatedSwagger))
		})
	return router
}

func watch(ctx context.Context, watchURL, jar string, locker *remoteLocker, notifierURL string) {
	metrics := createIpedMetrics()
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
			var payloads []todo
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

func processPayloads(ctx context.Context, payloads []todo, jar string, locker *remoteLocker, notifierURL string, metrics ipedMetrics) {
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

func listen(jar string, locker *remoteLocker, notifierURL string) func(w http.ResponseWriter, r *http.Request) {
	metrics := createIpedMetrics()

	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		w.Header().Set("Access-Control-Allow-Origin", "*")
		var payload todo
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

type todo struct {
	EvidencePath    string `json:"evidencePath,omitempty"`
	OutputPath      string `json:"outputPath,omitempty"`
	Profile         string `json:"profile,omitempty"`
	AdditionalArgs  string `json:"additionalArgs,omitempty"`
	AdditionalPaths string `json:"additionalPaths,omitempty"`
}
