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
	mvPath          string
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

	_, errFs := os.Stat(params.evidence)
	if os.IsNotExist(errFs) {
		err_type := "failed"
		err = sendEvent(notifierURL, event{
			Type: err_type,
			Payload: eventPayload{
				EvidencePath: params.evidence,
			},
		})

		if err != nil {
			return fmt.Errorf("could not set status to '%s': %v", err_type, err)
		}
		return fmt.Errorf("Main evidence does not exist '%s': %v", err_type, errFs)
	}

	args := []string{
		"-Djava.awt.headless=true",
		"-jar", params.jar,
		"-Xms8G",
		"-d", path.Base(params.evidence),
		"-o", params.output,
		"--portable",
		"--nologfile",
		"--nogui",
	}
	if params.profile != "" {
		args = append(args, "-profile", params.profile)
	} else {
		args = append(args, "-profile", "pedo")
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

	var ipedFolder string
	// ipedfolder is the absolute path of the target output folder
	// Ex: /data/mat1/SARD
	// params.output will usually be 'SARD', but it can be an absolute path
	if path.IsAbs(params.output) {
		ipedFolder = params.output
	} else {
		ipedFolder = path.Join(path.Dir(params.evidence), params.output)
	}
	err = os.MkdirAll(ipedFolder, 0755)
	if err != nil {
		return err
	}
	err = os.Chmod(ipedFolder, 0750)
	if err != nil {
		return err
	}
	log, err := os.OpenFile(path.Join(ipedFolder, "IPED.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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

	return postProcessing(ipedFolder, params.mvPath)

}

func postProcessing(dirPath string, mvPath string) (finalError error) {
	err := os.Chmod(dirPath, 0755)
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
		permissions(dirPath, p)
	}

	return moveDir(dirPath, mvPath)

}

func moveDir(dirPath string, mvPath string) (finalError error) {
	if mvPath != "" {
		_, err := os.Stat(mvPath)
		if os.IsNotExist(err) {
			srcPathArray := strings.Split(dirPath, "/")
			srcDir := ""
			// because absolute path split creates an empty first item on array we start i with 1
			for i := 1; i < len(srcPathArray)-1; i++ {
				srcDir = srcDir + "/" + srcPathArray[i]
			}

			dstPathArray := strings.Split(mvPath, "/")
			dstDir := ""
			for i := 1; i < len(dstPathArray)-1; i++ {
				dstDir = dstDir + "/" + dstPathArray[i]
			}
			cmd := exec.Command("mkdir", "-pv", dstDir)
			out, err := cmd.CombinedOutput()
			log.Printf("%v", string(out))
			if err != nil {
				return err
			} else {
				cmd := exec.Command("mv", "-v", srcDir, mvPath)
				out, err = cmd.CombinedOutput()
				log.Printf("%v", string(out))
				if err != nil {
					return err
				}
			}
		} else {
			return fmt.Errorf("Destination " + mvPath + " path already exists. Fix it manually!")
		}
	}
	return nil
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
