package main

import (
	"flag"
	"log"
	"os"
)

//go:generate go run generate/main.go

func main() {
	path := flag.String("path", os.Getenv("EVIDENCE_PATH"), "(EVIDENCE_PATH) path to a datasource, to create a single job")
	jar := flag.String("jar", os.Getenv("IPEDJAR"), "(IPEDJAR) path to the IPED.jar file")
	lockURL := flag.String("lock", os.Getenv("LOCK_URL"), "(LOCK_URL) URL of the lock service")
	notifierURL := flag.String("notifier", os.Getenv("NOTIFY_URL"), "(NOTIFY_URL) URL of the notifier service")
	port := flag.String("port", os.Getenv("PORT"), "(PORT=80) port to serve metrics")

	flag.Parse()

	if "" == *path {
		log.Fatal("environment variable not set: EVIDENCE_PATH")
	}
	if "" == *jar {
		log.Fatal("environment variable not set: IPEDJAR")
	}
	if "" == *lockURL {
		log.Fatal("environment variable not set: LOCK_URL")
	}
	locker := remoteLocker{
		URL: *lockURL,
	}
	if "" == *notifierURL {
		log.Fatal("environment variable not set: NOTIFY_URL")
	}
	if "" == *port {
		*port = "80"
	}

	ctx := Serve(*port, &locker)
	job := Job{
		EvidencePath: *path,
	}
	processPayloads(ctx, []Job{job}, *jar, &locker, *notifierURL)
}
