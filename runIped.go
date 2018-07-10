package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"
)

type childStruct struct {
	lock sync.Mutex
	cmd  *exec.Cmd
}

var child childStruct

func assertEnv(key string) string {
	data := os.Getenv(key)
	if data == "" {
		log.Fatal("environment variable not set: ", key)
	}
	return data
}

type IpedParams struct {
	memory   string
	jar      string
	evidence string
	output   string
}

func runIped(params IpedParams) chan error {
	if params.jar == "" {
		params.jar = assertEnv("IPEDJAR")
	}
	if params.memory == "" {
		params.memory = assertEnv("MEMORY")
	}
	timeout := make(chan bool, 1)
	result := make(chan error, 1)
	go func() {
		time.Sleep(10 * time.Millisecond)
		timeout <- true
		result <- errors.New("busy")
	}()
	go func() {
		child.lock.Lock()
		defer child.lock.Unlock()
		select {
		case <-timeout:
			return
		default:
			result <- nil
			child.cmd = exec.Command("java",
				"-Djava.awt.headless=true",
				fmt.Sprintf("-Xmx%s", params.memory),
				"-jar", params.jar,
				"-d", params.evidence,
				"-o", params.output,
				"--nologfile",
				"--nogui",
				"--portable")
			child.cmd.Stdout = os.Stdout
			child.cmd.Stderr = os.Stderr
			err := child.cmd.Run()
			if err != nil {
				// log.Fatal(err)
			}
		}
	}()
	return result
}
