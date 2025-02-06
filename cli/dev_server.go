package cli

import (
	"bytes"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/esm-dev/esm.sh/server/common"
	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/gorilla/websocket"
	"github.com/ije/esbuild-internal/xxhash"
	"github.com/ije/gox/term"
	"github.com/ije/gox/utils"
	"golang.org/x/net/html"
)

type DevServer struct {
	fs               *embed.FS
	rootDir          string
	watchData        map[*websocket.Conn]map[string]int64
	watchDataMapLock sync.RWMutex
	loaderWorker     *LoaderWorker
	loaderInitLock   sync.Mutex
	loaderCache      sync.Map
}

func (d *DevServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	pathname := r.URL.Path
	switch pathname {
	case "/@hmr", "/@refresh", "/@prefresh", "/@vdr":
		d.ServeInternalJS(w, r, pathname[2:])
	case "/@hmr-ws":
		if d.watchData == nil {
			d.watchData = make(map[*websocket.Conn]map[string]int64)
			go d.watchFS()
		}
		d.ServeHmrWS(w, r)
	default:
		filename := filepath.Join(d.rootDir, pathname)
		fi, err := os.Lstat(filename)
		if err == nil && fi.IsDir() {
			if pathname != "/" && !strings.HasSuffix(pathname, "/") {
				http.Redirect(w, r, pathname+"/", http.StatusMovedPermanently)
				return
			}
			pathname = strings.TrimSuffix(pathname, "/") + "/index.html"
			filename = filepath.Join(d.rootDir, pathname)
			fi, err = os.Lstat(filename)
		}
		if err != nil {
			if os.IsNotExist(err) {
				http.Error(w, "Not Found", 404)
			} else {
				http.Error(w, "Internal Server Error", 500)
			}
			return
		}
		if fi.IsDir() {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		query := r.URL.Query()
		if query.Has("url") {
			header := w.Header()
			header.Set("Content-Type", "application/javascript; charset=utf-8")
			header.Set("Cache-Control", "public, max-age=31536000, immutable")
			w.Write([]byte(`const url = new URL(import.meta.url);url.searchParams.delete("url");export default url.href;`))
			return
		}
		switch extname := filepath.Ext(filename); extname {
		case ".html":
			etag := fmt.Sprintf("w/\"%d-%d-%d\"", fi.ModTime().UnixMilli(), fi.Size(), VERSION)
			if r.Header.Get("If-None-Match") == etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			header := w.Header()
			header.Set("Content-Type", "text/html; charset=utf-8")
			header.Set("Cache-Control", "max-age=0, must-revalidate")
			header.Set("Etag", etag)
			d.ServeHtml(w, r, pathname)
		case ".js", ".mjs", ".jsx", ".ts", ".mts", ".tsx", ".vue", ".svelte":
			if !query.Has("raw") {
				d.ServeModule(w, r, pathname, nil)
				return
			}
			fallthrough
		case ".css":
			if query.Has("module") {
				d.ServeCSSModule(w, r, pathname, query)
				return
			}
			if strings.HasSuffix(pathname, "/uno.css") {
				d.ServeUnoCSS(w, r)
				return
			}
			fallthrough
		case ".md":
			if extname == ".md" && !query.Has("raw") {
				markdown, err := os.ReadFile(filename)
				if err != nil {
					http.Error(w, "Internal Server Error", 500)
					return
				}
				if query.Has("jsx") {
					jsxCode, err := common.RenderMarkdown(markdown, common.MarkdownRenderKindJSX)
					if err != nil {
						http.Error(w, "Failed to render markdown to jsx", 500)
						return
					}
					d.ServeModule(w, r, pathname+"?jsx", jsxCode)
				} else if query.Has("svelte") {
					svelteCode, err := common.RenderMarkdown(markdown, common.MarkdownRenderKindSvelte)
					if err != nil {
						http.Error(w, "Failed to render markdown to svelte component", 500)
						return
					}
					d.ServeModule(w, r, pathname+"?svelte", svelteCode)
				} else if query.Has("vue") {
					vueCode, err := common.RenderMarkdown(markdown, common.MarkdownRenderKindVue)
					if err != nil {
						http.Error(w, "Failed to render markdown to vue component", 500)
						return
					}
					d.ServeModule(w, r, pathname+"?vue", vueCode)
				} else {
					js, err := common.RenderMarkdown(markdown, common.MarkdownRenderKindJS)
					if err != nil {
						http.Error(w, "Failed to render markdown", 500)
						return
					}
					etag := fmt.Sprintf("w/\"%d-%d-%d\"", fi.ModTime().UnixMilli(), fi.Size(), VERSION)
					if r.Header.Get("If-None-Match") == etag && !query.Has("t") {
						w.WriteHeader(http.StatusNotModified)
						return
					}
					if r.Header.Get("If-None-Match") == etag && !query.Has("t") {
						w.WriteHeader(http.StatusNotModified)
						return
					}
					header := w.Header()
					header.Set("Content-Type", "application/javascript; charset=utf-8")
					if !query.Has("t") {
						header.Set("Cache-Control", "max-age=0, must-revalidate")
						header.Set("Etag", etag)
					}
					w.Write(js)
				}
				return
			}
			fallthrough
		default:
			etag := fmt.Sprintf("w/\"%d-%d-%d\"", fi.ModTime().UnixMilli(), fi.Size(), VERSION)
			if r.Header.Get("If-None-Match") == etag && !query.Has("t") {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			file, err := os.Open(filename)
			if err != nil {
				http.Error(w, "Internal Server Error", 500)
				return
			}
			defer file.Close()
			contentType := common.ContentType(filename)
			if contentType == "" {
				contentType = "application/octet-stream"
			}
			header := w.Header()
			header.Set("Content-Type", contentType)
			if !query.Has("t") {
				header.Set("Cache-Control", "max-age=0, must-revalidate")
				header.Set("Etag", etag)
			}
			io.Copy(w, file)
		}
	}
}

func (d *DevServer) ServeHtml(w http.ResponseWriter, r *http.Request, pathname string) {
	htmlFile, err := os.Open(filepath.Join(d.rootDir, pathname))
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Not Found", 404)
		} else {
			http.Error(w, "Internal Server Error", 500)
		}
		return
	}
	defer htmlFile.Close()

	tokenizer := html.NewTokenizer(htmlFile)
	unocss := ""
	cssLinks := []string{}
	overriding := ""

	for {
		tt := tokenizer.Next()
		if tt == html.ErrorToken {
			break
		}
		if overriding != "" {
			if tt == html.TextToken {
				continue
			}
			if tt == html.EndTagToken {
				tagName, _ := tokenizer.TagName()
				if string(tagName) == overriding {
					overriding = ""
					continue
				}
			}
			overriding = ""
		}
		if tt == html.StartTagToken {
			tagName, moreAttr := tokenizer.TagName()
			attrs := map[string]string{}
			for moreAttr {
				var key, val []byte
				key, val, moreAttr = tokenizer.TagAttr()
				attrs[string(key)] = string(val)
			}
			switch string(tagName) {
			case "script":
				srcAttr := attrs["src"]
				hrefAttr := attrs["href"]
				// replace `<script src="https://esm.sh/x" href="..."></script>`
				// with `<script type="module" src="..."></script>`
				if hrefAttr != "" && (strings.HasPrefix(srcAttr, "https://") || strings.HasPrefix(srcAttr, "http://")) {
					if srcUrl, parseErr := url.Parse(srcAttr); parseErr == nil && srcUrl.Path == "/x" {
						hrefAttr, _ = utils.SplitByFirstByte(hrefAttr, '?')
						if hrefAttr == "uno.css" || strings.HasSuffix(hrefAttr, "/uno.css") {
							w.Write([]byte("<link rel=\"stylesheet\" href=\""))
							w.Write([]byte(hrefAttr + "?ctx=" + base64.RawURLEncoding.EncodeToString([]byte(pathname))))
							w.Write([]byte{'"', '>'})
							overriding = "script"
						} else {
							w.Write([]byte("<script type=\"module\""))
							for attrKey, attrVal := range attrs {
								switch attrKey {
								case "href", "type":
									// strip
								case "src":
									hrefAttr, _ = utils.SplitByFirstByte(hrefAttr, '?')
									w.Write([]byte(" src=\""))
									w.Write([]byte(hrefAttr + "?im=" + base64.RawURLEncoding.EncodeToString([]byte(pathname))))
									w.Write([]byte{'"'})
								default:
									w.Write([]byte{' '})
									w.Write([]byte(attrKey))
									if attrVal != "" {
										w.Write([]byte{'=', '"'})
										w.Write([]byte(attrVal))
										w.Write([]byte{'"'})
									}
								}
							}
							w.Write([]byte{'>'})
						}
						continue
					}
				}
			case "link":
				if attrs["rel"] == "stylesheet" {
					if src := attrs["href"]; isAbsPathSpecifier(src) || isRelPathSpecifier(src) {
						cssLinks = append(cssLinks, src)
					}
				}
			}
		}
		w.Write(tokenizer.Raw())
	}
	// reload the page when the html file is modified
	fmt.Fprintf(w, `<script type="module">import createHotContext from"/@hmr";const hot=createHotContext("%s");hot.watch(()=>location.reload());`, pathname)
	// reload the page when the css file is modified
	if len(cssLinks) > 0 {
		for _, cssLink := range cssLinks {
			u := &url.URL{Path: pathname}
			u = u.ResolveReference(&url.URL{Path: cssLink})
			fmt.Fprintf(w, `const linkEl=document.querySelector('link[rel="stylesheet"][href="%s"]');hot.watch("%s",(kind,filename)=>{if(kind==="modify")linkEl.href=filename+"?t="+Date.now().toString(36)});`, cssLink, u.Path)
		}
	}
	// reload the unocss when the module dependency tree is changed
	if unocss != "" {
		fmt.Fprintf(w, `hot.watch("*",(kind,filename)=>{if(/\.(js|mjs|jsx|ts|mts|tsx|vue|svelte)$/i.test(filename)){document.getElementById("@unocss").href="%s&t="+Date.now().toString(36)}});`, unocss)
		u := &url.URL{Path: pathname}
		u = u.ResolveReference(&url.URL{Path: "uno.css"})
		filename := filepath.Join(d.rootDir, u.Path)
		if _, err := os.Stat(filename); err != nil && os.IsNotExist(err) {
			u = &url.URL{Path: "/uno.css"}
			filename = filepath.Join(d.rootDir, u.Path)
		}
		if _, err := os.Stat(filename); err == nil || os.IsExist(err) {
			// reload the page when the uno.css file is modified
			w.Write([]byte("hot.watch(\""))
			w.Write([]byte(u.Path))
			w.Write([]byte("\",()=>location.reload());"))
		}
	}
	w.Write([]byte("</script>"))
	w.Write([]byte(`<script>console.log("%c💚 Built with esm.sh/x, please uncheck \"Disable cache\" in Network tab for better DX!", "color:green")</script>`))
}

