.PHONY: build test check vet fmt fmt-check coverage-check check-help docs-check auth-terminal-check mod-check changelog-context release-check release-check-ci release release-dry-run verify cli-tooling-check

build:
	go build -o todoist ./cmd/todoist

test:
	go test ./...

# Check metadata before Go commands can fill in missing checksums.
check: mod-check
	$(MAKE) fmt-check vet coverage-check docs-check auth-terminal-check

vet:
	go vet ./...

fmt:
	gofmt -w cmd internal

fmt-check:
	@files="$$(gofmt -l cmd internal)" || exit 1; \
	if [ -n "$$files" ]; then printf 'Run make fmt to format:\n%s\n' "$$files"; exit 1; fi

coverage-check:
	./scripts/coverage-check.sh

check-help:
	./scripts/check-help.sh

docs-check:
	./scripts/docs-check.sh

auth-terminal-check:
	python3 scripts/test-auth-terminal.py

mod-check: cli-tooling-check
	./scripts/mod-check.sh

changelog-context:
	@if [ -z "$(VERSION)" ]; then echo "VERSION is required (e.g. make changelog-context VERSION=v0.1.0)"; exit 2; fi
	./scripts/changelog-context.sh "$(VERSION)"

release-check:
	@if [ -z "$(VERSION)" ]; then echo "VERSION is required (e.g. make release-check VERSION=v0.1.0)"; exit 2; fi
	./scripts/release-check.sh "$(VERSION)"

release-check-ci:
	./scripts/release-check.sh --ci

release:
	@if [ -z "$(VERSION)" ]; then echo "VERSION is required (e.g. make release VERSION=v0.1.0)"; exit 2; fi
	./scripts/release.sh "$(VERSION)"

release-dry-run:
	@if [ -z "$(VERSION)" ]; then echo "VERSION is required (e.g. make release-dry-run VERSION=v0.1.0)"; exit 2; fi
	./scripts/release.sh "$(VERSION)" --dry-run

# Preserve make check as the ordinary cross-platform gate.
verify: check

cli-tooling-check:
	./scripts/check-cli-tooling.sh
