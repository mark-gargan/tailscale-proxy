BIN:=tailscale-proxy
PKG:=./cmd/tailscale-proxy

.PHONY: build run test lint fmt

build:
	GOFLAGS= go build -o bin/$(BIN) $(PKG)

run:
	go run $(PKG)

test:
	go test ./... -race -cover

fmt:
	gofmt -s -w .

