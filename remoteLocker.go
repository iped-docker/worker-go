package main

import (
	"sync"
)

type remoteLocker struct {
	Locker       sync.Mutex
	URL          string
	EvidencePath string
}

func (l *remoteLocker) Lock(evidencePath string) error {
	l.Locker.Lock()
	l.EvidencePath = evidencePath
	body := event{
		Type: "LOCK",
		Payload: eventPayload{
			EvidencePath: l.EvidencePath,
		},
	}
	err := sendEvent(l.URL, body)
	if err != nil {
		l.EvidencePath = ""
	}
	return err
}

func (l *remoteLocker) Unlock() error {
	defer func() {
		l.EvidencePath = ""
		l.Locker.Unlock()
	}()
	body := event{
		Type: "UNLOCK",
		Payload: eventPayload{
			EvidencePath: l.EvidencePath,
		},
	}
	return sendEvent(l.URL, body)
}
