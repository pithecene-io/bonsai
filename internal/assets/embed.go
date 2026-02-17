// Package assets provides access to embedded asset files with
// filesystem-first override resolution.
package assets

import "embed"

//go:embed all:data
var embeddedFS embed.FS

// EmbeddedFS returns the embedded filesystem.
// All files are under the "data/" prefix within this FS.
func EmbeddedFS() embed.FS {
	return embeddedFS
}
