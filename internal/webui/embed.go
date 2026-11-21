// Package webui holds the built web app, compiled into the server binary.
package webui

import (
	"embed"
	"io/fs"
)

// dist is filled by `make build`. The tracked .gitkeep lets the package
// compile before the first build.
//
//go:embed all:dist
var dist embed.FS

// FS returns the built web app with index.html at its root.
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // "dist" is a fixed, valid path
	}
	return sub
}
