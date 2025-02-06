package server

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"

	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/ije/gox/term"
)

type LoaderOutput struct {
	Lang  string `json:"lang"`
	Code  string `json:"code"`
	Error string `json:"error"`
}

func runLoader(loaderJsPath string, filename string, code string) (output *LoaderOutput, err error) {
	stdout, recycle := NewBuffer()
	defer recycle()
	stderr, recycle := NewBuffer()
	defer recycle()
	cmd := exec.Command(
		path.Join(config.WorkDir, "bin", "deno"), "run",
		"--no-config",
		"--no-lock",
		"--cached-only",
		"--no-prompt",
		"--allow-read=.",
		"--quiet",
		loaderJsPath,
		filename, // args[0]
	)
	cmd.Dir = os.TempDir()
	cmd.Stdin = strings.NewReader(code)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err = cmd.Run()
	if err != nil {
		if stderr.Len() > 0 {
			err = errors.New(stderr.String())
		}
		return
	}
	if stdout.Len() < 2 {
		err = errors.New("bad loader output")
		return
	}
	data := stdout.Bytes()
	if data[0] != '1' && data[0] != '2' {
		err = errors.New(string(data[2:]))
		return
	}
	lang := "js"
	if data[0] == '2' {
		lang = "ts"
	}
	return &LoaderOutput{Lang: lang, Code: string(data[2:])}, nil
}

func buildLoader(wd, loaderJs, outfile string) (err error) {
	ret := esbuild.Build(esbuild.BuildOptions{
		Outfile:           outfile,
		Stdin:             &esbuild.StdinOptions{Contents: loaderJs, ResolveDir: wd},
		Platform:          esbuild.PlatformBrowser,
		Format:            esbuild.FormatESModule,
		Target:            esbuild.ESNext,
		Bundle:            true,
		MinifySyntax:      true,
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		Write:             true,
		PreserveSymlinks:  true,
		Plugins: []esbuild.Plugin{{
			Name: "resolver",
			Setup: func(build esbuild.PluginBuild) {
				build.OnResolve(esbuild.OnResolveOptions{Filter: ".*"}, func(args esbuild.OnResolveArgs) (esbuild.OnResolveResult, error) {
					if strings.HasPrefix(args.Path, "node:") || nodeBuiltinModules[args.Path] {
						return esbuild.OnResolveResult{Path: "node:" + strings.TrimPrefix(args.Path, "node:"), External: true}, nil
					}
					if strings.HasPrefix(args.Path, "jsr:") {
						return esbuild.OnResolveResult{Path: args.Path, External: true}, nil
					}
					return esbuild.OnResolveResult{}, nil
				})
			},
		}},
	})
	if len(ret.Errors) > 0 {
		err = errors.New(ret.Errors[0].Text)
	}
	return
}

func installDeno(version string) (installedVersion string, err error) {
	binDir := path.Join(config.WorkDir, "bin")
	err = ensureDir(binDir)
	if err != nil {
		return
	}

	// check local installed deno
	installedDeno, err := exec.LookPath("deno")
	if err == nil {
		output, err := run(installedDeno, "eval", "console.log(Deno.version.deno)")
		if err == nil {
			v := strings.TrimSpace(string(output))
			if !semverLessThan(v, "1.45") {
				err = os.Symlink(installedDeno, path.Join(binDir, "deno"))
				if err != nil && !os.IsExist(err) {
					return "", err
				}
				return v, nil
			}
		}
	}

	if existsFile(path.Join(binDir, "deno")) {
		output, err := run(path.Join(binDir, "deno"), "eval", "console.log(Deno.version.deno)")
		if err == nil {
			version := strings.TrimSpace(string(output))
			if !semverLessThan(version, version) {
				return version, nil
			}
		}
	}

	url, err := getDenoInstallURL(version)
	if err != nil {
		return
	}

	if DEBUG {
		fmt.Println(term.Dim(fmt.Sprintf("Downloading %s...", path.Base(url))))
	}

	res, err := http.Get(url)
	if err != nil {
		return
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return "", fmt.Errorf("failed to download Deno install package: %s", res.Status)
	}

	tmpFile := path.Join(binDir, "deno.zip")
	defer os.Remove(tmpFile)

	f, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	_, err = io.Copy(f, res.Body)
	if err != nil {
		return
	}

	zr, err := zip.OpenReader(tmpFile)
	if err != nil {
		return
	}
	defer zr.Close()

	for _, zf := range zr.File {
		if zf.Name == "deno" {
			r, err := zf.Open()
			if err != nil {
				return "", err
			}
			defer r.Close()

			f, err := os.OpenFile(path.Join(binDir, "deno"), os.O_CREATE|os.O_WRONLY, 0755)
			if err != nil {
				return "", err
			}
			defer f.Close()

			_, err = io.Copy(f, r)
			if err != nil {
				return "", err
			}
			break
		}
	}

	return version, nil
}

func getDenoInstallURL(version string) (string, error) {
	var arch string
	var os string

	switch runtime.GOARCH {
	case "arm64":
		arch = "aarch64"
	case "amd64", "386":
		arch = "x86_64"
	default:
		return "", errors.New("unsupported architecture: " + runtime.GOARCH)
	}

	switch runtime.GOOS {
	case "darwin":
		os = "apple-darwin"
	case "linux":
		os = "unknown-linux-gnu"
	// case "windows":
	// 	os = "pc-windows-msvc"
	default:
		return "", errors.New("unsupported os: " + runtime.GOOS)
	}

	return fmt.Sprintf("https://github.com/denoland/deno/releases/download/v%s/deno-%s-%s.zip", version, arch, os), nil
}
