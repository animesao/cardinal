.PHONY: build build-amd64 build-arm64 completion completion-bash completion-zsh completion-fish completion-powershell deb clean release test test-race test-cover lint audit audit-strict fuzz docs docs-check verify-cobra

VERSION := $(shell cat VERSION 2>/dev/null || echo "dev")
# Reproducible-build flags: strip paths and VCS info from the binary.
GOFLAGS  := -trimpath
LDFLAGS  := -s -w -buildid= -X dck/cmd.version=$(VERSION)

build: build-amd64 build-arm64

build-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build -tags netgo -installsuffix netgo \
		$(GOFLAGS) -ldflags="$(LDFLAGS)" -o dck-linux-amd64 .

build-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
		go build -tags netgo -installsuffix netgo \
		$(GOFLAGS) -ldflags="$(LDFLAGS)" -o dck-linux-arm64 .

# Verify the cobra command tree compiles and matches the expected shape.
# Run on every CI push so the registration stays in sync with the legacy
# free functions.
verify-cobra:
	go vet ./cmd/...
	go build ./cmd/...

# Cobra auto-generates a `completion` sub-command; we expose helpers so
# users piping into their .bashrc / .zshrc / fish-config / PowerShell
# profile have a single entry point.
completion:
	@echo "dck completion bash | zsh | fish | powershell"
	@echo
	@echo "Examples:"
	@echo "  dck completion bash > ~/.local/share/bash-completion/completions/dck"
	@echo "  dck completion zsh > \"\$${fpath[1]}/_dck\""
	@echo "  dck completion fish > ~/.config/fish/completions/dck.fish"

completion-bash:
	@./dck-linux-amd64 completion bash

completion-zsh:
	@./dck-linux-amd64 completion zsh

completion-fish:
	@./dck-linux-amd64 completion fish

completion-powershell:
	@./dck-linux-amd64 completion powershell

test:
	go test ./... -count=1

test-race:
	go test -race ./... -count=1

test-cover:
	go test ./... -count=1 -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out | tail -1

lint:
	go vet ./...
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 run ./...

# Repository static audit (fail on PASS-list violations).
audit:
	bash scripts/audit.sh

audit-strict:
	bash scripts/audit.sh --strict

fuzz:
	@echo "Run fuzz tests for short duration (~30s each)"
	go test -run='^$$' -fuzz='^Fuzz' -fuzztime=30s ./internal/builder/

docs:
	sh scripts/sync-docs-version.sh

docs-check:
	sh scripts/sync-docs-version.sh --check

deb: build
	./scripts/build-deb.sh

clean:
	rm -f dck dck-linux-*
	rm -rf dist/ coverage.out

release: deb
	@echo "Release v$(VERSION) ready: dist/dck_$(VERSION)_amd64.deb"