func (d *DevServer) ServeModule(w http.ResponseWriter, r *http.Request, pathname string, sourceCode []byte) {
	query := r.URL.Query()
	im, err := base64.RawURLEncoding.DecodeString(query.Get("im"))
	if err != nil {
		http.Error(w, "Bad Request", 400)
		return
	}
	imHtmlFilename := filepath.Join(d.rootDir, string(im))
	imHtmlFile, err := os.Open(imHtmlFilename)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Bad Request", 400)
		} else {
			http.Error(w, "Internal Server Error", 500)
		}
		return
	}
	defer imHtmlFile.Close()
	var importMap common.ImportMap
	tokenizer := html.NewTokenizer(imHtmlFile)
	for {
		tt := tokenizer.Next()
		if tt == html.ErrorToken {
			break
		}
		if tt == html.StartTagToken {
			tagName, moreAttr := tokenizer.TagName()
			if string(tagName) == "script" {
				var typeAttr string
				for moreAttr {
					var key, val []byte
					key, val, moreAttr = tokenizer.TagAttr()
					if string(key) == "type" {
						typeAttr = string(val)
						break
					}
				}
				if typeAttr == "importmap" {
					tokenizer.Next()
					if json.Unmarshal(tokenizer.Text(), &importMap) != nil {
						header := w.Header()
						header.Set("Content-Type", "application/javascript; charset=utf-8")
						header.Set("Cache-Control", "max-age=0, must-revalidate")
						w.Write([]byte(`throw new Error("Failed to parse import map: invalid JSON")`))
						return
					}
					importMap.Src = "file://" + string(im)
					break
				}
			} else if string(tagName) == "body" {
				break
			}
		} else if tt == html.EndTagToken {
			tagName, _ := tokenizer.TagName()
			if bytes.Equal(tagName, []byte("head")) {
				break
			}
		}
	}
	var modTime uint64
	var size int64
	if sourceCode != nil {
		sha := xxhash.New()
		sha.Write(sourceCode)
		modTime = sha.Sum64()
		size = int64(len(sourceCode))
	} else {
		fi, err := os.Lstat(filepath.Join(d.rootDir, pathname))
		if err != nil {
			if os.IsNotExist(err) {
				http.Error(w, "Not Found", 404)
			} else {
				http.Error(w, "Internal Server Error", 500)
			}
			return
		}
		modTime = uint64(fi.ModTime().UnixMilli())
		size = fi.Size()
	}
	sha := xxhash.New()
	if json.NewEncoder(sha).Encode(importMap) != nil {
		http.Error(w, "Internal Server Error", 500)
		return
	}
	etag := fmt.Sprintf("w/\"%d-%d-%d-%x\"", modTime, size, VERSION, sha.Sum(nil))
	if r.Header.Get("If-None-Match") == etag && !query.Has("t") {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	cacheKey := fmt.Sprintf("module-%s", pathname)
	etagCacheKey := fmt.Sprintf("module-%s.etag", pathname)
	if js, ok := d.loaderCache.Load(cacheKey); ok {
		if e, ok := d.loaderCache.Load(etagCacheKey); ok {
			if e.(string) == etag {
				header := w.Header()
				header.Set("Content-Type", "application/javascript; charset=utf-8")
				if !query.Has("t") {
					header.Set("Cache-Control", "max-age=0, must-revalidate")
					header.Set("Etag", etag)
				}
				w.Write(js.([]byte))
				return
			}
		}
	}
	loader, err := d.getLoader()
	if err != nil {
		fmt.Println(term.Red("[error] failed to start loader process: " + err.Error()))
		http.Error(w, "Internal Server Error", 500)
		return
	}
	args := []any{pathname, importMap}
	if sourceCode != nil {
		args = append(args, string(sourceCode))
	}
	_, js, err := loader.Load("module", args)
	if err != nil {
		fmt.Println(term.Red("[error] " + err.Error()))
		http.Error(w, "Internal Server Error", 500)
		return
	}
	d.loaderCache.Store(cacheKey, []byte(js))
	d.loaderCache.Store(etagCacheKey, etag)
	header := w.Header()
	header.Set("Content-Type", "application/javascript; charset=utf-8")
	if !query.Has("t") {
		header.Set("Cache-Control", "max-age=0, must-revalidate")
		header.Set("Etag", etag)
	}
	w.Write([]byte(js))
}

