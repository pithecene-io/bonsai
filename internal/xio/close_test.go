package xio_test

import (
	"errors"
	"testing"

	"github.com/pithecene-io/bonsai/internal/xio"
)

type nopCloser struct{}

func (nopCloser) Close() error { return nil }

type errCloser struct{ err error }

func (c errCloser) Close() error { return c.err }

func TestDiscardClose_Nil(t *testing.T) {
	// Should not panic on a successful close.
	xio.DiscardClose(nopCloser{})
}

func TestDiscardClose_Error(t *testing.T) {
	// Should not panic when Close returns an error — the error is discarded.
	xio.DiscardClose(errCloser{err: errors.New("close failed")})
}
