package main

import (
	"bytes"
	"testing"

	"github.com/go-ctap/token2"
)

func TestMonitorResultsPrintsAccumulatedSummary(t *testing.T) {
	results := &monitorResults{
		attempts: 3,
		failures: 1,
		serials: map[string]token2.DeviceInfo{
			"72102935780528": {SerialNumber: "72102935780528"},
			"66103930925563": {SerialNumber: "66103930925563"},
		},
	}
	var output bytes.Buffer

	results.print(&output)

	const want = "SUMMARY attempts=3 unique_serials=2 failures=1\n"
	if output.String() != want {
		t.Fatalf("summary = %q, want %q", output.String(), want)
	}
}
