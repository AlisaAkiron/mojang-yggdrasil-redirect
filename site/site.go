// Package site holds the docs sources (content/, static/, layout.html) and
// embeds the generated output in dist/. Regenerate it after editing the docs:
//
//	go generate ./site
package site

import (
	"embed"
	"io/fs"
)

//go:generate go run ../cmd/sitegen -src . -out dist

//go:embed all:dist
var dist embed.FS

// Dist returns the generated site. It is empty (bar dist/.gitkeep) until
// cmd/sitegen has run.
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
