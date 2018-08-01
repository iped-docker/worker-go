package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"sync"
	"time"
)

type childStruct struct {
	lock sync.Mutex
	cmd  *exec.Cmd
}

var child childStruct

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
	eWriter := eventWriter{
		EvidencePath: params.evidence,
		URL:          notifierURL,
		Writer:       os.Stdout,
		events:       events,
	}
	child.cmd = exec.Command("java", args...)
	child.cmd.Dir = path.Dir(params.evidence)
	child.cmd.Stdout = eWriter
	child.cmd.Stderr = eWriter
	err = child.cmd.Start()
	if err != nil {
		return fmt.Errorf("error in execution: %v", err)
	}
	events <- event{
		Type: "running",
		Payload: eventPayload{
			EvidencePath: params.evidence,
		},
	}
	err = child.cmd.Wait()
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
