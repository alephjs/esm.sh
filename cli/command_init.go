package cli

import (
	"bytes"
	"embed"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ije/gox/term"
)

var frameworks = []string{
	"Vanilla",
	"React",
	"Preact",
	"Vue",
	"Svelte",
}

var cssFrameworks = []string{
	"Vanilla",
	"UnoCSS",
}

var langVariants = []string{
	"JavaScript",
	"TypeScript",
}

// Create a new no-build web app with esm.sh CDN.
func Init(fs *embed.FS) {
	framework := flag.String("framework", "", "javascript framework")
	cssFramework := flag.String("css-framework", "", "CSS framework")
	lang := flag.String("lang", "", "language")
	projectName, _ := parseCommandFlag()
	raw := &termRaw{}

	if projectName == "" {
		projectName = term.Input(raw, "Project name:", "esm-app")
	}

	if *framework == "" {
		*framework = term.Select(raw, "Select a framework:", frameworks)
	} else if !includes(frameworks, *framework) {
		fmt.Println("Invalid framework: ", *framework)
		os.Exit(1)
	}

	if *cssFramework == "" {
		*cssFramework = term.Select(raw, "Select a CSS framework:", cssFrameworks)
	} else if !includes(cssFrameworks, *cssFramework) {
		*cssFramework = cssFrameworks[0]
	}

	if *lang == "" {
		*lang = term.Select(raw, "Select a variant:", langVariants)
	} else if !includes(langVariants, *lang) {
		*lang = langVariants[0]
	}

	_, err := os.Lstat(projectName)
	if err == nil || os.IsExist(err) {
		if !term.Confirm(raw, "The directory already exists, do you want to overwrite it?") {
			fmt.Println(term.Dim("Canceled."))
			return
		}
	}

	dir := "cli/demo/" + strings.ToLower(*framework)
	if *cssFramework == "UnoCSS" {
		dir = "cli/demo/with-unocss/" + strings.ToLower(*framework)
	}
	err = walkEmbedFS(fs, dir, func(filename string) error {
		savePath := projectName + strings.TrimPrefix(filename, dir)
		os.MkdirAll(filepath.Dir(savePath), 0755)
		if *lang == "JavaScript" {
			if (strings.HasSuffix(savePath, ".ts") || strings.HasSuffix(savePath, ".tsx")) && !strings.HasSuffix(savePath, ".d.ts") {
				data, err := fs.ReadFile(filename)
				if err != nil {
					return err
				}
				if strings.HasSuffix(savePath, ".tsx") {
					savePath = strings.TrimSuffix(savePath, ".tsx") + ".jsx"
				} else {
					savePath = strings.TrimSuffix(savePath, ".ts") + ".js"
				}
				data = bytes.ReplaceAll(data, []byte(")!"), []byte(")"))
				data = bytes.ReplaceAll(data, []byte(".ts\""), []byte(".js\""))
				data = bytes.ReplaceAll(data, []byte(".tsx\""), []byte(".jsx\""))
				return os.WriteFile(savePath, data, 0644)
			} else if strings.HasSuffix(savePath, ".html") {
				data, err := fs.ReadFile(filename)
				if err != nil {
					return err
				}
				data = bytes.ReplaceAll(data, []byte(".ts\""), []byte(".js\""))
				data = bytes.ReplaceAll(data, []byte(".tsx\""), []byte(".jsx\""))
				return os.WriteFile(savePath, data, 0644)
			} else if strings.HasSuffix(savePath, ".vue") || strings.HasSuffix(savePath, ".svelte") {
				data, err := fs.ReadFile(filename)
				if err != nil {
					return err
				}
				data = bytes.ReplaceAll(data, []byte(" lang=\"ts\""), []byte(""))
				return os.WriteFile(savePath, data, 0644)
			}
		}
		f, err := fs.Open(filename)
		if err != nil {
			return err
		}
		defer f.Close()
		w, err := os.Create(savePath)
		if err != nil {
			return err
		}
		defer w.Close()
		_, err = io.Copy(w, f)
		return err
	})
	if err != nil {
		fmt.Println(term.Red("✘ Failed to create project: " + err.Error()))
		os.Exit(1)
	}

	fmt.Println(" ")
	fmt.Println(term.Dim("Project created successfully."))
	fmt.Println(term.Dim("We highly recommend installing our VS Code extension for a better DX: https://link.esm.sh/vsce"))
	fmt.Println(term.Dim("To start the app in development mode, run:"))
	fmt.Println(" ")
	fmt.Print(term.Dim("$ ") + "cd " + projectName)
	if strings.Contains(os.Args[0], "/node_modules/") {
		fmt.Println(" && npx esm.sh serve")
	} else {
		fmt.Println(" && esm.sh serve")
	}
	fmt.Println(" ")
}

func walkEmbedFS(fs *embed.FS, dir string, cb func(filename string) error) error {
	entries, err := fs.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			err = walkEmbedFS(fs, dir+"/"+entry.Name(), cb)
			if err != nil {
				return err
			}
		} else {
			err = cb(dir + "/" + entry.Name())
			if err != nil {
				return err
			}
		}
	}
	return nil
}
