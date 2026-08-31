package hid

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/telesma-app/iso7816"
	"github.com/telesma-app/token2/internal/protocol"
)

type featureResponse struct {
	report []byte
	n      int
	err    error
}

type featureScript struct {
	sent      [][]byte
	responses []featureResponse
	sendErr   error
	closeErr  error
	closed    bool
}

func (d *featureScript) SendFeatureReport(report []byte) error {
	d.sent = append(d.sent, append([]byte(nil), report...))
	return d.sendErr
}

func (d *featureScript) GetFeatureReport(report []byte) (int, error) {
	response := d.responses[0]
	d.responses = d.responses[1:]
	copy(report, response.report)
	return response.n, response.err
}

func (d *featureScript) Close() error {
	d.closed = true
	return d.closeErr
}

func TestTransmitSingleChunk(t *testing.T) {
	response := responseReport(0, false, []byte{0xd1, 0x00, 0x90, 0x00})
	script := &featureScript{responses: []featureResponse{{report: response, n: len(response)}}}

	got, err := (transceiver{device: script}).Transmit(t.Context(), []byte{0x80, 0x33})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	{
		want, got := []byte{0xd1, 0x00, 0x90, 0x00}, got
		if (got == nil) != (want == nil) || !bytes.Equal(got, want) {
			t.Errorf("got %#v, want %#v", got, want)
		}
	}
	if got, want := len(script.sent), 1; got != want {
		t.Fatalf("got length %d, want %d", got, want)
	}
	if got, want := len(script.sent[0]), reportSize; got != want {
		t.Errorf("got length %d, want %d", got, want)
	}
	{
		want, got := byte(reportMagic), script.sent[0][1]
		if got != want {
			t.Errorf("got %#v, want %#v", got, want)
		}
	}
	{
		want, got := byte(0), script.sent[0][2]
		if got != want {
			t.Errorf("got %#v, want %#v", got, want)
		}
	}
	{
		want, got := byte(2), script.sent[0][3]
		if got != want {
			t.Errorf("got %#v, want %#v", got, want)
		}
	}
	{
		want, got := []byte{0x80, 0x33}, script.sent[0][4:6]
		if (got == nil) != (want == nil) || !bytes.Equal(got, want) {
			t.Errorf("got %#v, want %#v", got, want)
		}
	}
	{
		want, got := make([]byte, reportSize-6), script.sent[0][6:]
		if (got == nil) != (want == nil) || !bytes.Equal(got, want) {
			t.Errorf("got %#v, want %#v", got, want)
		}
	}
}

func TestTransmitMultipleChunks(t *testing.T) {
	command := make([]byte, 2*chunkSize+8)
	for i := range command {
		command[i] = byte(i)
	}

	first := bytes.Repeat([]byte{0xa1}, chunkSize)
	second := []byte{0xb2, 0xc3, 0xd4}
	firstReport := responseReport(0, true, first)
	secondReport := responseReport(1, false, second)
	script := &featureScript{responses: []featureResponse{
		{report: firstReport, n: len(firstReport)},
		{report: secondReport, n: len(secondReport)},
	}}

	got, err := (transceiver{device: script}).Transmit(t.Context(), command)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	{
		want, got := append(first, second...), got
		if (got == nil) != (want == nil) || !bytes.Equal(got, want) {
			t.Errorf("got %#v, want %#v", got, want)
		}
	}
	if got, want := len(script.sent), 3; got != want {
		t.Fatalf("got length %d, want %d", got, want)
	}
	{
		want, got := byte(reportMore|0), script.sent[0][2]
		if got != want {
			t.Errorf("got %#v, want %#v", got, want)
		}
	}
	{
		want, got := byte(chunkSize), script.sent[0][3]
		if got != want {
			t.Errorf("got %#v, want %#v", got, want)
		}
	}
	{
		want, got := command[:chunkSize], script.sent[0][4:]
		if (got == nil) != (want == nil) || !bytes.Equal(got, want) {
			t.Errorf("got %#v, want %#v", got, want)
		}
	}
	{
		want, got := byte(reportMore|1), script.sent[1][2]
		if got != want {
			t.Errorf("got %#v, want %#v", got, want)
		}
	}
	{
		want, got := byte(chunkSize), script.sent[1][3]
		if got != want {
			t.Errorf("got %#v, want %#v", got, want)
		}
	}
	{
		want, got := command[chunkSize:2*chunkSize], script.sent[1][4:]
		if (got == nil) != (want == nil) || !bytes.Equal(got, want) {
			t.Errorf("got %#v, want %#v", got, want)
		}
	}
	{
		want, got := byte(2), script.sent[2][2]
		if got != want {
			t.Errorf("got %#v, want %#v", got, want)
		}
	}
	{
		want, got := byte(8), script.sent[2][3]
		if got != want {
			t.Errorf("got %#v, want %#v", got, want)
		}
	}
	{
		want, got := command[2*chunkSize:], script.sent[2][4:12]
		if (got == nil) != (want == nil) || !bytes.Equal(got, want) {
			t.Errorf("got %#v, want %#v", got, want)
		}
	}
}

func TestTransmitWaitsForPendingReport(t *testing.T) {
	pending := make([]byte, reportSize)
	pending[1] = reportMagic
	pending[2] = reportPending | 0x0f
	response := responseReport(0, false, []byte{0x90, 0x00})
	script := &featureScript{responses: []featureResponse{
		{report: pending, n: len(pending)},
		{report: response, n: len(response)},
	}}

	got, err := (transceiver{device: script}).Transmit(t.Context(), []byte{0x80, 0x33})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	{
		want, got := []byte{0x90, 0x00}, got
		if (got == nil) != (want == nil) || !bytes.Equal(got, want) {
			t.Errorf("got %#v, want %#v", got, want)
		}
	}
	if got := script.responses; len(got) != 0 {
		t.Errorf("got non-empty value %#v", got)
	}
}

