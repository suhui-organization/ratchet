.PHONY: test go-test py-test web-test build fmt

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

fmt:
	$(GO) fmt ./...
