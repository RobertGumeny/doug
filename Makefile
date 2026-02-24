.PHONY: build test lint release-dry

build:
	go build -o doug .

test:
	go test ./...

lint:
	go vet ./...

release-dry:
	goreleaser release --snapshot --clean
