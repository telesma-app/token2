package hid

import (
	"bytes"
	"errors"
	"testing"

	"github.com/go-ctap/iso7816"
	"github.com/go-ctap/token2/internal/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)

	assert.Equal(t, []byte{0xd1, 0x00, 0x90, 0x00}, got)
	require.Len(t, script.sent, 1)
	assert.Len(t, script.sent[0], reportSize)
	assert.Equal(t, byte(reportMagic), script.sent[0][1])
	assert.Equal(t, byte(0), script.sent[0][2])
	assert.Equal(t, byte(2), script.sent[0][3])
	assert.Equal(t, []byte{0x80, 0x33}, script.sent[0][4:6])
	assert.Equal(t, make([]byte, reportSize-6), script.sent[0][6:])
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
	require.NoError(t, err)

	assert.Equal(t, append(first, second...), got)
	require.Len(t, script.sent, 3)
	assert.Equal(t, byte(reportMore|0), script.sent[0][2])
	assert.Equal(t, byte(chunkSize), script.sent[0][3])
	assert.Equal(t, command[:chunkSize], script.sent[0][4:])
	assert.Equal(t, byte(reportMore|1), script.sent[1][2])
	assert.Equal(t, byte(chunkSize), script.sent[1][3])
	assert.Equal(t, command[chunkSize:2*chunkSize], script.sent[1][4:])
	assert.Equal(t, byte(2), script.sent[2][2])
	assert.Equal(t, byte(8), script.sent[2][3])
	assert.Equal(t, command[2*chunkSize:], script.sent[2][4:12])
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
	require.NoError(t, err)
	assert.Equal(t, []byte{0x90, 0x00}, got)
	assert.Empty(t, script.responses)
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

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestTransmitPropagatesFeatureReportErrors(t *testing.T) {
	sendErr := errors.New("send failed")
	_, err := (transceiver{device: &featureScript{sendErr: sendErr}}).Transmit(t.Context(), []byte{0x80, 0x33})
	assert.ErrorIs(t, err, sendErr)

	receiveErr := errors.New("receive failed")
	script := &featureScript{responses: []featureResponse{{err: receiveErr}}}
	_, err = (transceiver{device: script}).Transmit(t.Context(), []byte{0x80, 0x33})
	assert.ErrorIs(t, err, receiveErr)
}

func TestDeviceClose(t *testing.T) {
	closeErr := errors.New("close failed")
	script := &featureScript{closeErr: closeErr}
	device := &Device{device: script}

	err := device.Close()

	assert.ErrorIs(t, err, closeErr)
	assert.True(t, script.closed)
}

func TestDeviceInfo(t *testing.T) {
	const serial = "0123456789abcdef"
	payload := append([]byte{0xd1, byte(len(serial))}, serial...)
	payload = append(payload, 0x90, 0x00)
	response := responseReport(0, false, payload)
	script := &featureScript{responses: []featureResponse{{report: response, n: len(response)}}}
	device := &Device{device: script}

	info, err := device.DeviceInfo(t.Context())
	require.NoError(t, err)

	assert.Equal(t, serial, info.SerialNumber)
	require.Len(t, script.sent, 1)
	command, err := protocol.SerialNumberCommand(iso7816.EncodingExtended).MarshalBinary()
	require.NoError(t, err)
	assert.Equal(t, command, script.sent[0][4:4+len(command)])
}

func TestDeviceInfoStatusError(t *testing.T) {
	response := responseReport(0, false, []byte{0x6a, 0x82})
	script := &featureScript{responses: []featureResponse{{report: response, n: len(response)}}}
	device := &Device{device: script}

	_, err := device.DeviceInfo(t.Context())

	var statusErr *iso7816.APDUError
	require.ErrorAs(t, err, &statusErr)
	assert.Equal(t, iso7816.StatusWord(0x6a82), statusErr.StatusWord())
	assert.Contains(t, err.Error(), "read serial number")
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
