.PHONY: build test lint release-dry

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
TMP_CACHE_ROOT ?= /tmp/doug-cache
GOCACHE_DIR := $(TMP_CACHE_ROOT)/go-build
GOLANGCI_LINT_CACHE_DIR := $(TMP_CACHE_ROOT)/golangci-lint
BUILD_DIR := $(CURDIR)/bin
BUILD_OUTPUT := $(BUILD_DIR)/doug
GOFMT_CHECK = files="$$(gofmt -l .)"; \
	if [ -n "$$files" ]; then \
		printf '%s\n' "$$files"; \
		exit 1; \
	fi

build:
	@mkdir -p "$(BUILD_DIR)"
	go build -ldflags "-X github.com/robertgumeny/doug/cmd.version=$(VERSION)" -o "$(BUILD_OUTPUT)" .

test:
	go test ./...

lint:
	@mkdir -p "$(GOCACHE_DIR)" "$(GOLANGCI_LINT_CACHE_DIR)"
	@$(GOFMT_CHECK)
	GOCACHE="$(GOCACHE_DIR)" GOLANGCI_LINT_CACHE="$(GOLANGCI_LINT_CACHE_DIR)" golangci-lint run ./...
	GOCACHE="$(GOCACHE_DIR)" go vet ./...

release-dry:
	goreleaser release --snapshot --clean
