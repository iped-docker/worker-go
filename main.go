package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
)

//go:generate go run scripts/generate.go

func assertEnv(key string) string {
	data := os.Getenv(key)
	if data == "" {
		log.Fatal("environment variable not set: ", key)
	}
	return data
}

func main() {
	jar := assertEnv("IPEDJAR")
	locker := remoteLocker{
		URL: assertEnv("LOCK_URL"),
	}
	notifierURL := assertEnv("NOTIFY_URL")
	PORT, ok := os.LookupEnv("PORT")
	if !ok {
		PORT = "80"
	}
	watchURL, isWatching := os.LookupEnv("WATCH_URL")

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

	if isWatching {
		go http.ListenAndServe(fmt.Sprintf(":%s", PORT), router)

		watch(watchURL, jar, &locker, notifierURL)
	} else {
		router.HandleFunc("/start",
			start(jar, &locker, notifierURL)).Methods("POST")
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
				w.Write([]byte(swagger_content))
			})

		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", PORT), router))
	}

}

func watch(watchURL, jar string, locker *remoteLocker, notifierURL string) {
	for i := 0; i < 2*6*60; i++ {
		r, err := http.Get(watchURL)
		if err != nil {
			log.Fatalf("could not watch URL: %v\n", err)
		}
		defer r.Body.Close()
		var payloads []todo
		err = json.NewDecoder(r.Body).Decode(&payloads)
		if err != nil {
			log.Fatalf("could not parse JSON: %v\n", err)
		}
		if len(payloads) < 1 {
			time.Sleep(5 * time.Second)
			continue
		}
		payload := payloads[0]
		params := ipedParams{
			jar:      jar,
			evidence: payload.EvidencePath,
			output:   payload.OutputPath,
			profile:  payload.Profile,
		}
		err = runIped(params, locker, notifierURL)
		if err != nil {
			log.Fatalf("error: %v\n", err)
		}
		break
	}
}

func start(jar string, locker *remoteLocker, notifierURL string) func(w http.ResponseWriter, r *http.Request) {
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
			jar:      jar,
			evidence: payload.EvidencePath,
			output:   payload.OutputPath,
			profile:  payload.Profile,
		}
		result := make(chan error)
		go func() {
			result <- runIped(params, locker, notifierURL)
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
	EvidencePath string `json:"evidencePath,omitempty"`
	OutputPath   string `json:"outputPath,omitempty"`
	Profile      string `json:"profile,omitempty"`
}
