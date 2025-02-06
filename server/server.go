package server

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/esm-dev/esm.sh/server/npm_replacements"
	"github.com/esm-dev/esm.sh/server/storage"
	"github.com/ije/gox/log"
	"github.com/ije/gox/set"
	"github.com/ije/rex"
)

// Serve serves the esm.sh server
func Serve() {
	var cfile string
	var err error

	flag.StringVar(&cfile, "config", "config.json", "the config file path")
	flag.Parse()

	if existsFile(cfile) {
		config, err = LoadConfig(cfile)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		if DEBUG {
			fmt.Printf("%s [info] Config loaded from %s\n", time.Now().Format("2006-01-02 15:04:05"), cfile)
		}
	} else {
		config = DefaultConfig()
	}

	if DEBUG {
		config.LogLevel = "debug"
	} else {
		// disable log color in release build
		os.Setenv("NO_COLOR", "1")
	}

	logger, err := log.New(fmt.Sprintf("file:%s?buffer=32k&fileDateFormat=20060102", path.Join(config.LogDir, "server.log")))
	if err != nil {
		fmt.Println("failed to initialize logger:", err)
		os.Exit(1)
	}
	logger.SetLevelByName(config.LogLevel)

	accessLogger, err := log.New(fmt.Sprintf("file:%s?buffer=32k&fileDateFormat=20060102", path.Join(config.LogDir, "access.log")))
	if err != nil {
		logger.Fatalf("failed to initialize access logger: %v", err)
	}
	// don't write log message to stdout
	accessLogger.SetQuite(true)

	db, err := OpenDB(path.Join(config.WorkDir, "esm.db"))
	if err != nil {
		logger.Fatalf("init db: %v", err)
	}

	buildStorage, err := storage.New(&config.Storage)
	if err != nil {
		logger.Fatalf("failed to initialize build storage(%s): %v", config.Storage.Type, err)
	}
	logger.Debugf("storage initialized, type: %s, endpoint: %s", config.Storage.Type, config.Storage.Endpoint)

	err = loadUnenvNodeRuntime()
	if err != nil {
		logger.Fatalf("load unenv node runtime: %v", err)
	}
	totalSize := 0
	for _, data := range unenvNodeRuntimeBulid {
		totalSize += len(data)
	}
	logger.Debugf("unenv node runtime loaded, %d files, total size: %d KB", len(unenvNodeRuntimeBulid), totalSize/1024)

	n, err := npm_replacements.Build()
	if err != nil {
		logger.Fatalf("build npm replacements: %v", err)
	}
	logger.Debugf("%d npm repalcements loaded", n)

	// add .esmd/bin to PATH
	os.Setenv("PATH", fmt.Sprintf("%s%c%s", path.Join(config.WorkDir, "bin"), os.PathListSeparator, os.Getenv("PATH")))

	// install deno
	denoVersion, err := installDeno("2.1.7")
	if err != nil {
		logger.Fatalf("failed to install deno: %v", err)
	}
	logger.Debugf("deno v%s installed", denoVersion)

	// install cjs module lexer
	err = installCommonJSModuleLexer()
	if err != nil {
		logger.Fatalf("failed to install cjs-module-lexer: %v", err)
	}
	logger.Debugf("cjs-module-lexer@%s installed", cjsModuleLexerVersion)

	// pre-comile uno generator in background
	go generateUnoCSS(&NpmRC{NpmRegistry: NpmRegistry{Registry: "https://registry.npmjs.org/"}}, "", "")

	// setup rex server
	rex.Use(
		rex.Header("Server", "esm.sh"),
		cors(config.CorsAllowOrigins),
		rex.Logger(logger),
		rex.Optional(rex.AccessLogger(accessLogger), config.AccessLog),
		rex.Optional(rex.Compress(), config.Compress),
		rex.Optional(customLandingPage(&config.CustomLandingPage), config.CustomLandingPage.Origin != ""),
		rex.Optional(esmLegacyRouter(buildStorage), config.LegacyServer != ""),
		esmRouter(db, buildStorage, logger),
	)

	// start server
	C := rex.Serve(rex.ServerConfig{
		Port: uint16(config.Port),
		TLS: rex.TLSConfig{
			Port: uint16(config.TlsPort),
			AutoTLS: rex.AutoTLSConfig{
				AcceptTOS: config.TlsPort > 0 && !DEBUG,
				CacheDir:  path.Join(config.WorkDir, "autotls"),
			},
		},
	})
	logger.Infof("Server is ready on http://localhost:%d", config.Port)

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGHUP, syscall.SIGABRT)
	select {
	case <-c:
	case err = <-C:
		logger.Error(err)
	}

	// release resources
	db.Close()
	logger.FlushBuffer()
	accessLogger.FlushBuffer()
}

