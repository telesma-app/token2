package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/telesma-app/ctap/protocol"
	"github.com/telesma-app/token2"
)

type recorder struct {
	// The CTAPHID transport reads on a background goroutine.
	mu                          sync.Mutex
	output                      io.Writer
	label, connection, endpoint string
	attempt                     int
}

func (r *recorder) emit(operation string, fields map[string]any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if fields == nil {
		fields = make(map[string]any)
	}
	fields["operation"], fields["time"] = operation, time.Now().UTC()
	fields["label"], fields["connection"] = r.label, r.connection
	fields["endpoint"], fields["attempt"] = r.endpoint, r.attempt
	if err := json.NewEncoder(r.output).Encode(fields); err != nil {
		log.Fatalf("save diagnostic event: %v", err)
	}
}

func errorText(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

func (r *recorder) decoded(operation, kind string, raw []byte) {
	fields := map[string]any{"rawHex": hex.EncodeToString(raw), "bytes": len(raw)}
	var err error
	switch kind {
	case "serial":
		var serial string
		serial, err = token2.ParseSerialNumber(raw)
		if err == nil {
			fields["value"] = serial
			model, known := token2.Identify(serial)
			fields["catalogueKnown"], fields["catalogueModel"] = known, model
		}
	case "atr":
		fields["value"], err = token2.ParseATR(raw)
	case "configuration":
		var config token2.Configuration
		config, err = token2.ParseConfiguration(raw)
		if err == nil {
			fields["value"], fields["interfaceState"] = config, config.InterfaceState()
			caps, known := config.Capabilities()
			fields["capabilities"], fields["capabilitiesKnown"] = caps, known
		}
	case "get-info":
		// Keep the actual field presence and bytes as well as the typed view.
		// Typed Go zero values alone cannot show absent versus explicit zero.
		var members map[uint64]cbor.RawMessage
		if memberErr := cbor.Unmarshal(raw, &members); memberErr == nil {
			hexMembers := make(map[uint64]string, len(members))
			for key, value := range members {
				hexMembers[key] = hex.EncodeToString(value)
			}
			fields["cborFieldsHex"] = hexMembers
		} else {
			fields["cborError"] = memberErr.Error()
		}
		var info protocol.AuthenticatorGetInfoResponse
		err = cbor.Unmarshal(raw, &info)
		if err == nil {
			fields["value"] = info
		}
	}
	fields["parseError"] = errorText(err)
	r.emit(operation+".decoded", fields)
	if err != nil {
		fmt.Fprintf(os.Stderr, "    %s: ошибка разбора: %v\n", operation, err)
	}
}
