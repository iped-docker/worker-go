package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ServeOptions has options for Serve()
type ServeOptions struct {
	locker      *remoteLocker
	PORT        string
	notifierURL string
	jar         string
}

// Serve creates the web server
func Serve(port string, locker *remoteLocker) context.Context {
	router := mux.NewRouter()
	endpoints := []string{}

	router.HandleFunc("/healthz", healthz).Methods("GET")
	endpoints = append(endpoints, "/healthz")

	router.HandleFunc("/readiness", readiness(locker)).Methods("GET")
	endpoints = append(endpoints, "/readiness")

	router.Handle("/metrics", promhttp.Handler())
	endpoints = append(endpoints, "/metrics")

	ctx := context.Background()
	srv := &http.Server{Addr: fmt.Sprintf(":%s", port), Handler: router}
	go func() {
		<-ctx.Done()
		srv.Shutdown(ctx)
	}()
	go func() {
		log.Println("Listening on port", port)
		log.Println("Endpoints:")
		for _, x := range endpoints {
			log.Println(x)
		}
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			// unexpected error. port in use?
			log.Fatalf("ListenAndServe(): %v", err)
		}
	}()
	return ctx
}

func healthz(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	w.Write([]byte("ok"))
}

func readiness(locker *remoteLocker) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if locker.EvidencePath != "" {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	}
}

func processPayloads(ctx context.Context, payloads []Job, jar string, locker *remoteLocker, notifierURL string) {
	metrics := createIpedMetrics()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
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
				mvPath:          payload.mvPath,
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
	MvPath 		string `json:"mvPath,omitempty"`
}
