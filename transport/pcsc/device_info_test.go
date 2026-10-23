package pcsc

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/telesma-app/iso7816"
	nativepcsc "github.com/telesma-app/pcsc"
	"github.com/telesma-app/token2"
)

func TestDeviceInfoKeepsIndependentReadingsAndCleanupError(t *testing.T) {
	cleanupErr := errors.New("cleanup failed")

	for _, configError := range []bool{false, true} {
		response := []byte{2, 0x90, 0}
		if configError {
			response = []byte{0x6a, 0x86}
		}

		card := &scriptedCard{endErr: cleanupErr, status: &nativepcsc.CardStatus{ATR: []byte{0x3b, 0x8f, 0x80, 0x01, 0x54, 0x4b, 0, 0x50, 0x49, 0x56, 4, 2, 0x38, 0x38, 0x38, 0x38, 0x38, 0x38, 0x38, 0x60}}, steps: []cardStep{
			{command: selectOTPAPDU, response: []byte{0x90, 0}},
			{command: serialAPDU, response: []byte{0xd1, 4, '1', '2', '3', '4', 0x90, 0}},
			{command: selectOTPAPDU, response: []byte{0x90, 0}},
			{command: configAPDU, response: response},
		}}

		info, err := (&Device{card: card}).DeviceInfo(t.Context())
		if !errors.Is(err, cleanupErr) || card.begins != 1 || card.ends != 1 || card.closed || len(card.steps) != 0 {
			t.Fatalf("transaction=%d/%d error=%v remaining=%d", card.begins, card.ends, err, len(card.steps))
		}

		if info.SerialNumber.Value == nil || *info.SerialNumber.Value != "1234" || info.ATR.Value == nil || info.ATR.Value.Identity != nil || info.ATR.Err != nil {
			t.Fatalf("independent values lost: %#v", info)
		}

		if configError {
			if info.Configuration.Value != nil || info.Configuration.Err == nil {
				t.Fatal("configuration failure lost")
			}

			var statusErr *iso7816.APDUError
			if !errors.As(info.Configuration.Err, &statusErr) || statusErr.SW1 != 0x6a || statusErr.SW2 != 0x86 || !errors.Is(err, info.Configuration.Err) {
				t.Fatalf("configuration error chain lost: %v", info.Configuration.Err)
			}

			encoded, err := json.Marshal(info)
			if err != nil {
				t.Fatal(err)
			}

			if !strings.Contains(string(encoded), `"configuration":{"value":null}`) || strings.Contains(string(encoded), `"error"`) || strings.Contains(string(encoded), `"Err"`) {
				t.Fatalf("error encoding=%s", encoded)
			}
		} else if info.Configuration.Value == nil || info.Configuration.Err != nil {
			t.Fatal("configuration value lost")
		}
	}
}

func TestDeviceInfoPreservesReadEmptySerial(t *testing.T) {
	card := &scriptedCard{status: &nativepcsc.CardStatus{ATR: []byte{0x3b, 0}}, steps: []cardStep{
		{command: selectOTPAPDU, response: []byte{0x90, 0}},
		{command: serialAPDU, response: []byte{0xd1, 0, 0x90, 0}},
		{command: selectOTPAPDU, response: []byte{0x90, 0}},
		{command: configAPDU, response: []byte{0, 0x90, 0}},
	}}

	info, err := ReadDeviceInfo(t.Context(), card)
	if err != nil || info.SerialNumber.Value == nil || *info.SerialNumber.Value != "" || info.SerialNumber.Err != nil {
		t.Fatalf("serial = %#v, error = %v", info.SerialNumber, err)
	}

	if card.begins != 0 || card.ends != 0 || card.closed || len(card.steps) != 0 {
		t.Fatalf("borrowed lifecycle: begins=%d ends=%d closed=%t remaining=%d", card.begins, card.ends, card.closed, len(card.steps))
	}

	if info.Model != nil || info.Configuration.Value == nil || info.Configuration.Value.TransferType != 0 {
		t.Fatalf("unknown model or zero configuration lost: %#v", info)
	}
}

func TestDeviceInfoContinuesAfterSerialAndATRFailures(t *testing.T) {
	transmitErr := errors.New("serial read failed")
	card := &scriptedCard{status: &nativepcsc.CardStatus{ATR: []byte{0x3b}}, steps: []cardStep{
		{command: selectOTPAPDU, response: successfulResponse(nil)},
		{command: serialAPDU, err: transmitErr},
		{command: selectOTPAPDU, response: successfulResponse(nil)},
		{command: configAPDU, response: successfulResponse([]byte{0})},
	}}

	info, err := ReadDeviceInfo(t.Context(), card)

	if info.SerialNumber.Value != nil || !errors.Is(info.SerialNumber.Err, transmitErr) {
		t.Fatalf("serial = %#v", info.SerialNumber)
	}

	if info.ATR.Value != nil || !errors.Is(info.ATR.Err, token2.ErrInvalidATR) {
		t.Fatalf("ATR = %#v", info.ATR)
	}

	if info.Configuration.Value == nil || info.Configuration.Value.TransferType != 0 || info.Configuration.Err != nil {
		t.Fatalf("configuration = %#v", info.Configuration)
	}

	if !errors.Is(err, transmitErr) || !errors.Is(err, token2.ErrInvalidATR) || len(card.steps) != 0 {
		t.Fatalf("error = %v, remaining commands = %d", err, len(card.steps))
	}
}
