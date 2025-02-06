dev/cli/serve:
	@go run -tags debug main.go serve cli/demo/${app}

dev/cli/init:
	@go run -tags debug main.go init

dev/cli/i:
	@go run -tags debug main.go i ${package}

dev/cli/im/add:
	@go run -tags debug main.go im add ${package}

dev/cli/im/update:
	@go run -tags debug main.go im update ${package}

dev/server: config.json
	@rm -rf .esmd/log
	@rm -rf .esmd/storage
	@rm -rf .esmd/esm.db
	@go run -tags debug server/cmd/main.go --config=config.json

test/server:
	@./test/bootstrap.ts ${dir}
