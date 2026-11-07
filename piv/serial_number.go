package piv

import (
	"context"
	"fmt"

	"github.com/telesma-app/iso7816"
	"github.com/telesma-app/piv/protocol"
)

// ReadSerialSuffix reads the last eight decimal digits of the TOKEN2 device
// serial number. The PIV applet returns four packed-BCD bytes; leading zeros
// are preserved. This is not the complete device serial. Use token2/transport
// identity readers for the full serial, which requires another applet.
func ReadSerialSuffix(ctx context.Context, transport VendorTransport) (string, error) {
	response, err := transport.Exchange(ctx, iso7816.Command{
		INS: 0xf8, Le: 256, Encoding: iso7816.EncodingShort,
	})
	if err != nil {
		return "", err
	}

	if err := response.APDUError(); err != nil {
		return "", err
	}

	if len(response.Data) != 4 {
		return "", fmt.Errorf("%w: TOKEN2 PIV serial suffix must contain four packed-BCD bytes", protocol.ErrInvalidResponse)
	}

	var digits [8]byte

	for i, value := range response.Data {
		high, low := value>>4, value&0x0f
		if high > 9 || low > 9 {
			return "", fmt.Errorf("%w: TOKEN2 PIV serial suffix contains a non-decimal BCD digit", protocol.ErrInvalidResponse)
		}

		digits[i*2], digits[i*2+1] = '0'+high, '0'+low
	}

	return string(digits[:]), nil
}
