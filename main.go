package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

// our main function
func main() {
	router := mux.NewRouter()
	router.HandleFunc("/start", start).Methods("POST")

	PORT, ok := os.LookupEnv("PORT")
	if !ok {
		PORT = "8000"
	}
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", PORT), router))
}

type startPayload struct {
	EvidencePath string `json:"evidencePath,omitempty"`
	OutputPath   string `json:"outputPath,omitempty"`
}

func start(w http.ResponseWriter, r *http.Request) {
	var payload startPayload
	_ = json.NewDecoder(r.Body).Decode(&payload)
	params := IpedParams{
		evidence: payload.EvidencePath,
		output:   payload.OutputPath,
	}
	err := <-runIped(params)
	if err != nil {
		w.WriteHeader(http.StatusLocked)
		fmt.Fprintf(w, "Error: %s", err.Error())
		return
	}
	fmt.Fprintf(w, "{\"status\":\"started\"}")
}
