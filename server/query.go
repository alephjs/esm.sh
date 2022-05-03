package server

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"esm.sh/server/storage"
	"github.com/ije/gox/utils"
	"github.com/ije/rex"
)

var httpClient = &http.Client{
	Transport: &http.Transport{
		Dial: func(network, addr string) (conn net.Conn, err error) {
			conn, err = net.DialTimeout(network, addr, 15*time.Second)
			if err != nil {
				return conn, err
			}

			// Set a one-time deadline for potential SSL handshaking
			conn.SetDeadline(time.Now().Add(60 * time.Second))
			return conn, nil
		},
		MaxIdleConnsPerHost:   6,
		ResponseHeaderTimeout: 60 * time.Second,
	},
}

// esm query middleware for rex
func query(devMode bool) rex.Handle {
	startTime := time.Now()

	return func(ctx *rex.Context) interface{} {
		pathname := ctx.Path.String()

		// ban malicious requests
		if strings.HasPrefix(pathname, ".") || strings.HasSuffix(pathname, ".php") {
			return rex.Status(400, "Bad Request")
		}

		// strip loc
		if strings.ContainsRune(pathname, ':') {
			pathname = regLocPath.ReplaceAllString(pathname, "$1")
		}

		// match static routes
		hasBuildVerPrefix := false
		prevBuildVer := ""

		// Build prefix may only be served from "${cdnBasePath}/${buildPrefix}/..."
		if strings.HasPrefix(pathname, cdnBasePath+"/") {
			pathname = strings.TrimPrefix(pathname, cdnBasePath)
			// Check current version
			buildBasePath := fmt.Sprintf("/v%d", VERSION)
			if strings.HasPrefix(pathname, buildBasePath+"/") {
				pathname = strings.TrimPrefix(pathname, buildBasePath)
				hasBuildVerPrefix = true
				// Otheerwise check possible pinned version
			} else if regBuildVersionPath.MatchString(pathname) {
				a := strings.Split(pathname, "/")
				pathname = "/" + strings.Join(a[2:], "/")
				hasBuildVerPrefix = true
				prevBuildVer = a[1]
			}
		} else if basePath != "" {
			if strings.HasPrefix(pathname, basePath+"/") {
				pathname = strings.TrimPrefix(pathname, basePath)
			} else if baseRedirect {
				url := strings.TrimPrefix(ctx.R.URL.String(), basePath)
				url = fmt.Sprintf("%s/%s", basePath, url)
				return rex.Redirect(url, http.StatusTemporaryRedirect)
			} else {
				return rex.Status(404, "not found")
			}
		}

		// match static routess
		switch pathname {
		case "/":
			indexHTML, err := embedFS.ReadFile("server/embed/index.html")
			if err != nil {
				return err
			}
			readme, err := embedFS.ReadFile("README.md")
			if err != nil {
				return err
			}
			readme = bytes.ReplaceAll(readme, []byte("./server/embed/"), []byte(basePath+"/embed/"))
			readme = bytes.ReplaceAll(readme, []byte("./HOSTING.md"), []byte("https://github.com/esm-dev/esm.sh/blob/master/HOSTING.md"))
			readme = bytes.ReplaceAll(readme, []byte("https://esm.sh"), []byte("{origin}"+basePath))
			readmeStrLit := utils.MustEncodeJSON(string(readme))
			html := bytes.ReplaceAll(indexHTML, []byte("'# README'"), readmeStrLit)
			html = bytes.ReplaceAll(html, []byte("{VERSION}"), []byte(fmt.Sprintf("%d", VERSION)))
			html = bytes.ReplaceAll(html, []byte("{basePath}"), []byte(basePath))
			ctx.SetHeader("Cache-Control", fmt.Sprintf("public, max-age=%d", 10*60))
			return rex.Content("index.html", startTime, bytes.NewReader(html))

		case "/status.json":
			buildQueue.lock.RLock()
			q := make([]map[string]interface{}, buildQueue.list.Len())
			i := 0
			for el := buildQueue.list.Front(); el != nil; el = el.Next() {
				t, ok := el.Value.(*queueTask)
				if ok {
					m := map[string]interface{}{
						"stage":      t.stage,
						"createTime": t.createTime.Format(http.TimeFormat),
						"consumers":  len(t.consumers),
						"pkg":        t.Pkg.String(),
						"target":     t.Target,
						"inProcess":  t.inProcess,
						"devMode":    t.DevMode,
						"bundleMode": t.BundleMode,
					}
					if !t.startTime.IsZero() {
						m["startTime"] = t.startTime.Format(http.TimeFormat)
					}
					if len(t.Deps) > 0 {
						m["deps"] = t.Deps.String()
					}
					q[i] = m
					i++
				}
			}
			buildQueue.lock.RUnlock()
			n := 0
			nsTasks.Range(func(key, value interface{}) bool {
				n++
				return false
			})
			return map[string]interface{}{
				"nsTasks":   n,
				"nsChannel": fmt.Sprintf("%d/%d", len(nsChannel), cap(nsChannel)),
				"uptime":    time.Since(startTime).String(),
				"queue":     q[:i],
			}

		case "/error.js":
			switch ctx.Form.Value("type") {
			case "resolve":
				return throwErrorJS(ctx, fmt.Errorf(
					`Can't resolve "%s" (Imported by "%s")`,
					ctx.Form.Value("name"),
					ctx.Form.Value("importer"),
				))
			case "unsupported-nodejs-builtin-module":
				return throwErrorJS(ctx, fmt.Errorf(
					`Unsupported nodejs builtin module "%s" (Imported by "%s")`,
					ctx.Form.Value("name"),
					ctx.Form.Value("importer"),
				))
			default:
				return throwErrorJS(ctx, fmt.Errorf("Unknown error"))
			}

		case "/favicon.ico":
			return rex.Status(404, "not found")
		}

		// serve embed assets
		if strings.HasPrefix(pathname, "/embed/") {
			data, err := embedFS.ReadFile("server" + pathname)
			if err != nil {
				// try `/embed/test/**/*`
				data, err = embedFS.ReadFile(pathname[7:])
			}
			if err == nil {
				ctx.SetHeader("Cache-Control", fmt.Sprintf("public, max-age=%d", 10*60))
				return rex.Content(pathname, startTime, bytes.NewReader(data))
			}
		}

		// serve embed polyfills/types
		if hasBuildVerPrefix {
			data, err := embedFS.ReadFile("server/embed/polyfills" + pathname)
			if err == nil {
				ctx.SetHeader("Cache-Control", "public, max-age=31536000, immutable")
				return rex.Content(pathname, startTime, bytes.NewReader(data))
			}
			data, err = embedFS.ReadFile("server/embed/types" + pathname)
			if err == nil {
				ctx.SetHeader("Content-Type", "application/typescript; charset=utf-8")
				ctx.SetHeader("Cache-Control", "public, max-age=31536000, immutable")
				return rex.Content(pathname, startTime, bytes.NewReader(data))
			}
		}

		// get package info
		reqPkg, _, err := parsePkg(pathname)
		if err != nil {
			status := 500
			message := err.Error()
			if message == "invalid path" {
				status = 400
			} else if strings.HasSuffix(message, "not found") {
				status = 404
			}
			return rex.Status(status, message)
		}

		if (reqPkg.Name == "react" || reqPkg.Name == "preact") && strings.HasSuffix(ctx.R.URL.RawQuery, "/jsx-runtime") {
			ctx.R.URL.RawQuery = strings.TrimSuffix(ctx.R.URL.RawQuery, "/jsx-runtime")
			pathname = fmt.Sprintf("/%s/jsx-runtime", reqPkg.Name)
			reqPkg.Submodule = "jsx-runtime"
		}

		if v := ctx.Form.Value("path"); v != "" {
			reqPkg.Submodule = utils.CleanPath(v)[1:]
		}

		var storageType string
		if reqPkg.Submodule != "" {
			switch path.Ext(pathname) {
			case ".js":
				if hasBuildVerPrefix {
					storageType = "builds"
				}

			// todo: transform ts/jsx/tsx for browser
			case ".ts", ".jsx", ".tsx":
				if hasBuildVerPrefix {
					if strings.HasSuffix(pathname, ".d.ts") {
						storageType = "types"
					}
				} else if len(strings.Split(pathname, "/")) > 2 {
					storageType = "raw"
				}

			case ".json", ".css", ".pcss", ".postcss", ".less", ".sass", ".scss", ".stylus", ".styl", ".wasm", ".xml", ".yaml", ".svg", ".png", ".jpg", ".webp", ".gif", ".eot", ".ttf", ".otf", ".woff", ".woff2":
				if hasBuildVerPrefix {
					if strings.HasSuffix(pathname, ".css") {
						storageType = "builds"
					}
				} else if len(strings.Split(pathname, "/")) > 2 {
					storageType = "raw"
				}
			}
		}

		redirectOrWorkerOrigin := origin(ctx.R.Host, cdnDomain, true)

		// serve raw dist files like CSS that is fetching from unpkg.com
		if storageType == "raw" {
			if !regFullVersionPath.MatchString(pathname) {
				url := fmt.Sprintf("%s/%s", redirectOrWorkerOrigin, reqPkg.String())
				return rex.Redirect(url, http.StatusTemporaryRedirect)
			}
			savePath := path.Join("raw", reqPkg.String())
			exists, size, modtime, err := fs.Exists(savePath)
			if err != nil {
				return rex.Status(500, err.Error())
			}
			if exists {
				r, err := fs.ReadFile(savePath, size)
				if err != nil {
					return rex.Status(500, err.Error())
				}
				if strings.HasSuffix(pathname, ".ts") {
					ctx.SetHeader("Content-Type", "application/typescript")
				}
				ctx.SetHeader("Cache-Control", "public, max-age=31536000, immutable")
				return rex.Content(savePath, modtime, r)
			}
			resp, err := httpClient.Get(fmt.Sprintf("https://unpkg.com/%s", reqPkg.String()))
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode >= 500 {
				return rex.Status(http.StatusBadGateway, "Bad Gateway")
			}
			data, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			if resp.StatusCode >= 400 {
				return rex.Status(resp.StatusCode, string(data))
			}
			err = fs.WriteData(savePath, data)
			if err != nil {
				return err
			}
			for key, values := range resp.Header {
				for _, value := range values {
					ctx.AddHeader(key, value)
				}
			}
			ctx.SetHeader("Cache-Control", "public, max-age=31536000, immutable")
			return data
		}

		// serve build files
		if hasBuildVerPrefix && (storageType == "builds" || storageType == "types") {
			var savePath string
			if prevBuildVer != "" {
				savePath = path.Join(storageType, prevBuildVer, pathname)
			} else {
				savePath = path.Join(storageType, fmt.Sprintf("v%d", VERSION), pathname)
			}

			exists, size, modtime, err := fs.Exists(savePath)
			if err != nil {
				return rex.Status(500, err.Error())
			}

			if exists {
				r, err := fs.ReadFile(savePath, size)
				if err != nil {
					return rex.Status(500, err.Error())
				}
				if storageType == "types" {
					ctx.SetHeader("Content-Type", "application/typescript; charset=utf-8")
				}
				ctx.SetHeader("Cache-Control", "public, max-age=31536000, immutable")
				return rex.Content(savePath, modtime, r)
			}
		}

		// check `deps` query
		deps := PkgSlice{}
		for _, p := range strings.Split(ctx.Form.Value("deps"), ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				m, _, err := parsePkg(p)
				if err != nil {
					if strings.HasSuffix(err.Error(), "not found") {
						continue
					}
					return rex.Status(400, fmt.Sprintf("Invalid deps query: %v not found", p))
				}
				if !deps.Has(m.Name) {
					deps = append(deps, *m)
				}
			}
		}

		// check `alias` query
		alias := map[string]string{}
		for _, p := range strings.Split(ctx.Form.Value("alias"), ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				name, to := utils.SplitByFirstByte(p, ':')
				name = strings.TrimSpace(name)
				to = strings.TrimSpace(to)
				if name != "" && to != "" {
					alias[name] = to
				}
			}
		}

		// determine build target
		target := strings.ToLower(ctx.Form.Value("target"))
		_, targeted := targets[target]
		if !targeted {
			target = getTargetByUA(ctx.R.UserAgent())
		}

		buildVersion := VERSION
		value := ctx.Form.Value("pin")
		if strings.HasPrefix(value, "v") {
			i, err := strconv.Atoi(value[1:])
			if err == nil && i > 0 && i < VERSION {
				buildVersion = i
			}
		}

		isBare := false
		isPkgCss := ctx.Form.Has("css")
		isBundleMode := ctx.Form.Has("bundle")
		isDev := ctx.Form.Has("dev")
		isPined := ctx.Form.Has("pin")
		isWorker := ctx.Form.Has("worker")
		noCheck := ctx.Form.Has("no-check")
		noRequire := ctx.Form.Has("no-require")

		// force react/jsx-dev-runtime and react-refresh into `dev` mode
		if !isDev {
			if (reqPkg.Name == "react" && reqPkg.Submodule == "jsx-dev-runtime") || reqPkg.Name == "react-refresh" {
				isDev = true
			}
		}

		// parse `resolvePrefix`
		if hasBuildVerPrefix {
			a := strings.Split(reqPkg.Submodule, "/")
			if len(a) > 1 && strings.HasPrefix(a[0], "X-") {
				s, err := atobUrl(strings.TrimPrefix(a[0], "X-"))
				if err == nil {
					for _, p := range strings.Split(s, ",") {
						if strings.HasPrefix(p, "alias:") {
							for _, p := range strings.Split(strings.TrimPrefix(p, "alias:"), ",") {
								p = strings.TrimSpace(p)
								if p != "" {
									name, to := utils.SplitByFirstByte(p, ':')
									name = strings.TrimSpace(name)
									to = strings.TrimSpace(to)
									if name != "" && to != "" {
										alias[name] = to
									}
								}
							}
						} else if strings.HasPrefix(p, "deps:") {
							for _, p := range strings.Split(strings.TrimPrefix(p, "deps:"), ",") {
								p = strings.TrimSpace(p)
								if p != "" {
									if strings.HasPrefix(p, "@") {
										scope, name := utils.SplitByFirstByte(p, '_')
										p = scope + "/" + name
									}
									m, _, err := parsePkg(p)
									if err != nil {
										if strings.HasSuffix(err.Error(), "not found") {
											continue
										}
										return throwErrorJS(ctx, err)
									}
									if !deps.Has(m.Name) {
										deps = append(deps, *m)
									}
								}
							}
						}
					}
				}
				reqPkg.Submodule = strings.Join(a[1:], "/")
			}
		}

		// check whether it is `bare` mode
		if hasBuildVerPrefix && endsWith(pathname, ".js") {
			a := strings.Split(reqPkg.Submodule, "/")
			if len(a) > 1 {
				if _, ok := targets[a[0]]; ok {
					submodule := strings.TrimSuffix(strings.Join(a[1:], "/"), ".js")
					if endsWith(submodule, ".bundle") {
						submodule = strings.TrimSuffix(submodule, ".bundle")
						isBundleMode = true
					}
					if endsWith(submodule, ".development") {
						submodule = strings.TrimSuffix(submodule, ".development")
						isDev = true
					}
					if endsWith(submodule, ".nr") {
						submodule = strings.TrimSuffix(submodule, ".nr")
						noRequire = true
					}
					pkgName := path.Base(reqPkg.Name)
					if submodule == pkgName || (strings.HasSuffix(pkgName, ".js") && submodule+".js" == pkgName) {
						submodule = ""
					}
					reqPkg.Submodule = submodule
					target = a[0]
					isBare = true
				}
			}
		}

		typesCdnOrigin := origin(ctx.R.Host, typesCdnDomain, true)

		if hasBuildVerPrefix && storageType == "types" {
			task := &BuildTask{
				CdnOrigin:    typesCdnOrigin,
				BuildVersion: buildVersion,
				Pkg:          *reqPkg,
				Deps:         deps,
				Alias:        alias,
				Target:       "types",
				stage:        "-",
			}
			var savePath string
			findTypesFile := func() (bool, int64, time.Time, error) {
				savePath = path.Join(fmt.Sprintf(
					"types/v%d/%s@%s/%s",
					buildVersion,
					reqPkg.Name,
					reqPkg.Version,
					task.resolvePrefix(),
				), reqPkg.Submodule)
				if strings.HasSuffix(savePath, "~.d.ts") {
					savePath = strings.TrimSuffix(savePath, "~.d.ts")
					ok, _, _, err := fs.Exists(path.Join(savePath, "index.d.ts"))
					if err != nil {
						return false, 0, time.Time{}, err
					}
					if ok {
						savePath = path.Join(savePath, "index.d.ts")
					} else {
						savePath += ".d.ts"
					}
				}
				return fs.Exists(savePath)
			}
			exists, size, modtime, err := findTypesFile()
			if err == nil && !exists {
				c := buildQueue.Add(task)
				select {
				case output := <-c.C:
					if output.err != nil {
						return rex.Status(500, "types: "+output.err.Error())
					}
				case <-time.After(time.Minute):
					buildQueue.RemoveConsumer(task, c)
					return rex.Status(http.StatusRequestTimeout, "timeout, we are transforming the types hardly, please try again later!")
				}
			}
			if err != nil {
				return rex.Status(500, err.Error())
			}
			var r io.ReadSeeker
			r, err = fs.ReadFile(savePath, size)
			if err != nil {
				if os.IsExist(err) {
					return rex.Status(500, err.Error())
				}
				r = bytes.NewReader([]byte("/* fake(empty) types */"))
			}
			ctx.SetHeader("Content-Type", "application/typescript; charset=utf-8")
			ctx.SetHeader("Cache-Control", "public, max-age=31536000, immutable")
			return rex.Content(savePath, modtime, r) // auto close
		}

		buildsCdnOrigin := origin(ctx.R.Host, cdnDomain, false)

		task := &BuildTask{
			CdnOrigin:    buildsCdnOrigin,
			BuildVersion: buildVersion,
			Pkg:          *reqPkg,
			Deps:         deps,
			Alias:        alias,
			Target:       target,
			BundleMode:   isBundleMode || isWorker,
			NoRequire:    noRequire,
			DevMode:      isDev,
			stage:        "init",
		}
		taskID := task.ID()
		esm, err := findModule(taskID)
		if err != nil && err != storage.ErrNotFound {
			return rex.Status(500, err.Error())
		}
		if err == storage.ErrNotFound {
			if !isBare && !isPined {
				// find previous build version
				for i := 0; i < VERSION; i++ {
					id := fmt.Sprintf("v%d/%s", VERSION-(i+1), taskID[len(fmt.Sprintf("v%d/", VERSION)):])
					esm, err = findModule(id)
					if err != nil && err != storage.ErrNotFound {
						return rex.Status(500, err.Error())
					}
					if err == nil {
						taskID = id
						break
					}
				}
			}

			// if the previous build exists and is not pin/bare mode, then build current module in backgound,
			// or wait the current build task for 30 seconds
			if esm != nil {
				// todo: maybe don't build?
				buildQueue.Add(task)
			} else {
				c := buildQueue.Add(task)
				select {
				case output := <-c.C:
					if output.err != nil {
						return throwErrorJS(ctx, output.err)
					}
					esm = output.meta
				case <-time.After(time.Minute):
					buildQueue.RemoveConsumer(task, c)
					return rex.Status(http.StatusRequestTimeout, "timeout, we are building the package hardly, please try again later!")
				}
			}
		}

		if esm.TypesOnly {
			if esm.Dts != "" && !noCheck {
				value := fmt.Sprintf(
					"%s%s/%s",
					typesCdnOrigin,
					cdnBasePath,
					strings.TrimPrefix(esm.Dts, "/"),
				)
				ctx.SetHeader("X-TypeScript-Types", value)
			}
			ctx.SetHeader("Cache-Control", "private, no-store, no-cache, must-revalidate")
			ctx.SetHeader("Content-Type", "application/javascript; charset=utf-8")
			return []byte("export default null;\n")
		}

		if isPkgCss {
			if !esm.PackageCSS {
				return rex.Status(404, "Package CSS not found")
			}

			if !regFullVersionPath.MatchString(pathname) || !isPined {
				url := fmt.Sprintf("%s/%s.css", redirectOrWorkerOrigin, strings.TrimSuffix(taskID, ".js"))
				return rex.Redirect(url, http.StatusTemporaryRedirect)
			}

			taskID = fmt.Sprintf("%s.css", strings.TrimSuffix(taskID, ".js"))
			isBare = true
		}

		if isBare {
			savePath := path.Join(
				"builds",
				taskID,
			)
			exists, size, modtime, err := fs.Exists(savePath)
			if err != nil {
				return rex.Status(500, err.Error())
			}
			if !exists {
				return rex.Status(404, "File not found")
			}
			r, err := fs.ReadFile(savePath, size)
			if err != nil {
				return rex.Status(500, err.Error())
			}
			if !hasBuildVerPrefix && esm.Dts != "" && !noCheck && !isWorker {
				value := fmt.Sprintf(
					"%s%s/%s",
					typesCdnOrigin,
					cdnBasePath,
					strings.TrimPrefix(esm.Dts, "/"),
				)
				ctx.SetHeader("X-TypeScript-Types", value)
			}
			ctx.SetHeader("Cache-Control", "public, max-age=31536000, immutable")
			return rex.Content(savePath, modtime, r)
		}

		buf := bytes.NewBuffer(nil)

		fmt.Fprintf(buf, `/* esm.sh - %v */%s`, reqPkg, "\n")
		if isWorker {
			fmt.Fprintf(buf, `export default function workerFactory() {%s  return new Worker('%s/%s', { type: 'module' })%s}`, "\n", redirectOrWorkerOrigin, taskID, "\n")
		} else {
			fmt.Fprintf(buf, `export * from "%s%s/%s";%s`, buildsCdnOrigin, cdnBasePath, taskID, "\n")
			if esm.CJS || esm.ExportDefault {
				fmt.Fprintf(
					buf,
					`export { default } from "%s%s/%s";%s`,
					buildsCdnOrigin,
					cdnBasePath,
					taskID,
					"\n",
				)
			}
		}

		if esm.Dts != "" && !noCheck && !isWorker {
			value := fmt.Sprintf(
				"%s%s/%s",
				typesCdnOrigin,
				cdnBasePath,
				strings.TrimPrefix(esm.Dts, "/"),
			)
			ctx.SetHeader("X-TypeScript-Types", value)
		}
		if regFullVersionPath.MatchString(pathname) {
			if isPined {
				ctx.SetHeader("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				ctx.SetHeader("Cache-Control", fmt.Sprintf("public, max-age=%d", 24*3600)) // cache for 24 hours
			}
		} else {
			ctx.SetHeader("Cache-Control", fmt.Sprintf("public, max-age=%d", 10*60)) // cache for 10 minutes
		}
		ctx.SetHeader("Content-Type", "application/javascript; charset=utf-8")
		return buf
	}
}

func throwErrorJS(ctx *rex.Context, err error) interface{} {
	buf := bytes.NewBuffer(nil)
	fmt.Fprintf(buf, "/* esm.sh - error */\n")
	fmt.Fprintf(
		buf,
		`throw new Error("[esm.sh] " + %s);%s`,
		strings.TrimSpace(string(utils.MustEncodeJSON(err.Error()))),
		"\n",
	)
	fmt.Fprintf(buf, "export default null;\n")
	ctx.SetHeader("Cache-Control", "private, no-store, no-cache, must-revalidate")
	ctx.SetHeader("Content-Type", "application/javascript; charset=utf-8")
	return rex.Status(500, buf)
}
