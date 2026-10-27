GO ?= /home/carawin/sdk/go/bin/go
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o bin/imperium ./cmd/imperium
test:
	$(GO) vet ./... && $(GO) test -race ./...
run:
	IMPERIUM_LANG=id $(GO) run ./cmd/imperium -addr 127.0.0.1:8100
.PHONY: build test run
