.PHONY: build start run release rerelease

build:
	go build -ldflags="-s -w -X main.version=dev" -o dbq ./cmd

start: build
	./dbq start

release:
	@latest=$$(git tag --sort=-version:refname | head -1); \
	if [ -z "$$latest" ]; then next="v0.1.0"; \
	else \
		patch=$$(echo $$latest | cut -d. -f3); \
		prefix=$$(echo $$latest | cut -d. -f1-2); \
		next="$$prefix.$$((patch + 1))"; \
	fi; \
	echo "Tagging $$next"; \
	git tag $$next && git push origin $$next

rerelease:
	@latest=$$(git tag --sort=-version:refname | head -1); \
	if [ -z "$$latest" ]; then echo "No tag to re-cut"; exit 1; fi; \
	echo "Re-cutting $$latest"; \
	git tag -d $$latest; \
	git push origin :refs/tags/$$latest; \
	git tag $$latest && git push origin $$latest