func cors(allowOrigins []string) rex.Handle {
	allowList := set.NewReadOnly[string](allowOrigins...)
	return func(ctx *rex.Context) any {
		origin := ctx.R.Header.Get("Origin")
		isOptionsMethod := ctx.R.Method == "OPTIONS"
		h := ctx.W.Header()
		if allowList.Len() > 0 {
			if origin != "" {
				if !allowList.Has(origin) {
					return rex.Status(403, "forbidden")
				}
				setCorsHeaders(h, isOptionsMethod, origin)
			} else if isOptionsMethod {
				// not a preflight request
				return rex.Status(405, "method not allowed")
			}
			appendVaryHeader(h, "Origin")
		} else {
			setCorsHeaders(h, isOptionsMethod, "*")
		}
		if isOptionsMethod {
			return rex.NoContent()
		}
		return ctx.Next()
	}
}

func setCorsHeaders(h http.Header, isOptionsMethod bool, origin string) {
	h.Set("Access-Control-Allow-Origin", origin)
	if isOptionsMethod {
		h.Set("Access-Control-Allow-Headers", "*")
		h.Set("Access-Control-Max-Age", "86400")
	}
}

func customLandingPage(options *LandingPageOptions) rex.Handle {
	assets := set.New[string]()
	for _, p := range options.Assets {
		assets.Add("/" + strings.TrimPrefix(p, "/"))
	}
	return func(ctx *rex.Context) any {
		if ctx.R.URL.Path == "/" || assets.Has(ctx.R.URL.Path) {
			query := ctx.R.URL.RawQuery
			if query != "" {
				query = "?" + query
			}
			url, err := ctx.R.URL.Parse(options.Origin + ctx.R.URL.Path + query)
			if err != nil {
				return rex.Err(http.StatusBadRequest, "Invalid url")
			}
			fetchClient, recycle := NewFetchClient(15, ctx.UserAgent(), false)
			defer recycle()
			res, err := fetchClient.Fetch(url, nil)
			if err != nil {
				return rex.Err(http.StatusBadGateway, "Failed to fetch custom landing page")
			}
			etag := res.Header.Get("Etag")
			if etag != "" {
				if ctx.R.Header.Get("If-None-Match") == etag {
					return rex.Status(http.StatusNotModified, nil)
				}
				ctx.SetHeader("Etag", etag)
			} else {
				lastModified := res.Header.Get("Last-Modified")
				if lastModified != "" {
					v := ctx.R.Header.Get("If-Modified-Since")
					if v != "" {
						timeIfModifiedSince, e1 := time.Parse(http.TimeFormat, v)
						timeLastModified, e2 := time.Parse(http.TimeFormat, lastModified)
						if e1 == nil && e2 == nil && !timeIfModifiedSince.After(timeLastModified) {
							return rex.Status(http.StatusNotModified, nil)
						}
					}
					ctx.SetHeader("Last-Modified", lastModified)
				}
			}
			cacheControl := res.Header.Get("Cache-Control")
			if cacheControl != "" {
				ctx.SetHeader("Cache-Control", cacheControl)
			} else {
				ctx.SetHeader("Cache-Control", ccMustRevalidate)
			}
			ctx.SetHeader("Content-Type", res.Header.Get("Content-Type"))
			return res.Body // auto closed
		}
		return ctx.Next()
	}
}
