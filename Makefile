.PHONY: test go-test py-test web-test build fmt dist release

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
# 产物托管地址。发版时用 `make release RELEASE_URL=https://...` 覆盖。
RELEASE_URL ?= https://example.com/ratchet
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

# 打成"拖上去就能用"的目录：install.sh 与全部产物平铺在 dist/release/。
# 安装脚本按 <base>/<asset> 取文件，所以这里必须是平铺结构，不能有子目录。
release: dist
	@rm -rf dist/release
	@mkdir -p dist/release
	@# 把发布地址烧进 install.sh：别人 `curl | sh` 时脚本里没有 $0 路径可用，
	@# 只能在这里替换默认值，否则内层下载会跑到占位地址去。
	@sed 's|__RATCHET_BASE_URL__|$(RELEASE_URL)|' \
	  install.sh > dist/release/install.sh
	@chmod 0755 dist/release/install.sh
	@grep -q '$(RELEASE_URL)' dist/release/install.sh \
	  || { echo "发布地址没写进 install.sh，检查 sed"; exit 1; }
	@cp dist/ratchet_$(VERSION)_*.tar.gz dist/ratchet_$(VERSION)_*.sha256 dist/release/
	@cd dist/release && cat ratchet_$(VERSION)_*.sha256 > SHA256SUMS
	@printf '%s\n' \
	  "Ratchet $(VERSION) — release bundle" \
	  "" \
	  "上传整个目录到任意可直链的静态位置，然后把该地址填进" \
	  "web/app/site.ts 的 installUrl。" \
	  "" \
	  "用户执行：" \
	  "  curl -fsSL <installUrl>/install.sh | sh" \
	  "" \
	  "install.sh 会按 uname 挑选对应平台，并在安装前校验 sha256" \
	  "（不一致直接中止，不写任何文件）。" \
	  "" \
	  "文件清单见 SHA256SUMS。install.sh 本身不参与校验——它是被信任的入口，" \
	  "要钉住它请另外记下它的哈希。" \
	  > dist/release/README.txt
	@echo "可上传目录：dist/release/"
	@ls -1 dist/release | sed 's/^/  /'
	@echo
	@echo "总体积：$$(du -sh dist/release | cut -f1)"
	@echo "烧进脚本的地址：$(RELEASE_URL)"
	@# 占位地址的包等于一个装不上的安装命令——宁可打包失败，也不要发出去。
	@# （本地验证流程可以 ALLOW_PLACEHOLDER=1 放行）
	@case "$(RELEASE_URL)" in \
	  *example.com*) \
	    if [ "$${ALLOW_PLACEHOLDER:-0}" != "1" ]; then \
	      echo "❌ RELEASE_URL 还是占位地址，打出来的安装命令会 404。"; \
	      echo "   带上真实地址重打：make release RELEASE_URL=https://你的域名/ratchet"; \
	      echo "   （只是本地验证流程时可加 ALLOW_PLACEHOLDER=1）"; \
	      exit 1; \
	    fi ;; \
	esac

fmt:
	$(GO) fmt ./...
