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
# 四个平台各出一个包 + sha256。安装脚本按 uname 选对应的那一个。
PLATFORMS := linux_amd64 linux_arm64 darwin_amd64 darwin_arm64
dist:
	@mkdir -p dist
	@rm -f dist/ratchet_$(VERSION)_*.tar.gz dist/ratchet_$(VERSION)_*.sha256
	@for p in $(PLATFORMS); do \
	  os=$${p%_*}; arch=$${p#*_}; \
	  out=dist/ratchet_$(VERSION)_$$p.tar.gz; \
	  echo "  build $$p"; \
	  GOOS=$$os GOARCH=$$arch $(GO) build -trimpath -ldflags "-s -w" -o dist/ratchet ./cmd/ratchet || exit 1; \
	  tar -C dist -czf $$out ratchet; \
	  rm -f dist/ratchet; \
	  (cd dist && sha256sum $$(basename $$out) > $$(basename $$out).sha256); \
	done
	@echo "产物："
	@ls -1 dist/ | sed 's/^/  /'

fmt:
	$(GO) fmt ./...
