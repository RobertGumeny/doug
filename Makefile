.PHONY: build test lint release-dry

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GOFMT_CHECK = files="$$(gofmt -l .)"; \
	if [ -n "$$files" ]; then \
		printf '%s\n' "$$files"; \
		exit 1; \
	fi

build:
	go build -ldflags "-X github.com/robertgumeny/doug/cmd.version=$(VERSION)" -o doug .

test:
	go test ./...

lint:
	@$(GOFMT_CHECK)
	golangci-lint run
	go vet ./...

release-dry:
	goreleaser release --snapshot --clean
