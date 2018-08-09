package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"time"
)

type ipedParams struct {
	jar      string
	evidence string
	output   string
	profile  string
}

func runIped(params ipedParams, locker *remoteLocker, notifierURL string) error {
	err := locker.Lock(params.evidence)
	if err != nil {
		return err
	}
	defer locker.Unlock()
	events := make(chan event)
	defer close(events)
	go func() {
		last := time.Now()
		for ev := range events {
			if ev.Type == "progress" {
				if time.Since(last) < time.Second {
					continue
				}
				last = time.Now()
			}
			go func(ev event) {
				sendEvent(notifierURL, ev)
			}(ev)
		}
	}()
	args := []string{
		"-Djava.awt.headless=true",
		"-jar", params.jar,
		"-d", path.Base(params.evidence),
		"-o", params.output,
		"--portable",
		"--nologfile",
		"--nogui",
	}
	if params.profile != "" {
		args = append(args, "--profile", params.profile)
	}
	os.MkdirAll(path.Join(path.Dir(params.evidence), "SARD"), 0777)
	log, err := os.Create(path.Join(path.Dir(params.evidence), "SARD", "IPED.log"))
	if err != nil {
		return err
	}
	defer log.Close()
	dw := doubleWriter{
		Writer1: os.Stdout,
		Writer2: log,
	}
	eWriter := eventWriter{
		EvidencePath: params.evidence,
		URL:          notifierURL,
		Writer:       dw,
		events:       events,
	}
	cmd := exec.Command("java", args...)
	cmd.Dir = path.Dir(params.evidence)
	cmd.Stdout = eWriter
	cmd.Stderr = eWriter
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("error in execution: %v", err)
	}
	events <- event{
		Type: "running",
		Payload: eventPayload{
			EvidencePath: params.evidence,
		},
	}
	err = cmd.Wait()
	t := "done"
	if err != nil {
		t = "failed"
	}
	events <- event{
		Type: t,
		Payload: eventPayload{
			EvidencePath: params.evidence,
		},
	}
	return err
}
