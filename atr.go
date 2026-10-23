package token2

import (
	"errors"
	"fmt"
)

const serialSuffixLength = 8

var historicalPrefix = [2]byte{0x86, 0x8e}

// ErrInvalidATR reports malformed ATR data or malformed Token2 identity bytes.
var ErrInvalidATR = errors.New("token2: invalid ATR")

// ATR contains Token2 identity data encoded in a PC/SC answer-to-reset.
type ATR struct {
	Raw      []byte       `json:"raw"`
	Identity *ATRIdentity `json:"identity,omitempty"`
}

// ATRIdentity is present only for Token2's vendor historical-byte format.
type ATRIdentity struct {
	ProductID    uint16 `json:"productId"`
	SerialSuffix string `json:"serialSuffix"`
}

// ParseATR extracts the USB product ID and decimal serial suffix from a Token2
// ATR. Raw refers to the supplied ATR slice. A valid ATR without Token2
// historical bytes has a nil Identity and is a successful result.
func ParseATR(atr []byte) (ATR, error) {
	historical, err := historicalBytes(atr)
	if err != nil {
		return ATR{}, err
	}

	if len(historical) < 2 || historical[0] != historicalPrefix[0] || historical[1] != historicalPrefix[1] {
		return ATR{Raw: atr}, nil
	}

	if len(historical) != 15 {
		return ATR{}, fmt.Errorf("%w: Token2 identity has %d historical bytes, need 15", ErrInvalidATR, len(historical))
	}

	serialOffset := len(historical) - serialSuffixLength

	return ATR{
		Raw: atr,
		Identity: &ATRIdentity{
			ProductID:    uint16(historical[2])<<8 | uint16(historical[3]),
			SerialSuffix: string(historical[serialOffset:]),
		},
	}, nil
}

// Walk the interface-byte groups to locate the historical bytes and verify
// the TCK when a protocol other than T=0 is advertised.
func historicalBytes(atr []byte) ([]byte, error) {
	if len(atr) < 2 || atr[0] != 0x3b && atr[0] != 0x3f {
		return nil, fmt.Errorf("%w: missing ATR header", ErrInvalidATR)
	}

	offset := 2
	presence := atr[1] >> 4
	hasTCK := false

	for {
		for mask := byte(1); mask <= 4; mask <<= 1 {
			if presence&mask != 0 {
				offset++
			}
		}

		if offset > len(atr) {
			return nil, fmt.Errorf("%w: truncated interface bytes", ErrInvalidATR)
		}

		if presence&8 == 0 {
			break
		}

		if offset == len(atr) {
			return nil, fmt.Errorf("%w: missing TD byte", ErrInvalidATR)
		}

		hasTCK = hasTCK || atr[offset]&15 != 0
		presence = atr[offset] >> 4
		offset++
	}

	end := offset + int(atr[1]&15)
	if end > len(atr) {
		return nil, fmt.Errorf("%w: truncated historical bytes", ErrInvalidATR)
	}

	expected := end
	if hasTCK {
		expected++
	}

	if len(atr) != expected {
		return nil, fmt.Errorf("%w: got %d bytes, expected %d", ErrInvalidATR, len(atr), expected)
	}

	if hasTCK {
		var checksum byte
		for _, value := range atr[1:] {
			checksum ^= value
		}

		if checksum != 0 {
			return nil, fmt.Errorf("%w: invalid TCK", ErrInvalidATR)
		}
	}

	return atr[offset:end], nil
}