func TestTransmitRejectsMalformedReports(t *testing.T) {
	valid := responseReport(0, false, []byte{0x90, 0x00})

	tests := []struct {
		name    string
		report  []byte
		n       int
		wantErr string
	}{
		{
			name:    "short header",
			report:  valid,
			n:       3,
			wantErr: "report is 3 bytes; need at least 4",
		},
		{
			name:    "reported length exceeds buffer",
			report:  valid,
			n:       reportSize + 1,
			wantErr: "report length 66 exceeds buffer size 65",
		},
		{
			name: "pending report with wrong magic",
			report: func() []byte {
				report := append([]byte(nil), valid...)
				report[1] = 0x22
				report[2] = reportPending
				return report
			}(),
			n:       reportSize,
			wantErr: "unexpected Token2 HID report magic: 22",
		},
		{
			name: "wrong sequence",
			report: func() []byte {
				report := append([]byte(nil), valid...)
				report[2] = 1
				return report
			}(),
			n:       reportSize,
			wantErr: "report sequence: got 1, want 0",
		},
		{
			name: "oversized payload",
			report: func() []byte {
				report := append([]byte(nil), valid...)
				report[3] = chunkSize + 1
				return report
			}(),
			n:       reportSize,
			wantErr: "payload length 62 exceeds 61",
		},
		{
			name:    "truncated payload",
			report:  responseReport(0, false, []byte{1, 2, 3, 4}),
			n:       7,
			wantErr: "payload length 4 exceeds received report size 7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script := &featureScript{responses: []featureResponse{{report: tt.report, n: tt.n}}}

			_, err := (transceiver{device: script}).Transmit(t.Context(), []byte{0x80, 0x33})

			if err == nil {
				t.Fatalf("expected an error")
			}
			{
				container, element := err.Error(), tt.wantErr
				if !strings.Contains(container, element) {
					t.Errorf("value does not contain %#v", element)
				}
			}
		})
	}
}

func TestTransmitPropagatesFeatureReportErrors(t *testing.T) {
	sendErr := errors.New("send failed")
	_, err := (transceiver{device: &featureScript{sendErr: sendErr}}).Transmit(t.Context(), []byte{0x80, 0x33})
	{
		err, target := err, sendErr
		if !errors.Is(err, target) {
			t.Errorf("got error %v, want errors.Is(error, %#v)", err, target)
		}
	}

	receiveErr := errors.New("receive failed")
	script := &featureScript{responses: []featureResponse{{err: receiveErr}}}
	_, err = (transceiver{device: script}).Transmit(t.Context(), []byte{0x80, 0x33})
	{
		err, target := err, receiveErr
		if !errors.Is(err, target) {
			t.Errorf("got error %v, want errors.Is(error, %#v)", err, target)
		}
	}
}

func TestDeviceClose(t *testing.T) {
	closeErr := errors.New("close failed")
	script := &featureScript{closeErr: closeErr}
	device := &Device{device: script}

	err := device.Close()

	{
		err, target := err, closeErr
		if !errors.Is(err, target) {
			t.Errorf("got error %v, want errors.Is(error, %#v)", err, target)
		}
	}
	if got := script.closed; !got {
		t.Errorf("got false, want true")
	}
}

func TestDeviceInfo(t *testing.T) {
	const serial = "0123456789abcdef"
	payload := append([]byte{0xd1, byte(len(serial))}, serial...)
	payload = append(payload, 0x90, 0x00)
	response := responseReport(0, false, payload)
	script := &featureScript{responses: []featureResponse{{report: response, n: len(response)}}}
	device := &Device{device: script}

	info, err := device.DeviceInfo(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	{
		want, got := serial, info.SerialNumber
		if got != want {
			t.Errorf("got %#v, want %#v", got, want)
		}
	}
	if got, want := len(script.sent), 1; got != want {
		t.Fatalf("got length %d, want %d", got, want)
	}
	command, err := protocol.SerialNumberCommand(iso7816.EncodingExtended).MarshalBinary()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	{
		want, got := command, script.sent[0][4:4+len(command)]
		if (got == nil) != (want == nil) || !bytes.Equal(got, want) {
			t.Errorf("got %#v, want %#v", got, want)
		}
	}
}

func TestDeviceInfoStatusError(t *testing.T) {
	response := responseReport(0, false, []byte{0x6a, 0x82})
	script := &featureScript{responses: []featureResponse{{report: response, n: len(response)}}}
	device := &Device{device: script}

	_, err := device.DeviceInfo(t.Context())

	var statusErr *iso7816.APDUError
	if err := err; !errors.As(err, &statusErr) {
		t.Fatalf("error %v does not match requested type", err)
	}
	{
		want, got := iso7816.StatusWord(0x6a82), statusErr.StatusWord()
		if got != want {
			t.Errorf("got %#v, want %#v", got, want)
		}
	}
	{
		container, element := err.Error(), "read serial number"
		if !strings.Contains(container, element) {
			t.Errorf("value does not contain %#v", element)
		}
	}
}

func responseReport(sequence byte, more bool, payload []byte) []byte {
	report := make([]byte, reportSize)
	report[1] = reportMagic
	report[2] = sequence & 0x0f
	if more {
		report[2] |= reportMore
	}
	report[3] = byte(len(payload))
	copy(report[4:], payload)
	return report
}
