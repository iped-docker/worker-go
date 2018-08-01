package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type event struct {
	Type    string       `json:"type"`
	Payload eventPayload `json:"payload"`
}

type eventPayload struct {
	EvidencePath string `json:"evidencePath"`
	Progress     string `json:"progress,omitempty"`
}

type eventWriter struct {
	URL          string
	EvidencePath string
	Writer       io.Writer
	events       chan event
}

func (r eventWriter) Write(p []byte) (int, error) {
	i, err := r.Writer.Write(p)
	if err != nil {
		ev := event{
			Type: "progress",
			Payload: eventPayload{
				EvidencePath: r.EvidencePath,
				Progress:     string(p[:i]),
			},
		}
		go func() {
			r.events <- ev
		}()
	}
	return i, err
}

func sendEvent(URL string, ev event) error {
	fmt.Printf("event: %v\n", ev)
	j, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	resp, err := http.Post(URL, "application/json", bytes.NewBuffer(j))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("response from remote locker not ok: %s", resp.Status)
		return fmt.Errorf("response from remote locker not ok: %s", resp.Status)
	}
	return nil
}
