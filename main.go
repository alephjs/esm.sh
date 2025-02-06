package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/esm-dev/esm.sh/cli"
)

const helpMessage = "\033[30mesm.sh - A no-build CDN for modern web development.\033[0m" + `

Usage: esm.sh [command] <options>

Commands:
  i, add [...pakcage]   Alias for 'esm.sh im add'.
  im, importmap         Manage "importmap" script.
  init                  Create a new no-build web app with esm.sh CDN.
  serve                 Serve a no-build web app with esm.sh CDN, HMR, transforming TS/Vue/Svelte on the fly.
`

//go:embed cli/internal
//go:embed cli/demo
var fs embed.FS

func main() {
	if len(os.Args) < 2 {
		fmt.Print(helpMessage)
		return
	}
	switch command := os.Args[1]; command {
	case "i", "add":
		cli.ManageImportMap("add")
	case "im", "importmap":
		cli.ManageImportMap("")
	case "init":
		cli.Init(&fs)
	case "serve":
		cli.Serve(&fs)
	default:
		fmt.Print(helpMessage)
	}
}
