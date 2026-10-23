package main

import (
	"bytes"
	"testing"
)

func TestMonitorResultsPrintsAccumulatedSummary(t *testing.T) {
	results := &monitorResults{
		attempts: 3,
		failures: 1,
		serials: map[string]struct{}{
			"72102935780528": {},
			"66103930925563": {},
		},
	}
	var output bytes.Buffer

	results.print(&output)

	const want = "SUMMARY attempts=3 unique_serials=2 failures=1\n"
	if output.String() != want {
		t.Fatalf("summary = %q, want %q", output.String(), want)
	}
}
