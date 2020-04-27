package main

import (
	"log"
	"os"
)

//go:generate go run generate/main.go

func main() {
	jar := assertEnv("IPEDJAR")
	locker := remoteLocker{
		URL: assertEnv("LOCK_URL"),
	}
	notifierURL := assertEnv("NOTIFY_URL")
	PORT, ok := os.LookupEnv("PORT")
	if !ok {
		PORT = "80"
	}
	watchURL, isWatching := os.LookupEnv("WATCH_URL")

	Serve(ServeOptions{
		locker:      &locker,
		PORT:        PORT,
		watchURL:    watchURL,
		isWatching:  isWatching,
		jar:         jar,
		notifierURL: notifierURL,
	})
}

func assertEnv(key string) string {
	data, ok := os.LookupEnv(key)
	if !ok {
		log.Fatal("environment variable not set: ", key)
	}
	return data
}
