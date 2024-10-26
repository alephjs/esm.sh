package server

import (
	"errors"
	"io"
	"net/url"
	"os"
	"path"
	"strings"

	esbuild "github.com/evanw/esbuild/pkg/api"
	esbuild_config "github.com/ije/esbuild-internal/config"
	"github.com/ije/esbuild-internal/js_ast"
	"github.com/ije/esbuild-internal/js_parser"
	"github.com/ije/esbuild-internal/logger"
)

var jsExts = []string{".js", ".mjs", ".jsx", ".ts", ".mts", ".tsx", ".cjs", ".cts"}

// stripModuleExt strips the module extension from the given string.
func stripModuleExt(s string, exts ...string) string {
	if len(exts) == 0 {
		exts = jsExts
	}
	for _, ext := range exts {
		if strings.HasSuffix(s, ext) {
			return s[:len(s)-len(ext)]
		}
	}
	return s
}

// validateJSFile validates the given javascript file.
func validateJSFile(filename string) (isESM bool, namedExports []string, err error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return
	}
	log := logger.NewDeferLog(logger.DeferLogNoVerboseOrDebug, nil)
	parserOpts := js_parser.OptionsFromConfig(&esbuild_config.Options{
		JSX: esbuild_config.JSXOptions{
			Parse: endsWith(filename, ".jsx", ".tsx"),
		},
		TS: esbuild_config.TSOptions{
			Parse: endsWith(filename, ".ts", ".mts", ".cts", ".tsx"),
		},
	})
	ast, pass := js_parser.Parse(log, logger.Source{
		Index:          0,
		KeyPath:        logger.Path{Text: "<stdin>"},
		PrettyPath:     "<stdin>",
		Contents:       string(data),
		IdentifierName: "stdin",
	}, parserOpts)
	if !pass {
		err = errors.New("invalid syntax, require javascript/typescript")
		return
	}
	isESM = ast.ExportsKind == js_ast.ExportsESM
	namedExports = make([]string, len(ast.NamedExports))
	i := 0
	for name := range ast.NamedExports {
		namedExports[i] = name
		i++
	}
	return
}

// minify minifies the given javascript code.
func minify(code string, target esbuild.Target, loader esbuild.Loader) ([]byte, error) {
	ret := esbuild.Transform(code, esbuild.TransformOptions{
		Target:            target,
		Format:            esbuild.FormatESModule,
		Platform:          esbuild.PlatformBrowser,
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		LegalComments:     esbuild.LegalCommentsExternal,
		Loader:            loader,
	})
	if len(ret.Errors) > 0 {
		return nil, errors.New(ret.Errors[0].Text)
	}

	return concatBytes(ret.LegalComments, ret.Code), nil
}

// bundleRemoteModule builds the remote module and it's submodules.
func bundleRemoteModule(npmrc *NpmRC, entry string, importMap ImportMap, fetcher *Fetcher) (js []byte, css []byte, sourceCodes [][]byte, err error) {
	if !isHttpSepcifier(entry) {
		err = errors.New("require a remote module")
		return
	}
	entryUrl, err := url.Parse(entry)
	if err != nil {
		err = errors.New("invalid enrtry, require a valid url")
		return
	}
	ret := esbuild.Build(esbuild.BuildOptions{
		EntryPoints:      []string{entry},
		Target:           esbuild.ESNext,
		Format:           esbuild.FormatESModule,
		Platform:         esbuild.PlatformBrowser,
		JSX:              esbuild.JSXPreserve,
		Bundle:           true,
		MinifyWhitespace: true,
		Outdir:           "/esbuild",
		Write:            false,
		Plugins: []esbuild.Plugin{
			{
				Name: "http-loader",
				Setup: func(build esbuild.PluginBuild) {
					build.OnResolve(esbuild.OnResolveOptions{Filter: ".*"}, func(args esbuild.OnResolveArgs) (esbuild.OnResolveResult, error) {
						path, _ := importMap.Resolve(args.Path)
						if isRelativeSpecifier(args.Path) && isHttpSepcifier(args.Importer) {
							u, e := url.Parse(args.Importer)
							if e == nil {
								path = u.ResolveReference(&url.URL{Path: args.Path}).String()
							}
						}
						if isHttpSepcifier(path) {
							u, e := url.Parse(path)
							if e == nil {
								if u.Host == entryUrl.Host && u.Scheme == entryUrl.Scheme {
									return esbuild.OnResolveResult{Path: path, Namespace: "http"}, nil
								}
							}
						}
						return esbuild.OnResolveResult{Path: path, External: true}, nil
					})
					build.OnLoad(esbuild.OnLoadOptions{Filter: ".*", Namespace: "http"}, func(args esbuild.OnLoadArgs) (esbuild.OnLoadResult, error) {
						url, err := url.Parse(args.Path)
						if err != nil {
							return esbuild.OnLoadResult{}, err
						}
						resp, err := fetcher.Fetch(url)
						if err != nil {
							return esbuild.OnLoadResult{}, errors.New("failed to fetch module " + args.Path + ": " + err.Error())
						}
						defer resp.Body.Close()
						if resp.StatusCode != 200 {
							return esbuild.OnLoadResult{}, errors.New("failed to fetch module " + args.Path + ": " + resp.Status)
						}
						data, err := io.ReadAll(resp.Body)
						if err != nil {
							return esbuild.OnLoadResult{}, errors.New("failed to fetch module " + args.Path)
						}
						sourceCodes = append(sourceCodes, data)
						code := string(data)
						loader := esbuild.LoaderJS
						switch path.Ext(url.Path) {
						case ".js", ".mjs", ".cjs":
							loader = esbuild.LoaderJS
						case ".ts", ".mts", ".cts":
							loader = esbuild.LoaderTS
						case ".jsx":
							loader = esbuild.LoaderJSX
						case ".tsx":
							loader = esbuild.LoaderTSX
						case ".css":
							loader = esbuild.LoaderCSS
						case ".json":
							loader = esbuild.LoaderJSON
						case ".vue":
							vueVersion, err := npmrc.getVueLoaderVersion(importMap)
							if err != nil {
								return esbuild.OnLoadResult{}, err
							}
							ret, err := npmrc.preTransform("vue", vueVersion, args.Path, code)
							if err != nil {
								return esbuild.OnLoadResult{}, err
							}
							code = ret.Code
						case ".svelte":
							svelteVersion, err := npmrc.getSvelteLoaderVersion(importMap)
							if err != nil {
								return esbuild.OnLoadResult{}, err
							}
							ret, err := npmrc.preTransform("svelte", svelteVersion, args.Path, code)
							if err != nil {
								return esbuild.OnLoadResult{}, err
							}
							code = ret.Code
						}
						return esbuild.OnLoadResult{Contents: &code, Loader: loader}, nil
					})
				},
			},
		},
	})
	if len(ret.Errors) > 0 {
		err = errors.New(ret.Errors[0].Text)
		return
	}
	for _, file := range ret.OutputFiles {
		if strings.HasSuffix(file.Path, ".js") {
			js = file.Contents
		} else if strings.HasSuffix(file.Path, ".css") {
			css = file.Contents
		}
	}
	return
}