func (d *DevServer) ServeCSSModule(w http.ResponseWriter, r *http.Request, pathname string, query url.Values) {
	ret := esbuild.Build(esbuild.BuildOptions{
		EntryPoints:      []string{filepath.Join(d.rootDir, pathname)},
		Write:            false,
		MinifyWhitespace: true,
		MinifySyntax:     true,
		Target:           esbuild.ES2022,
		Bundle:           true,
	})
	if len(ret.Errors) > 0 {
		fmt.Println(term.Red(ret.Errors[0].Text))
		http.Error(w, "Internal Server Error", 500)
		return
	}
	css := bytes.TrimSpace(ret.OutputFiles[0].Contents)
	sha := xxhash.New()
	sha.Write(css)
	etag := fmt.Sprintf("w/\"%x-%d\"", sha.Sum(nil), VERSION)
	if r.Header.Get("If-None-Match") == etag && !query.Has("t") {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	header := w.Header()
	header.Set("Content-Type", "application/javascript; charset=utf-8")
	if !query.Has("t") {
		header.Set("Cache-Control", "max-age=0, must-revalidate")
		header.Set("Etag", etag)
	}
	fmt.Fprintf(w, `const CSS=%s;`, string(utils.MustEncodeJSON(string(css))))
	w.Write([]byte(`let styleEl;`))
	w.Write([]byte(`function applyCSS(css){if(styleEl)styleEl.textContent=css;else{styleEl=document.createElement("style");styleEl.textContent=css;document.head.appendChild(styleEl);}}`))
	w.Write([]byte(`!(new URL(import.meta.url)).searchParams.has("t")&&applyCSS(CSS);`))
	w.Write([]byte(`import createHotContext from"/@hmr";`))
	fmt.Fprintf(w, `createHotContext("%s").accept(m=>applyCSS(m.default));`, pathname)
	w.Write([]byte(`export default CSS;`))
}

func (d *DevServer) ServeUnoCSS(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	ctx, err := base64.RawURLEncoding.DecodeString(query.Get("ctx"))
	if err != nil {
		http.Error(w, "Bad Request", 400)
		return
	}
	imHtmlFilename := filepath.Join(d.rootDir, string(ctx))
	imHtmlFile, err := os.Open(imHtmlFilename)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Bad Request", 400)
		} else {
			http.Error(w, "Internal Server Error", 500)
		}
		return
	}
	defer imHtmlFile.Close()

	contents := [][]byte{}
	jsEntries := map[string]struct{}{}
	importMap := common.ImportMap{}
	tokenizer := html.NewTokenizer(imHtmlFile)
	for {
		tt := tokenizer.Next()
		if tt == html.ErrorToken {
			break
		}
		if tt == html.StartTagToken {
			name, moreAttr := tokenizer.TagName()
			switch string(name) {
			case "script":
				var (
					typeAttr string
					srcAttr  string
					hrefAttr string
				)
				for moreAttr {
					var key, val []byte
					key, val, moreAttr = tokenizer.TagAttr()
					if bytes.Equal(key, []byte("src")) {
						srcAttr = string(val)
					} else if bytes.Equal(key, []byte("href")) {
						hrefAttr = string(val)
					} else if bytes.Equal(key, []byte("type")) {
						typeAttr = string(val)
					}
				}
				if typeAttr == "importmap" {
					tokenizer.Next()
					innerText := bytes.TrimSpace(tokenizer.Text())
					if len(innerText) > 0 {
						err := json.Unmarshal(innerText, &importMap)
						if err == nil {
							importMap.Src = string(ctx)
						}
					}
				} else if srcAttr == "" {
					// inline script content
					tokenizer.Next()
					contents = append(contents, tokenizer.Text())
				} else {
					if hrefAttr != "" && isHttpSepcifier(srcAttr) {
						if !isHttpSepcifier(hrefAttr) && endsWith(hrefAttr, moduleExts...) {
							jsEntries[hrefAttr] = struct{}{}
						}
					} else if !isHttpSepcifier(srcAttr) && endsWith(srcAttr, moduleExts...) {
						jsEntries[srcAttr] = struct{}{}
					}
				}
			case "link", "meta", "title", "base", "head", "noscript", "slot", "template", "option":
				// ignore
			default:
				contents = append(contents, tokenizer.Raw())
			}
		}
	}

	configCSS, err := os.ReadFile(filepath.Join(d.rootDir, r.URL.Path))
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Not Found", 404)
		}
		http.Error(w, "Internal Server Error", 500)
		return
	}

	tree := map[string][]byte{}
	for entry := range jsEntries {
		t, err := d.analyzeDependencyTree(filepath.Join(filepath.Dir(imHtmlFilename), entry), importMap)
		if err == nil {
			for filename, code := range t {
				tree[filename] = code
			}
		}
	}
	xh := xxhash.New()
	xh.Write([]byte(configCSS))
	keys := make([]string, 0, len(tree))
	for k := range tree {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		code := tree[k]
		contents = append(contents, code)
		xh.Write(code)
	}
	etag := fmt.Sprintf("w\"%x\"", xh.Sum(nil))
	if r.Header.Get("If-None-Match") == etag && !query.Has("t") {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	cacheKey := fmt.Sprintf("unocss-%s", ctx)
	etagCacheKey := fmt.Sprintf("unocss-%s.etag", ctx)
	if css, ok := d.loaderCache.Load(cacheKey); ok {
		if e, ok := d.loaderCache.Load(etagCacheKey); ok {
			if e.(string) == etag {
				header := w.Header()
				header.Set("Content-Type", "text/css; charset=utf-8")
				if !query.Has("t") {
					header.Set("Cache-Control", "max-age=0, must-revalidate")
					header.Set("Etag", etag)
				}
				w.Write(css.([]byte))
				return
			}
		}
	}
	loader, err := d.getLoader()
	if err != nil {
		fmt.Println(term.Red("[error] failed to start loader process: " + err.Error()))
		http.Error(w, "Internal Server Error", 500)
		return
	}
	config := map[string]any{"filename": r.URL.Path, "css": string(configCSS)}
	_, css, err := loader.Load("unocss", []any{config, string(bytes.Join(contents, []byte{'\n'}))})
	if err != nil {
		fmt.Println(term.Red("[error] " + err.Error()))
		http.Error(w, "Internal Server Error", 500)
		return
	}
	d.loaderCache.Store(cacheKey, []byte(css))
	d.loaderCache.Store(etagCacheKey, etag)
	header := w.Header()
	header.Set("Content-Type", "text/css; charset=utf-8")
	if !query.Has("t") {
		header.Set("Cache-Control", "max-age=0, must-revalidate")
		header.Set("Etag", etag)
	}
	w.Write([]byte(css))
}

