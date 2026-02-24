// Package xio provides small I/O helper functions.
package xio

import "io"

// DiscardClose closes c and discards the error.
// Use it in defer statements to satisfy errcheck without noise:
//
//	defer xio.DiscardClose(f)
func DiscardClose(c io.Closer) { _ = c.Close() }
