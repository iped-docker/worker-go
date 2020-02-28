package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
)

//go:generate go run generate/main.go

func assertEnv(key string) string {
	data, ok := os.LookupEnv(key)
	if !ok {
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

		ctxWatch, _ := context.WithTimeout(context.Background(), 1*time.Hour)
		watch(ctxWatch, watchURL, jar, &locker, notifierURL)
	} else {
		router.HandleFunc("/start",
			listen(jar, &locker, notifierURL)).Methods("POST")
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

		log.Println("Listening on port", PORT)
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", PORT), router))
	}

}

func watch(ctx context.Context, watchURL, jar string, locker *remoteLocker, notifierURL string) {
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
			ctxPayloads, _ := context.WithTimeout(context.Background(), 60*time.Second)
			processPayloads(ctxPayloads, payloads, jar, locker, notifierURL)
			return
		}
	}
}

func processPayloads(ctx context.Context, payloads []todo, jar string, locker *remoteLocker, notifierURL string) {
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
				jar:      jar,
				evidence: payload.EvidencePath,
				output:   payload.OutputPath,
				profile:  payload.Profile,
				additionalArgs:  payload.AdditionalArgs,
				additionalPaths:  payload.AdditionalPaths,
			}
			err = runIped(params, locker, notifierURL)
			if err != nil {
				fmt.Printf("error: %v\n", err)
			}
		}
	}
}

func listen(jar string, locker *remoteLocker, notifierURL string) func(w http.ResponseWriter, r *http.Request) {
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
			additionalArgs:  payload.AdditionalArgs,
			additionalPaths:  payload.AdditionalPaths,
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
	AdditionalArgs      string `json:"additionalArgs,omitempty"`
	AdditionalPaths      string `json:"additionalPaths,omitempty"`
}
