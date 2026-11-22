BINARY := httpserver
GO ?= go

.PHONY: all build test lint vulncheck fmt-check check clean

all: build

build:
	$(GO) build -trimpath -o $(BINARY) .

test:
	$(GO) test -race -cover ./...

lint:
	golangci-lint run ./...

vulncheck:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@latest ./...

fmt-check:
	@unformatted=$$(gofmt -s -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "Unformatted Go files:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

check: fmt-check
	$(GO) vet ./...
	golangci-lint run ./...
	$(GO) test -race ./...

clean:
	rm -rf $(BINARY) dist/ coverage.out
