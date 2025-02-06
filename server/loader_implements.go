package server

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/esm-dev/esm.sh/server/common"
	"github.com/ije/gox/sync"
	"github.com/ije/gox/term"
)

var (
	compileSyncMap   = sync.Map{}
	regexpSveltePath = regexp.MustCompile(`/\*?svelte@([~\^]?[\w\+\-\.]+)(/|\?|&|$)`)
	regexpVuePath    = regexp.MustCompile(`/\*?vue@([~\^]?[\w\+\-\.]+)(/|\?|&|$)`)
)

func transformSvelte(npmrc *NpmRC, svelteVersion string, filename string, code string) (output *LoaderOutput, err error) {
	loaderExecPath := path.Join(npmrc.StoreDir(), "svelte@"+svelteVersion, "loader.js")

	once, _ := compileSyncMap.LoadOrStore(loaderExecPath, &sync.Once{})
	err = once.(*sync.Once).Do(func() (err error) {
		if !existsFile(loaderExecPath) {
			if DEBUG {
				fmt.Println(term.Dim("Compiling svelte loader..."))
			}
			err = compileSvelteLoader(npmrc, svelteVersion, loaderExecPath)
		}
		return
	})
	if err != nil {
		err = errors.New("failed to compile svelte loader: " + err.Error())
		return
	}

	return runLoader(loaderExecPath, filename, code)
}

func compileSvelteLoader(npmrc *NpmRC, svelteVersion string, loaderExecPath string) (err error) {
	wd := path.Join(npmrc.StoreDir(), "svelte@"+svelteVersion)

	// install svelte
	pkgJson, err := npmrc.installPackage(Package{Name: "svelte", Version: svelteVersion})
	if err != nil {
		return
	}
	npmrc.installDependencies(wd, pkgJson, false, nil)

	loaderJS := `
	  import { compile } from "svelte/compiler";
	  const { stdin, stdout } = Deno;
	  const write = data => stdout.write(new TextEncoder().encode(data));
	  try {
	    let sourceCode = "";
	    for await (const text of stdin.readable.pipeThrough(new TextDecoderStream())) {
	      sourceCode += text;
	    }
	    const { js } = compile(sourceCode, { filename: Deno.args[0], css: "injected" });
	    await write("1\n" + js.code);
	  } catch (err) {
	    await write("0\n" + err.message);
	  }
	`
	err = buildLoader(wd, loaderJS, loaderExecPath)
	return
}

func resolveSvelteVersion(npmrc *NpmRC, importMap common.ImportMap) (svelteVersion string, err error) {
	svelteVersion = "5"
	if len(importMap.Imports) > 0 {
		sveltePath, ok := importMap.Imports["svelte"]
		if ok {
			a := regexpSveltePath.FindAllStringSubmatch(sveltePath, 1)
			if len(a) > 0 {
				svelteVersion = a[0][1]
			}
		}
	}
	if !isExactVersion(svelteVersion) {
		var info *PackageJSON
		info, err = npmrc.getPackageInfo("svelte", svelteVersion)
		if err != nil {
			return
		}
		svelteVersion = info.Version
	}
	if semverLessThan(svelteVersion, "4.0.0") {
		err = errors.New("unsupported svelte version, only 4.0.0+ is supported")
	}
	return
}

func generateUnoCSS(npmrc *NpmRC, configCSS string, content string) (output *LoaderOutput, err error) {
	loaderVersion := "0.4.3"
	loaderExecPath := path.Join(config.WorkDir, "bin", "unocss-"+loaderVersion)

	once, _ := compileSyncMap.LoadOrStore(loaderExecPath, &sync.Once{})
	err = once.(*sync.Once).Do(func() (err error) {
		if !existsFile(loaderExecPath) {
			if DEBUG {
				fmt.Println(term.Dim("Compiling unocss loader..."))
			}
			err = compileUnocssLoader(npmrc, loaderVersion, loaderExecPath)
		}
		return
	})
	if err != nil {
		err = errors.New("failed to compile unocss engine: " + err.Error())
		return
	}

	outBuf := bufferPool.Get().(*bytes.Buffer)
	errBuf := bufferPool.Get().(*bytes.Buffer)
	defer func() {
		outBuf.Reset()
		errBuf.Reset()
		bufferPool.Put(outBuf)
		bufferPool.Put(errBuf)
	}()
	c := exec.Command(loaderExecPath, strconv.Itoa(len(configCSS)), path.Join(config.WorkDir, "cache/unocss"))
	c.Dir = os.TempDir()
	c.Stdin = strings.NewReader(configCSS + content)
	c.Stdout = outBuf
	c.Stderr = errBuf
	err = c.Run()
	if err != nil {
		if errBuf.Len() > 0 {
			err = errors.New(errBuf.String())
		}
		return
	}
	if outBuf.Len() < 2 {
		err = errors.New("bad loader output")
		return
	}
	data := outBuf.Bytes()
	if data[0] != '1' {
		err = errors.New(string(data[2:]))
		return
	}
	return &LoaderOutput{Lang: "css", Code: string(data[2:])}, nil
}

