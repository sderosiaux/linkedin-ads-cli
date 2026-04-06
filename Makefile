.PHONY: build test lint check

build:
	go build -o linkedin-ads ./cmd/linkedin-ads

test:
	go test ./... -count=1 -short

lint:
	golangci-lint run

check: test lint
