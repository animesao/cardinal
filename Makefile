.PHONY: build build-amd64 build-arm64 deb clean release test lint docs docs-check

VERSION := $(shell cat VERSION 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X dck/cmd.version=$(VERSION)

build: build-amd64 build-arm64

build-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags netgo -installsuffix netgo -ldflags="$(LDFLAGS)" -o dck-linux-amd64 .

build-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags netgo -installsuffix netgo -ldflags="$(LDFLAGS)" -o dck-linux-arm64 .

test:
	go test ./... -count=1

lint:
	go vet ./...
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 run ./...

docs:
	sh scripts/sync-docs-version.sh

docs-check:
	sh scripts/sync-docs-version.sh --check

deb: build
	./scripts/build-deb.sh

clean:
	rm -f dck dck-linux-*
	rm -rf dist/

release: deb
	@echo "Release v$(VERSION) ready: dist/dck_$(VERSION)_amd64.deb"