func compileUnocssLoader(npmrc *NpmRC, loaderVersion string, loaderExecPath string) (err error) {
	wd := path.Join(npmrc.StoreDir(), "@esm.sh/unocss@"+loaderVersion)

	// install @esm.sh/unocss
	pkgJson, err := npmrc.installPackage(Package{Name: "@esm.sh/unocss", Version: loaderVersion})
	if err != nil {
		return
	}
	npmrc.installDependencies(wd, pkgJson, false, nil)

	loaderJS := `
	  import { generate } from "@esm.sh/unocss";
	  const { stdin, stdout } = Deno;
	  const write = data => stdout.write(new TextEncoder().encode(data));
	  const iconLoader = async (collectionName) => {
	    const { UntarStream } = await import("jsr:@std/tar/untar-stream");
			const jsonRes = await fetch("https://registry.npmjs.org/@iconify-json/" + collectionName + "/latest")
			if (jsonRes.status !== 200) {
				jsonRes.body.cancel()
				throw new Error("Failed to fetch @iconify-json/" + collectionName)
			}
			const { dist } = await jsonRes.json()
			const tgzRes = await fetch(dist.tarball)
			if (tgzRes.status !== 200) {
				tgzRes.body.cancel()
				throw new Error("Failed to fetch tarball of @iconify-json/" + collectionName)
			}
			for await (const entry of tgzRes.body.pipeThrough(new DecompressionStream("gzip")).pipeThrough(new UntarStream())) {
				if (entry.path === "package/icons.json" ) {
					return await new Response(entry.readable).json()
				} else {
					entry.readable.cancel()
				}
			}
			throw new Error("icons.json not found in @iconify-json/" + collectionName)
	  }
	  try {
	    let content = "";
	    for await (const text of stdin.readable.pipeThrough(new TextDecoderStream())) {
	      content += text;
	    }
	    let configCSS = undefined;
	    const n = Number(Deno.args[0]);
	    if (n > 0) {
	      configCSS = content.slice(0, n);
	      content = content.slice(n);
	    }
	    const code = await generate(content, { configCSS, iconLoader, customCacheDir: Deno.args[1] });
	    await write("1\n" + code);
	  } catch (err) {
	    await write("0\n" + err.message);
	  }
	`
	err = buildLoader(wd, loaderJS, path.Join(wd, "loader.js"))
	if err != nil {
		return
	}

	_, err = run(
		"deno", "compile",
		"--no-config",
		"--no-lock",
		"--no-check",
		"--include=jsr:@std/tar/untar-stream",
		"--no-prompt",
		"--allow-read="+config.WorkDir+"/cache",
		"--allow-write="+config.WorkDir+"/cache",
		"--allow-net=registry.npmjs.org,fonts.googleapis.com",
		"--quiet",
		"--output", loaderExecPath,
		path.Join(wd, "loader.js"),
	)
	if err != nil {
		err = fmt.Errorf("failed to compile %s: %s", path.Base(loaderExecPath), err.Error())
	}
	return
}

func transformVue(npmrc *NpmRC, vueVersion string, filename string, code string) (output *LoaderOutput, err error) {
	loaderVersion := "1.0.1" // @esm.sh/vue-compiler
	loaderExecPath := path.Join(npmrc.StoreDir(), "@vue/compiler-sfc@"+vueVersion, "loader-"+loaderVersion+".js")

	once, _ := compileSyncMap.LoadOrStore(loaderExecPath, &sync.Once{})
	err = once.(*sync.Once).Do(func() (err error) {
		if !existsFile(loaderExecPath) {
			if DEBUG {
				fmt.Println(term.Dim("Compiling vue loader..."))
			}
			err = compileVueLoader(npmrc, vueVersion, loaderVersion, loaderExecPath)
		}
		return
	})
	if err != nil {
		err = errors.New("failed to compile vue loader: " + err.Error())
		return
	}

	return runLoader(loaderExecPath, filename, code)
}

func compileVueLoader(npmrc *NpmRC, vueVersion string, loaderVersion, loaderExecPath string) (err error) {
	wd := path.Join(npmrc.StoreDir(), "@vue/compiler-sfc@"+vueVersion)

	// install vue sfc compiler
	pkgJson, err := npmrc.installPackage(Package{Name: "@vue/compiler-sfc", Version: vueVersion})
	if err != nil {
		return
	}
	npmrc.installDependencies(wd, pkgJson, false, nil)
	npmrc.installDependencies(wd, &PackageJSON{Dependencies: map[string]string{"@esm.sh/vue-compiler": loaderVersion}}, false, nil)

	loaderJS := `
	  import * as vueCompilerSFC from "@vue/compiler-sfc";
	  import { transform } from "@esm.sh/vue-compiler";
	  const { stdin, stdout } = Deno;
	  const write = data => stdout.write(new TextEncoder().encode(data));
	  try {
	    let sourceCode = "";
	    for await (const text of stdin.readable.pipeThrough(new TextDecoderStream())) {
	      sourceCode += text;
	    }
	    const { lang, code } = await transform(Deno.args[0], sourceCode, { imports: { "@vue/compiler-sfc": vueCompilerSFC } });
	    await write((lang === "ts" ? '2' : '1') + '\n' + code);
	  } catch (err) {
	    await write("0\n" + err.message);
	  }
	`
	err = buildLoader(wd, loaderJS, loaderExecPath)
	return
}

func resolveVueVersion(npmrc *NpmRC, importMap common.ImportMap) (vueVersion string, err error) {
	vueVersion = "3"
	if len(importMap.Imports) > 0 {
		vuePath, ok := importMap.Imports["vue"]
		if ok {
			a := regexpVuePath.FindAllStringSubmatch(vuePath, 1)
			if len(a) > 0 {
				vueVersion = a[0][1]
			}
		}
	}
	if !isExactVersion(vueVersion) {
		var info *PackageJSON
		info, err = npmrc.getPackageInfo("vue", vueVersion)
		if err != nil {
			return
		}
		vueVersion = info.Version
	}
	if semverLessThan(vueVersion, "3.0.0") {
		err = errors.New("unsupported vue version, only 3.0.0+ is supported")
	}
	return
}
