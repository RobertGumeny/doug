.PHONY: build test lint release-dry

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

build:
	go build -ldflags "-X github.com/robertgumeny/doug/cmd.version=$(VERSION)" -o doug .

test:
	go test ./...

lint:
	go vet ./...

release-dry:
	goreleaser release --snapshot --clean
