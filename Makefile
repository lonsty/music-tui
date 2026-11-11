SHELL := /bin/sh

# CGO is required by the SQLite driver and the audio backend.
export CGO_ENABLED := 1
# Let a system Go toolchain older than go.mod download the required version.
export GOTOOLCHAIN := auto

.PHONY: build test vet fmt bench all

all: build vet test

build:
	go build ./...

test:
	go test ./...

vet:
	go vet ./...

fmt:
	@out="$$(gofmt -l cmd internal)"; \
	if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

bench:
	go test ./internal/tui -run '^$$' -bench . -benchmem