func (d *DevServer) ServeInternalJS(w http.ResponseWriter, r *http.Request, name string) {
	data, err := d.fs.ReadFile("cli/internal/" + name + ".js")
	if err != nil {
		http.Error(w, "Internal Server Error", 500)
		return
	}
	etag := fmt.Sprintf("w/\"%d\"", VERSION)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	header := w.Header()
	header.Set("Content-Type", "application/javascript; charset=utf-8")
	header.Set("Cache-Control", "max-age=0, must-revalidate")
	header.Set("Etag", etag)
	w.Write(data)
}

func (d *DevServer) ServeHmrWS(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Upgrade") != "websocket" {
		http.Error(w, "Bad Request", 400)
		return
	}
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Internal Server Error", 500)
		return
	}
	defer conn.Close()
	watchList := make(map[string]int64)
	d.watchDataMapLock.Lock()
	d.watchData[conn] = watchList
	d.watchDataMapLock.Unlock()
	defer func() {
		d.watchDataMapLock.Lock()
		delete(d.watchData, conn)
		d.watchDataMapLock.Unlock()
	}()
	for {
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if messageType == websocket.TextMessage {
			msg := string(data)
			if strings.HasPrefix(msg, "watch:") {
				pathname := msg[6:]
				if pathname != "" {
					filename, _ := utils.SplitByFirstByte(pathname, '?')
					fi, err := os.Lstat(filepath.Join(d.rootDir, filename))
					if err != nil {
						if os.IsNotExist(err) {
							// file not found, watch if it's created
							watchList[pathname] = 0
						} else {
							conn.WriteMessage(websocket.TextMessage, []byte("error:could not watch "+pathname))
						}
					} else {
						watchList[pathname] = fi.ModTime().UnixMilli()
					}
				}
			}
		}
	}
}

