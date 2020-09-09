package main

import (
	"flag"
	"log"
	"os"
)

func main() {
	path := flag.String("path", os.Getenv("EVIDENCE_PATH"), "(EVIDENCE_PATH) path to a datasource, to create a single job")
	jar := flag.String("jar", os.Getenv("IPEDJAR"), "(IPEDJAR) path to the IPED.jar file")
	lockURL := flag.String("lock", os.Getenv("LOCK_URL"), "(LOCK_URL) URL of the lock service")
	notifierURL := flag.String("notifier", os.Getenv("NOTIFY_URL"), "(NOTIFY_URL) URL of the notifier service")
	port := flag.String("port", os.Getenv("PORT"), "(PORT=80) port to serve metrics")

	outputPath := flag.String("output", os.Getenv("OUTPUT_PATH"), "(OUTPUT_PATH) IPED output folder")
	profile := flag.String("profile", os.Getenv("IPED_PROFILE"), "(IPED_PROFILE) IPED profile")
	addArgs := flag.String("addargs", os.Getenv("ADD_ARGS"), "(ADD_ARGS) extra arguments to IPED")
	addPaths := flag.String("addpaths", os.Getenv("ADD_PATHS"), "(ADD_PATHS) extra source paths to IPED")

	flag.Parse()

	job := Job{
		EvidencePath:    *path,
		OutputPath:      *outputPath,
		Profile:         *profile,
		AdditionalArgs:  *addArgs,
		AdditionalPaths: *addPaths,
	}

	if "" == *path {
		log.Fatal("environment variable not set: EVIDENCE_PATH")
	}
	if "" == *outputPath {
		log.Fatal("environment variable not set: OUTPUT_PATH")
	}
	if "" == *profile {
		log.Fatal("environment variable not set: IPED_PROFILE")
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
	processPayloads(ctx, []Job{job}, *jar, &locker, *notifierURL)
}
