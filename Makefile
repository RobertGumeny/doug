.PHONY: build test test-integration lint release-dry

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
TMP_CACHE_ROOT ?= /tmp/doug-cache
GOCACHE_DIR := $(TMP_CACHE_ROOT)/go-build
GOLANGCI_LINT_CACHE_DIR := $(TMP_CACHE_ROOT)/golangci-lint
BUILD_DIR := $(CURDIR)/bin
BUILD_OUTPUT := $(BUILD_DIR)/doug
UNIT_TEST_TIMEOUT ?= 60s
INTEGRATION_TEST_TIMEOUT ?= 120s
GOFMT_CHECK = files="$$(gofmt -l .)"; \
	if [ -n "$$files" ]; then \
		printf '%s\n' "$$files"; \
		exit 1; \
	fi

build:
	@mkdir -p "$(BUILD_DIR)"
	go build -ldflags "-X github.com/robertgumeny/doug/cmd.version=$(VERSION)" -o "$(BUILD_OUTPUT)" .

test:
	go test ./... -count=1 -timeout $(UNIT_TEST_TIMEOUT)

test-integration:
	go test -tags=integration ./integration -count=1 -v -timeout $(INTEGRATION_TEST_TIMEOUT)

lint:
	@mkdir -p "$(GOCACHE_DIR)" "$(GOLANGCI_LINT_CACHE_DIR)"
	@$(GOFMT_CHECK)
	GOCACHE="$(GOCACHE_DIR)" GOLANGCI_LINT_CACHE="$(GOLANGCI_LINT_CACHE_DIR)" golangci-lint run ./...
	GOCACHE="$(GOCACHE_DIR)" go vet ./...

release-dry:
	goreleaser release --snapshot --clean
