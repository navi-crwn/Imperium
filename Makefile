# go from PATH, else the SDK in ~/sdk/go (where the hel.ink server keeps it). Override with make GO=...
GO ?= $(firstword $(shell command -v go 2>/dev/null) $(wildcard $(HOME)/sdk/go/bin/go) go)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o bin/imperium ./cmd/imperium
# Refresh the HOP family fonts and floating tiles from ../HEL.ink (or: scripts/sync-hop-assets.sh --remote).
assets:
	scripts/sync-hop-assets.sh
test:
	$(GO) vet ./... && $(GO) test -race ./...
run:
	IMPERIUM_LANG=id $(GO) run ./cmd/imperium -addr 127.0.0.1:8100
.PHONY: build test run assets
