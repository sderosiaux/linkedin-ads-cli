.PHONY: build test lint check fmt vet tidy clean install-hooks

build:
	go build -o linkedin-ads ./cmd/linkedin-ads

test:
	go test ./... -count=1 -short

lint:
	golangci-lint run

fmt:
	gofumpt -l -w .

vet:
	go vet ./...

tidy:
	go mod tidy
	@git diff --exit-code go.mod go.sum || (echo "go.mod/go.sum not tidy" && exit 1)

clean:
	rm -f linkedin-ads
	rm -rf dist/

check: tidy vet lint test

install-hooks:
	git config core.hooksPath .githooks
	@echo "Git hooks installed from .githooks/"