func (d *DevServer) analyzeDependencyTree(entry string, importMap common.ImportMap) (tree map[string][]byte, err error) {
	tree = make(map[string][]byte)
	ret := esbuild.Build(esbuild.BuildOptions{
		EntryPoints:      []string{entry},
		Target:           esbuild.ES2022,
		Format:           esbuild.FormatESModule,
		Platform:         esbuild.PlatformBrowser,
		JSX:              esbuild.JSXPreserve,
		MinifyWhitespace: true,
		Bundle:           true,
		Write:            false,
		Outdir:           "/esbuild",
		Plugins: []esbuild.Plugin{
			{
				Name: "loader",
				Setup: func(build esbuild.PluginBuild) {
					build.OnResolve(esbuild.OnResolveOptions{Filter: ".*"}, func(args esbuild.OnResolveArgs) (esbuild.OnResolveResult, error) {
						path, _ := importMap.Resolve(args.Path)
						if isHttpSepcifier(path) || (!isRelPathSpecifier(path) && !isAbsPathSpecifier(path)) {
							return esbuild.OnResolveResult{Path: path, External: true}, nil
						}
						if endsWith(path, moduleExts...) {
							if isRelPathSpecifier(path) {
								path = filepath.Join(filepath.Dir(args.Importer), path)
							}
							return esbuild.OnResolveResult{Path: path, Namespace: "module", PluginData: path}, nil
						}
						return esbuild.OnResolveResult{}, nil
					})
					build.OnLoad(esbuild.OnLoadOptions{Filter: ".*", Namespace: "module"}, func(args esbuild.OnLoadArgs) (esbuild.OnLoadResult, error) {
						data, err := os.ReadFile(args.Path)
						if err != nil {
							return esbuild.OnLoadResult{}, err
						}
						tree[args.Path] = data
						contents := string(data)
						loader := esbuild.LoaderJS
						pathname := args.PluginData.(string)
						ext := filepath.Ext(pathname)
						switch ext {
						case ".jsx":
							loader = esbuild.LoaderJSX
						case ".ts", ".mts":
							loader = esbuild.LoaderTS
						case ".tsx":
							loader = esbuild.LoaderTSX
						case ".vue", ".svelte":
							sha := xxhash.New()
							sha.Write(data)
							etag := hex.EncodeToString(sha.Sum(nil))
							cacheKey := fmt.Sprintf("preload-%s", pathname)
							etagCacheKey := fmt.Sprintf("preload-%s.etag", pathname)
							langCacheKey := fmt.Sprintf("preload-%s.lang", pathname)
							if js, ok := d.loaderCache.Load(cacheKey); ok {
								if e, ok := d.loaderCache.Load(etagCacheKey); ok {
									if e.(string) == etag {
										contents = string(js.([]byte))
										if lang, ok := d.loaderCache.Load(langCacheKey); ok {
											if lang.(string) == "ts" {
												loader = esbuild.LoaderTS
											}
										}
										break
									}
								}
							}
							preloader, err := d.getLoader()
							if err != nil {
								return esbuild.OnLoadResult{}, err
							}
							lang, code, err := preloader.Load(ext[1:], []any{pathname, contents, importMap})
							if err != nil {
								return esbuild.OnLoadResult{}, err
							}
							d.loaderCache.Store(cacheKey, []byte(code))
							d.loaderCache.Store(etagCacheKey, etag)
							d.loaderCache.Store(langCacheKey, lang)
							contents = code
							if lang == "ts" {
								loader = esbuild.LoaderTS
							}
						}
						return esbuild.OnLoadResult{Contents: &contents, Loader: loader}, nil
					})
				},
			},
		},
	})
	if len(ret.Errors) > 0 {
		tree = nil
		err = errors.New(ret.Errors[0].Text)
	}
	return
}

