package main

import "testing"

func TestProgress(t *testing.T) {
	cases := []struct {
		input           event
		expectProcessed float64
		expectFound     float64
		expectOk        bool
	}{
		{
			event{
				Type: "progress",
				Payload: eventPayload{
					Progress: "Processando 1/2",
				},
			},
			1.0,
			2.0,
			true,
		},
		{
			event{
				Type: "progress",
				Payload: eventPayload{
					Progress: "Processando x/2",
				},
			},
			0,
			0,
			false,
		},
		{
			event{
				Type: "else",
				Payload: eventPayload{
					Progress: "Processando 1/2",
				},
			},
			0,
			0,
			false,
		},
		{
			event{
				Type: "progress",
				Payload: eventPayload{
					Progress: "2020-04-24 15:12:43     [MSG]   [indexer.process.ProgressConsole]                       Processando 2153/3591 (7%) 64GB/h Termino em 0h 55m 9s",
				},
			},
			2153,
			3591,
			true,
		},
	}
	for _, c := range cases {
		processed, found, ok := progress(c.input)
		if processed != c.expectProcessed {
			t.Errorf("expected: %v, got %v, input: %x", c.expectProcessed, processed, c.input)
		}
		if found != c.expectFound {
			t.Errorf("expected: %v, got %v, input: %x", c.expectFound, found, c.input)
		}
		if ok != c.expectOk {
			t.Errorf("expected: %v, got %v, input: %x", c.expectOk, ok, c.input)
		}
	}
}
