package main

import (
	"regexp"
	"strconv"
)

func progress(ev event) (float64, float64, bool) {
	progressRegexp := regexp.MustCompile("Processando ([0-9]+)/([0-9]+)")
	if ev.Type == "progress" {
		matches := progressRegexp.FindSubmatch([]byte(ev.Payload.Progress))
		if len(matches) != 3 {
			return 0, 0, false
		}
		processed, err := strconv.Atoi(string(matches[1]))
		if err != nil {
			return 0, 0, false
		}
		found, err := strconv.Atoi(string(matches[2]))
		if err != nil {
			return 0, 0, false
		}
		return float64(processed), float64(found), true
	}
	return 0, 0, false
}
