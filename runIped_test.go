package main

import (
	"testing"
)

func TestEventThrottle(t *testing.T) {
	t.Run("should call syncSender", func(t *testing.T) {
		events := make(chan event)
		calls := 0
		go eventThrottle(events, func(event) {
			calls++
		})
		events <- event{}
		close(events)
		if calls != 1 {
			t.Error()
		}
	})
	t.Run("throttle fast progress events", func(t *testing.T) {
		events := make(chan event)
		calls := 0
		go eventThrottle(events, func(event) {
			calls++
		})
		events <- event{
			Type: "progress",
		}
		events <- event{
			Type: "progress",
		}
		events <- event{
			Type: "progress",
		}
		close(events)

		expect := 1
		got := calls
		if expect != got {
			t.Errorf("expected: %v, got: %v", expect, got)
		}
	})
}
