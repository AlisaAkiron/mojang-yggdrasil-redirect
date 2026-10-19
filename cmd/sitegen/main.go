// Command sitegen renders the Markdown docs into the static site the api
// command embeds:
//
//	go run ./cmd/sitegen -src site -out site/dist
package main

import (
	"flag"
	"log"

	"github.com/AlisaAkiron/mojang-yggdrasil-redirect/internal/sitegen"
)

func main() {
	log.SetFlags(0)
	src := flag.String("src", "site", "site source directory (config.json, layout.html, content/, static/)")
	out := flag.String("out", "site/dist", "output directory; its contents are replaced")
	flag.Parse()

	if err := sitegen.Build(*src, *out); err != nil {
		log.Fatalf("sitegen: %v", err)
	}
	log.Printf("sitegen: built %s -> %s", *src, *out)
}
