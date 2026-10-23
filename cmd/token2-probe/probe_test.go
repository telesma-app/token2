package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/telesma-app/iso7816"
)

func TestMalformedATRRemainsInReport(t *testing.T) {
	var output bytes.Buffer
	r := recorder{output: &output}
	r.decoded("atr", "atr", []byte{1, 2, 3})
	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record["rawHex"] != "010203" || record["parseError"] == "" {
		t.Fatalf("lost malformed response: %s", output.String())
	}
}

func TestGenericATRIsNotAReadFailure(t *testing.T) {
	var output bytes.Buffer
	r := recorder{output: &output}
	raw, _ := hex.DecodeString("3b8f8001544b0050495604023838383838383860")
	r.decoded("atr", "atr", raw)
	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record["parseError"] != "" {
		t.Fatalf("generic ATR rejected: %s", output.String())
	}
	if _, present := record["value"].(map[string]any)["identity"]; present {
		t.Fatal("invented Token2 identity")
	}
}

func TestGetInfoKeepsExplicitZeroField(t *testing.T) {
	var output bytes.Buffer
	r := recorder{output: &output}
	r.decoded("get-info", "get-info", []byte{0xa1, 0x0e, 0x00})
	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record["cborFieldsHex"].(map[string]any)["14"] != "00" {
		t.Fatalf("lost explicit firmware zero: %s", output.String())
	}
}

type commandCard struct{ commands [][]byte }

func (c *commandCard) Transmit(_ context.Context, raw []byte) ([]byte, error) {
	c.commands = append(c.commands, bytes.Clone(raw))
	if raw[1] == 0xa4 {
		return []byte{0x90, 0}, nil
	}
	return []byte{0x6d, 0}, nil
}

func TestUnsupportedCommandsDoNotSkipOtherMeasurements(t *testing.T) {
	card := &commandCard{}
	var output bytes.Buffer
	probeAppletCommands(t.Context(), card, &recorder{output: &output})
	if len(card.commands) != 9 {
		t.Fatalf("got %d commands, want all 9", len(card.commands))
	}
	if card.commands[0][1] != 0xa4 || card.commands[1][1] != 0x33 {
		t.Fatalf("initial commands: %#v", card.commands[:2])
	}
	if !strings.Contains(output.String(), "\"operation\":\"fido.get-info\"") || strings.Contains(output.String(), ".decoded") {
		t.Fatalf("incomplete probe or parsed failure payload: %s", output.String())
	}
}

type featureScript struct {
	reports [][]byte
	sent    [][]byte
	cancel  context.CancelFunc
}

func (s *featureScript) SendFeatureReport(p []byte) error {
	s.sent = append(s.sent, bytes.Clone(p))
	return nil
}

func (s *featureScript) GetFeatureReport(p []byte) (int, error) {
	report := s.reports[0]
	s.reports = s.reports[1:]
	n := copy(p, report)
	if s.cancel != nil {
		s.cancel()
	}
	return n, nil
}

func TestFeatureFramingPreservesMalformedReply(t *testing.T) {
	var output bytes.Buffer
	script := &featureScript{reports: [][]byte{{0, 0x21, 1, 2, 0x90, 0}}}
	_, err := (featureCard{device: script, r: &recorder{output: &output}}).Transmit(t.Context(), []byte{0, 1})
	if err == nil || !strings.Contains(output.String(), "002101029000") {
		t.Fatalf("malformed fragment lost: error=%v report=%s", err, output.String())
	}
}

func TestFeaturePendingHonorsDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var output bytes.Buffer
	script := &featureScript{reports: [][]byte{{0, 0x21, 0xc0, 0}}, cancel: cancel}
	_, err := (featureCard{device: script, r: &recorder{output: &output}}).Transmit(ctx, []byte{0, 1})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
}

func TestFeatureFragmentsAndAPDUContinuation(t *testing.T) {
	var output bytes.Buffer
	script := &featureScript{reports: [][]byte{
		{0, 0x21, 0x20, 2, 0xd1, 2},
		{0, 0x21, 1, 3, '4', 0x61, 1},
		{0, 0x21, 0, 3, '2', 0x90, 0},
	}}
	response, err := iso7816.Exchange(t.Context(), featureCard{device: script, r: &recorder{output: &output}}, iso7816.Command{CLA: 0x80, INS: 0x33})
	if err != nil || response.Status != 0x9000 || !bytes.Equal(response.Data, []byte{0xd1, 2, '4', '2'}) {
		t.Fatalf("response=%#v error=%v", response, err)
	}
	if len(script.sent) != 2 || !bytes.Equal(script.sent[1][4:9], []byte{0x80, 0xc0, 0, 0, 1}) {
		t.Fatalf("GET RESPONSE: %x", script.sent)
	}
}
