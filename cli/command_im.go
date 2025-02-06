package cli

import (
	"fmt"
	"os"
)

const helpMessage = "\033[30mImport Map Management CLI with esm.sh CDN.\033[0m" + `

Usage: esm.sh im [sub-command] <options>

Sub Commands:
  add    [...packages]   Add packages to "importmap" script
  update [...packages]   Update packages in "importmap" script
`

// Manage `importmap` script
func ManageImportMap(subCommand string) {
	if len(os.Args) < 3 {
		fmt.Print(helpMessage)
		return
	}
	var packages []string
	if subCommand == "" {
		subCommand = os.Args[2]
		packages = os.Args[3:]
	} else {
		packages = os.Args[2:]
	}
	switch subCommand {
	case "add":
		if len(packages) == 0 {
			return
		}
		fmt.Println(packages)
	case "update":
		fmt.Println(packages)
	default:
		fmt.Printf("Unknown sub command \"%s\"\n", subCommand)
	}
}
