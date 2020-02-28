package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"time"
	"strings"
)

type ipedParams struct {
	jar      string
	evidence string
	output   string
	profile  string
	additionalArgs string
	additionalPaths string
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
	if params.additionalArgs != "" {
		addArgsArray := strings.split(additionalArgs, " ")
		for  i := 0; i < len(addArgsArray); i++ {
			args = append(args, addArgsArray[i])			
		}
	}
	if params.additionalPaths != "" {
		addPathsArray := strings.split(additionalPaths, "\n")
		for  i := 0; i < len(addPathsArray); i++ {
			args = append(args, "-d", addPathsArray[i])			
		}
	}
		
	var ipedfolder string
	// ipedfolder is the absolute path of the target output folder
	// Ex: /data/mat1/SARD
	// params.output will usually be 'SARD', but it can be an absolute path
	if path.IsAbs(params.output) {
		ipedfolder = params.output
	} else {
		ipedfolder = path.Join(path.Dir(params.evidence), params.output)
	}
	err = os.MkdirAll(ipedfolder, 0755)
	if err != nil {
		return err
	}
	err = os.Chmod(ipedfolder, 0750)
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
	// since params.output will be usually a relative path,
	// like 'SARD', we execute the command at the folder of the evidence path
	// using cmd.Dir. It is important to keep the relative path on the option -d
	// but still have the absolute path in the var ipedfolder
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
	if err != nil {
		return err
	}
	err = os.Chmod(ipedfolder, 0755)
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

	for _, p := range permPaths {
		permissions(ipedfolder, p)
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
