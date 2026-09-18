.PHONY: test go-test py-test web-test build fmt dist

GO ?= go
PY ?= python3

test: go-test py-test web-test

go-test:
	cd . && $(GO) test ./...

py-test:
	cd service && $(PY) -m pytest -q

web-test:
	cd web && npm run test --silent

build:
	$(GO) build -o bin/ratchet ./cmd/ratchet

# 发布产物：二进制 + sha256。页面上的下载地址指向的就是这两个文件。
# 先把 sha256 放出来，是因为"拿到东西先验哈希"就是这个产品要教给用户的习惯。
VERSION := $(shell sed -n 's/^const version = "\(.*\)"/\1/p' cmd/ratchet/main.go)
dist:
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags "-s -w" -o dist/ratchet ./cmd/ratchet
	tar -C dist -czf dist/ratchet_$(VERSION)_linux_amd64.tar.gz ratchet
	cd dist && sha256sum ratchet_$(VERSION)_linux_amd64.tar.gz > ratchet_$(VERSION)_linux_amd64.tar.gz.sha256
	@echo "产物："
	@ls -1 dist/ | sed 's/^/  /'
	@cat dist/ratchet_$(VERSION)_linux_amd64.tar.gz.sha256

fmt:
	$(GO) fmt ./...
