package token2

import (
	"errors"
	"fmt"

	"github.com/telesma-app/token2/internal/protocol"
)

// ErrInvalidSerialResponse reports a malformed serial-number response received
// from a Token2 device.
var ErrInvalidSerialResponse = errors.New("token2: invalid serial-number response")

// ParseSerialNumber parses the TAG-LENGTH-VALUE response returned by the
// Token2 serial-number command.
func ParseSerialNumber(response []byte) (string, error) {
	if len(response) < 2 {
		return "", fmt.Errorf("%w: got %d bytes, need at least 2", ErrInvalidSerialResponse, len(response))
	}
	if response[0] != protocol.SerialResponseTag {
		return "", fmt.Errorf("%w: unexpected tag %02x", ErrInvalidSerialResponse, response[0])
	}

	length := int(response[1])
	if length > len(response)-2 {
		return "", fmt.Errorf(
			"%w: declared length %d exceeds payload length %d",
			ErrInvalidSerialResponse,
			length,
			len(response)-2,
		)
	}

	return string(response[2 : 2+length]), nil
}
