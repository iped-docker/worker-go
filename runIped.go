package main

import (
	"fmt"
	"log"
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
		"-XX:+UnlockExperimentalVMOptions",
		"-XX:+UseCGroupMemoryLimitForHeap",
		"-Xmx6G",
		"-jar", params.jar,
		"-d", path.Base(params.evidence),
		"-o", params.output,
		"--portable",
		"--nologfile",
		"--nogui",
	}
	if params.profile != "" {
		args = append(args, "-profile", params.profile)
	}
	ipedfolder := path.Join(path.Dir(params.evidence), "SARD")
	err = os.MkdirAll(ipedfolder, 0777)
	if err != nil {
		return err
	}
	err = os.Chmod(ipedfolder, 0770)
	if err != nil {
		return err
	}
	log, err := os.OpenFile(path.Join(ipedfolder, "IPED.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer log.Close()
	hostname, _ := os.Hostname()
	log.WriteString(fmt.Sprintf("HOSTNAME: %s\n", hostname))
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
	err = os.Chmod(ipedfolder, 0777)
	if err != nil {
		return err
	}
	permPaths := []string{
		"Ferramenta de Pesquisa.exe",
		"IPED-SearchApp.exe",
		"indexador/tools",
		"indexador/jre/bin",
		"indexador/lib",
	}
	var d string
	if path.IsAbs(params.output) {
		d = params.output
	} else {
		d = path.Join(ipedfolder, params.output)
	}
	for _, p := range permPaths {
		permissions(d, p)
	}
	return err
}

func permissions(dirPath string, targetPath string) {
	cmd := exec.Command("chmod", "-cR", "a+x", targetPath)
	cmd.Dir = dirPath
	out, err := cmd.CombinedOutput()
	log.Printf("%v", string(out))
	if err != nil {
		log.Printf("%v", err.Error())
	}
}
