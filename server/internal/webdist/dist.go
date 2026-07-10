// Package webdist embeds the built SPA. The build flow copies web/dist/*
// into dist/ before go build (go:embed cannot reach outside the module).
// The committed placeholder dist/index.html keeps go build/vet/test working
// on a fresh clone without a frontend build; real build output is never
// committed.
package webdist

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

// FS returns the SPA files rooted at the dist directory.
func FS() fs.FS {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic(err) // unreachable: dist is embedded at compile time
	}
	return sub
}