func (d *DevServer) watchFS() {
	for {
		time.Sleep(100 * time.Millisecond)
		d.watchDataMapLock.RLock()
		for conn, watchList := range d.watchData {
			for pathname, mtime := range watchList {
				filename, _ := utils.SplitByFirstByte(pathname, '?')
				fi, err := os.Lstat(filepath.Join(d.rootDir, filename))
				if err != nil {
					if os.IsNotExist(err) {
						if watchList[pathname] > 0 {
							watchList[pathname] = 0
							d.purgeLoaderCache(pathname)
							conn.WriteMessage(websocket.TextMessage, []byte("remove:"+pathname))
						}
					} else {
						fmt.Println(term.Red("watch: " + err.Error()))
					}
				} else if modtime := fi.ModTime().UnixMilli(); modtime > mtime {
					kind := "modify"
					if mtime == 0 {
						kind = "create"
					}
					watchList[pathname] = modtime
					conn.WriteMessage(websocket.TextMessage, []byte(kind+":"+pathname))
				}
			}
		}
		d.watchDataMapLock.RUnlock()
	}
}

func (d *DevServer) purgeLoaderCache(filename string) {
	d.loaderCache.Delete(fmt.Sprintf("module-%s", filename))
	d.loaderCache.Delete(fmt.Sprintf("module-%s.etag", filename))
	if strings.HasSuffix(filename, ".vue") || strings.HasSuffix(filename, ".svelte") {
		d.loaderCache.Delete(fmt.Sprintf("preload-%s", filename))
		d.loaderCache.Delete(fmt.Sprintf("preload-%s.etag", filename))
		d.loaderCache.Delete(fmt.Sprintf("preload-%s.lang", filename))
	}
}

func (d *DevServer) getLoader() (loader *LoaderWorker, err error) {
	d.loaderInitLock.Lock()
	defer d.loaderInitLock.Unlock()
	if d.loaderWorker != nil {
		return d.loaderWorker, nil
	}
	loaderJs, err := d.fs.ReadFile("cli/internal/loader.js")
	if err != nil {
		return
	}
	loader = &LoaderWorker{}
	err = loader.Start(d.rootDir, loaderJs)
	if err != nil {
		return nil, err
	}
	d.loaderWorker = loader
	return
}
