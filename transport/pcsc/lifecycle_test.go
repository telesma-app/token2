package pcsc

import (
	"context"
	"errors"
	"testing"
	"time"

	nativepcsc "github.com/telesma-app/pcsc"
)

type blockingCard struct {
	scriptedCard
	started chan struct{}
	closed  chan struct{}
}

func (c *blockingCard) Transmit(ctx context.Context, _ []byte) ([]byte, error) {
	close(c.started)

	select {
	case <-c.closed:
		return nil, nativepcsc.ErrClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *blockingCard) Close() error {
	close(c.closed)
	return nil
}

func TestDeviceCloseInterruptsConfiguration(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	card := &blockingCard{started: make(chan struct{}), closed: make(chan struct{})}
	device := &Device{card: card}
	readResult := make(chan error, 1)

	go func() {
		_, err := device.Configuration(ctx)
		readResult <- err
	}()

	select {
	case <-card.started:
	case <-time.After(5 * time.Second):
		t.Fatal("Configuration did not start card I/O")
	}

	closeResult := make(chan error, 1)
	go func() { closeResult <- device.Close() }()

	select {
	case err := <-closeResult:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close waited for the blocked card I/O")
	}

	select {
	case err := <-readResult:
		if !errors.Is(err, nativepcsc.ErrClosed) {
			t.Fatalf("Configuration error = %v, want ErrClosed", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not interrupt Configuration")
	}
}
