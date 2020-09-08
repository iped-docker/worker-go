package main

import (
	"flag"
	"log"
	"os"
)

//go:generate go run generate/main.go

func main() {
	shouldListen := flag.Bool("listen", false, "if the server should receive jobs via API")
	shouldWatch := flag.Bool("watch", false, "if the server should use polling to get jobs")
	path := flag.String("path", "", "path to a datasource, to create a single job")
	flag.Parse()

	jar := assertEnv("IPEDJAR")
	locker := remoteLocker{
		URL: assertEnv("LOCK_URL"),
	}
	notifierURL := assertEnv("NOTIFY_URL")
	PORT, ok := os.LookupEnv("PORT")
	if !ok {
		PORT = "80"
	}

	watchURL := ""
	if *shouldWatch {
		watchURL = assertEnv("WATCH_URL")
	}

	hasJob := (*path != "")

	count := 0
	for _, x := range []bool{*shouldListen, *shouldWatch, hasJob} {
		if x {
			count = count + 1
		}
	}
	if count == 0 {
		log.Fatalf("One of --listen --watch or --path should be used")
	}
	if count > 1 {
		log.Fatalf("Only one of --listen --watch or --path should be used")
	}

	Serve(ServeOptions{
		locker:       &locker,
		PORT:         PORT,
		watchURL:     watchURL,
		shouldWatch:  *shouldWatch,
		jar:          jar,
		notifierURL:  notifierURL,
		hasJob:       hasJob,
		shouldListen: *shouldListen,
	}, Job{
		EvidencePath: *path,
	})
}

func assertEnv(key string) string {
	data, ok := os.LookupEnv(key)
	if !ok {
		log.Fatal("environment variable not set: ", key)
	}
	return data
}
