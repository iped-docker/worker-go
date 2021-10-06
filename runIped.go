package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type ipedParams struct {
	jar             string
	evidence        string
	output          string
	profile         string
	additionalArgs  string
	additionalPaths string
	mvPath		string
}

func createIpedMetrics() ipedMetrics {
	return ipedMetrics{
		calls: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "ipedworker_runIped_calls",
			Help: "Number of calls to runIped",
		}, []string{"hostname", "evidence"}),
		finish: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "ipedworker_runIped_finish",
			Help: "Number of finished runs",
		}, []string{"hostname", "evidence", "result"}),
		running: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "ipedworker_runIped_running",
			Help: "Whether IPED is running or not",
		}, []string{"hostname", "evidence"}),
		found: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "ipedworker_runIped_found",
			Help: "Number of items found",
		}, []string{"hostname", "evidence"}),
		processed: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "ipedworker_runIped_processed",
			Help: "Number of items processed",
		}, []string{"hostname", "evidence"}),
	}
}

type ipedMetrics struct {
	calls     *prometheus.CounterVec
	finish    *prometheus.CounterVec
	running   *prometheus.GaugeVec
	found     *prometheus.GaugeVec
	processed *prometheus.GaugeVec
}

func runIped(params ipedParams, locker *remoteLocker, notifierURL string, metrics ipedMetrics) (finalError error) {
	hostname, _ := os.Hostname()
	metrics.calls.WithLabelValues(hostname, params.evidence).Inc()
	metrics.running.WithLabelValues(hostname, params.evidence).Set(0)
	err := locker.Lock(params.evidence)
	if err != nil {
		return err
	}
	defer func() {
		result := "done"
		if err != nil {
			result = "failed"
		}
		metrics.running.WithLabelValues(hostname, params.evidence).Set(0)
		metrics.finish.WithLabelValues(hostname, params.evidence, result).Inc()
		err := locker.Unlock()
		if err != nil {
			finalError = err
		}
	}()
	events := make(chan event)
	defer close(events)
	go eventThrottle(events, func(ev event) {
		processed, found, ok := progress(ev)
		if ok {
			metrics.processed.WithLabelValues(hostname, params.evidence).Set(processed)
			metrics.found.WithLabelValues(hostname, params.evidence).Set(found)
		}
		sendEvent(notifierURL, ev)
	})
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
	err = sendEvent(notifierURL, event{
		Type: "running",
		Payload: eventPayload{
			EvidencePath: params.evidence,
		},
	})
	if err != nil {
		return fmt.Errorf("could not set status to 'running': %v", err)
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
	metrics.running.WithLabelValues(hostname, params.evidence).Set(1)
	errCmd := cmd.Wait()
	t := "done"
	if errCmd != nil {
		t = "failed"
	}
	err = sendEvent(notifierURL, event{
		Type: t,
		Payload: eventPayload{
			EvidencePath: params.evidence,
		},
	})
	if err != nil {
		return fmt.Errorf("could not set status to '%s': %v", t, err)
	}
	if errCmd != nil {
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

func permissions(dirPath string, targetPath string) {
	cmd := exec.Command("chmod", "-cR", "a+x", targetPath)
	cmd.Dir = dirPath
	out, err := cmd.CombinedOutput()
	log.Printf("%v", string(out))
	if err != nil {
		log.Printf("%v", err.Error())
	}
}
