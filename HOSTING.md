# Self-Hosting

[esm.sh](https://esm.sh) provides a global fast CDN publicly which is powered by [Cloudflare](https://cloudflare.com).
You can also host esm.sh service by yourself. To do this, please follow the instructions below.

## Clone the Source Code

```bash
git clone https://github.com/esm-dev/esm.sh
cd esm.sh
```

## Configuration

To configure the server, create a `config.json` file then pass it to the server bootstrap command. For example:

```jsonc
// config.json
{
  "port": 8080,
  "npmRegistry": "https://registry.npmjs.org/",
  "npmToken": "******"
}
```

You can find all the server options in [config.example.jsonc](./config.example.jsonc).

## Run the Server Locally

You will need [Go](https://golang.org/dl) 1.22+ to compile and run the server.

```bash
go run server/cmd/main.go --config=config.json
```

Then you can import `React` from <http://localhost:8080/react>.

## Deploy the Server to a Single Machine

You can deploy the server to a single machine with the [deploy.sh](./scripts/deploy.sh) script.

```bash
# first time deploy
./scripts/deploy.sh --init
# update the server
./scripts/deploy.sh
```

Recommended hosting requirements:

- Linux system (with systemd)
- 4x CPU cores or more
- 8GB RAM or more
- 100GB disk space or more

## Deploy with Docker

[![Docker Image](https://img.shields.io/github/v/tag/esm-dev/esm.sh?label=Docker&display_name=tag&sort=semver&style=flat&colorA=232323&colorB=232323&logo=docker&logoColor=eeeeee)](https://github.com/esm-dev/esm.sh/pkgs/container/esm.sh)

esm.sh provides a Docker image for fast deployment. You can pull the container image from <https://ghcr.io/esm-dev/esm.sh>.

```bash
docker pull ghcr.io/esm-dev/esm.sh      # latest version
docker pull ghcr.io/esm-dev/esm.sh:v136 # specific version
```

Run the container:

```bash
docker run -p 8080:8080 \
  -e NPM_REGISTRY=https://registry.npmjs.org/ \
  -e NPM_TOKEN=****** \
  -v MY_VOLUME:/esmd \
  ghcr.io/esm-dev/esm.sh:latest
```

Available environment variables:

- `COMPRESS`: Compress http responses with gzip/brotli, default is `true`.
- `CUSTOM_LANDING_PAGE_ORIGIN`: The custom landing page origin, default is empty.
- `CUSTOM_LANDING_PAGE_ASSETS`: The custom landing page assets separated by comma(,), default is empty.
- `CORS_ALLOW_ORIGINS`: The CORS allow origins separated by comma(,), default is allow all origins.
- `LOG_LEVEL`: The log level, available values are ["debug", "info", "warn", "error"], default is "info".
- `ACCESS_LOG`: Enable access log, default is `false`.
- `MINIFY`: Minify the built JS/CSS files, default is `true`.
- `NPM_QUERY_CACHE_TTL`: The cache TTL for NPM query, default is 10 minutes.
- `NPM_REGISTRY`: The global NPM registry, default is "https://registry.npmjs.org/".
- `NPM_TOKEN`: The access token for the global NPM registry.
- `NPM_USER`: The access user for the global NPM registry.
- `NPM_PASSWORD`: The access password for the global NPM registry.
- `SOURCEMAP`: Generate source map for built JS/CSS files, default is `true`.
- `STORAGE_TYPE`: The storage type, available values are ["fs", "s3"], default is "fs".
- `STORAGE_ENDPOINT`: The storage endpoint, default is "~/.esmd/storage".
- `STORAGE_REGION`: The region for S3 storage.
- `STORAGE_ACCESS_KEY_ID`: The access key for S3 storage.
- `STORAGE_SECRET_ACCESS_KEY`: The secret key for S3 storage.

You can also create your own Dockerfile based on `ghcr.io/esm-dev/esm.sh`:

```dockerfile
FROM ghcr.io/esm-dev/esm.sh:latest
ADD --chown=esm:esm ./config.json /etc/esmd/config.json
CMD ["esmd", "--config", "/etc/esmd/config.json"]
```

## Deploy with CloudFlare CDN

To deploy the server with CloudFlare CDN, you need to create following cache rules in the CloudFlare dashboard (see [link](https://developers.cloudflare.com/cache/how-to/cache-rules/create-dashboard/)), and each rule should be set to **"Eligible for cache"**:

#### 1. Cache `.d.ts` Files

```ruby
(ends_with(http.request.uri.path, ".d.ts")) or
(ends_with(http.request.uri.path, ".d.mts")) or
(ends_with(http.request.uri.path, ".d.cts"))
```

#### 2. Cache Package Assets

```ruby
(http.request.uri.path.extension in {"node" "wasm" "less" "sass" "scss" "stylus" "styl" "json" "jsonc" "csv" "xml" "plist" "tmLanguage" "tmTheme" "yml" "yaml" "txt" "glsl" "frag" "vert" "md" "mdx" "markdown" "html" "htm" "svg" "png" "jpg" "jpeg" "webp" "gif" "ico" "eot" "ttf" "otf" "woff" "woff2" "m4a" "mp3" "m3a" "ogg" "oga" "wav" "weba" "gz" "tgz" "css" "map"})
```

#### 3. Cache `?target=*`

```ruby
(http.request.uri.query contains "target=es2015") or
(http.request.uri.query contains "target=es2016") or
(http.request.uri.query contains "target=es2017") or
(http.request.uri.query contains "target=es2018") or
(http.request.uri.query contains "target=es2019") or
(http.request.uri.query contains "target=es2020") or
(http.request.uri.query contains "target=es2021") or
(http.request.uri.query contains "target=es2022") or
(http.request.uri.query contains "target=es2023")or
(http.request.uri.query contains "target=es2024") or
(http.request.uri.query contains "target=esnext") or
(http.request.uri.query contains "target=denonext") or
(http.request.uri.query contains "target=deno") or
(http.request.uri.query contains "target=node")
```

#### 4. Cache `/(target)/`

```ruby
(http.request.uri.path contains "/es2015/" and http.request.uri.path.extension in {"mjs" "map" "css"}) or
(http.request.uri.path contains "/es2016/" and http.request.uri.path.extension in {"mjs" "map" "css"}) or
(http.request.uri.path contains "/es2017/" and http.request.uri.path.extension in {"mjs" "map" "css"}) or
(http.request.uri.path contains "/es2018/" and http.request.uri.path.extension in {"mjs" "map" "css"}) or
(http.request.uri.path contains "/es2019/" and http.request.uri.path.extension in {"mjs" "map" "css"}) or
(http.request.uri.path contains "/es2020/" and http.request.uri.path.extension in {"mjs" "map" "css"}) or
(http.request.uri.path contains "/es2021/" and http.request.uri.path.extension in {"mjs" "map" "css"}) or
(http.request.uri.path contains "/es2022/" and http.request.uri.path.extension in {"mjs" "map" "css"}) or
(http.request.uri.path contains "/es2023/" and http.request.uri.path.extension in {"mjs" "map" "css"}) or
(http.request.uri.path contains "/es2024/" and http.request.uri.path.extension in {"mjs" "map" "css"}) or
(http.request.uri.path contains "/esnext/" and http.request.uri.path.extension in {"mjs" "map" "css"}) or
(http.request.uri.path contains "/denonext/" and http.request.uri.path.extension in {"mjs" "map" "css"}) or
(http.request.uri.path contains "/deno/" and http.request.uri.path.extension in {"mjs" "map" "css"}) or
(http.request.uri.path contains "/node/" and http.request.uri.path.extension in {"mjs" "map" "css"})
```

#### 5. Bypass Cache for Deno/Bun/Node

```ruby
(not starts_with(http.user_agent, "Deno/") and not starts_with(http.user_agent, "Bun/") and not starts_with(http.user_agent, "Node/") and not starts_with(http.user_agent, "Node.js/") and http.user_agent ne "undici")
```
