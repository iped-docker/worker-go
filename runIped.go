package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
)

type ipedParams struct {
	jar             string
	evidence        string
	output          string
	profile         string
	additionalArgs  string
	additionalPaths string
}

func runIped(params ipedParams, locker *remoteLocker, notifierURL string, metrics ipedMetrics) (finalError error) {
	hostname, _ := os.Hostname()
	metrics.calls.WithLabelValues(hostname, params.evidence).Inc()
	metrics.running.WithLabelValues(hostname, params.evidence).Set(0)

	return withLocker(params, locker, metrics, func() error {
		logWriter, err := makeLogWriter(params, notifierURL, metrics)
		if err != nil {
			return err
		}

		ipedfolder, err := makeIpedFolder(params)
		if err != nil {
			return err
		}

		err = sendEvent(notifierURL, event{
			Type: "running",
			Payload: eventPayload{
				EvidencePath: params.evidence,
			},
		})
		if err != nil {
			return fmt.Errorf("could not set status to 'running': %v", err)
		}

		errCmd := coreRun(params, logWriter)

		finalStatus := "done"
		if errCmd != nil {
			finalStatus = "failed"
		}
		err = sendEvent(notifierURL, event{
			Type: finalStatus,
			Payload: eventPayload{
				EvidencePath: params.evidence,
			},
		})
		if err != nil {
			return fmt.Errorf("could not set status to '%s': %v", finalStatus, err)
		}

		if errCmd != nil {
			return err
		}

		err = postActions(ipedfolder)
		return err
	})
}

func withLocker(params ipedParams, locker *remoteLocker, metrics ipedMetrics, f func() error) (finalError error) {
	hostname, _ := os.Hostname()
	err := locker.Lock(params.evidence)
	if err != nil {
		return err
	}
	defer func() {
		result := "done"
		if finalError != nil {
			result = "failed"
		}
		metrics.running.WithLabelValues(hostname, params.evidence).Set(0)
		metrics.finish.WithLabelValues(hostname, params.evidence, result).Inc()
		err := locker.Unlock()
		if err != nil {
			finalError = err
		}
	}()
	return f()
}

func makeLogWriter(params ipedParams, notifierURL string, metrics ipedMetrics) (io.Writer, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	events := make(chan event)
	go eventThrottle(events, func(ev event) {
		processed, found, ok := progress(ev)
		if ok {
			metrics.processed.WithLabelValues(hostname, params.evidence).Set(processed)
			metrics.found.WithLabelValues(hostname, params.evidence).Set(found)
		}
		sendEvent(notifierURL, ev)
	})

	ipedfolder, err := makeIpedFolder(params)
	if err != nil {
		return nil, err
	}

	log, err := os.OpenFile(path.Join(ipedfolder, "IPED.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
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
	return eWriter, nil
}

func eventThrottle(events <-chan event, syncSender func(event)) {
	last := time.Now().Add(-1 * time.Second)
	for ev := range events {
		if ev.Type == "progress" {
			if time.Since(last) < time.Second {
				continue
			}
			last = time.Now()
		}
		go func(ev event) {
			syncSender(ev)
		}(ev)
	}
}

func coreRun(params ipedParams, logWriter io.Writer) error {
	args := makeArgs(params)

	cmd := exec.Command("java", args...)
	cmd.Dir = path.Dir(params.evidence)
	cmd.Stdout = logWriter
	cmd.Stderr = logWriter
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("error in execution: %v", err)
	}
	return cmd.Wait()
}

func makeArgs(params ipedParams) []string {
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
		addArgsArray := strings.Split(params.additionalArgs, " ")
		for i := 0; i < len(addArgsArray); i++ {
			args = append(args, addArgsArray[i])
		}
	}
	if params.additionalPaths != "" {
		addPathsArray := strings.Split(params.additionalPaths, "\n")
		for i := 0; i < len(addPathsArray); i++ {
			args = append(args, "-d", addPathsArray[i])
		}
	}
	return args
}

func makeIpedFolder(params ipedParams) (string, error) {
	var ipedfolder string
	// ipedfolder is the absolute path of the target output folder
	// Ex: /data/mat1/SARD
	// params.output will usually be 'SARD', but it can be an absolute path
	if path.IsAbs(params.output) {
		ipedfolder = params.output
	} else {
		ipedfolder = path.Join(path.Dir(params.evidence), params.output)
	}
	err := os.MkdirAll(ipedfolder, 0755)
	if err != nil {
		return "", err
	}
	err = os.Chmod(ipedfolder, 0750)
	if err != nil {
		return "", err
	}
	return ipedfolder, nil
}

func postActions(ipedfolder string) error {
	err := os.Chmod(ipedfolder, 0755)
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
