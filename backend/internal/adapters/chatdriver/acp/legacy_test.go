package acp

import (
	"io"
	"testing"
	"time"
)

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

func TestLegacyACPUpdateWindowDoesNotLeakPermits(t *testing.T) {
	transport, _, _ := newLegacyACPTransport(
		nopWriteCloser{Writer: io.Discard},
		io.LimitReader(&zeroReader{}, 0),
	)

	for i := 0; i < updateWindow; i++ {
		transport.updates <- struct{}{}
	}

	acquired := make(chan struct{})
	go func() {
		transport.acquireUpdate()
		close(acquired)
	}()

	select {
	case <-acquired:
		t.Fatal("full update window admitted an untracked notification")
	case <-time.After(25 * time.Millisecond):
	}

	// If the SDK/connection can no longer drain notifications, watchConnection
	// closes this signal. That must unblock the reader without inventing a permit.
	transport.closeUpdates()
	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("connection shutdown did not release blocked update reader")
	}
}

type zeroReader struct{}

func (*zeroReader) Read([]byte) (int, error) { return 0, io.EOF }
