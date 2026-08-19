.PHONY: build start run release rerelease snapshot

VERSION ?= dev
GOOS    ?= $(shell go env GOOS)
GOARCH  ?= $(shell go env GOARCH)
LDFLAGS  = -s -w -X main.version=$(VERSION)

build:
	go build -ldflags="$(LDFLAGS)" -o dbq ./cmd

start: build
	./dbq start

# Releasing = push a tag; .github/workflows/release.yml builds + publishes.
# Auto-bumps the patch of the newest tag; override with VERSION=v1.2.3.
release:
	@if [ "$(VERSION)" != "dev" ]; then next="$(VERSION)"; \
	else \
		latest=$$(git tag --sort=-version:refname | head -1); \
		if [ -z "$$latest" ]; then next="v0.1.0"; \
		else \
			patch=$$(echo $$latest | cut -d. -f3); \
			prefix=$$(echo $$latest | cut -d. -f1-2); \
			next="$$prefix.$$((patch + 1))"; \
		fi; \
	fi; \
	echo "Tagging $$next"; \
	git tag $$next && git push origin $$next

# Deletes + re-pushes the newest tag to re-run its workflow.
rerelease:
	@latest=$$(git tag --sort=-version:refname | head -1); \
	if [ -z "$$latest" ]; then echo "No tag to re-cut"; exit 1; fi; \
	echo "Re-cutting $$latest"; \
	git tag -d $$latest; \
	git push origin :refs/tags/$$latest; \
	git tag $$latest && git push origin $$latest

# Local artifact build, identical to what CI uploads (test before cutting a tag):
# make snapshot VERSION=v1.2.3 GOOS=darwin GOARCH=arm64
snapshot:
	@mkdir -p dist
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -trimpath -ldflags="$(LDFLAGS)" -o dist/dbq ./cmd
	tar -czf dist/dbq_$(VERSION)_$(GOOS)_$(GOARCH).tar.gz -C dist dbq && rm dist/dbq
