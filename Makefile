.PHONY: help all build install install-git-hooks lint test goreleaser-check dist

#: lint, test, and build (default)
all: lint test build

define PROMPT
	@echo
	@echo "**********************************************************"
	@echo "*"
	@echo "*   $(1)"
	@echo "*"
	@echo "**********************************************************"
	@echo
endef

#: compile ./dirtygit for the current platform in the repo root
build:
	$(call PROMPT, $@)
	go build -o dirtygit .

#: run goreleaser to build release binaries for all platforms under dist/
dist:
	$(call PROMPT, $@)
	goreleaser build --snapshot --clean

#: install the binary in $GOPATH/bin
install:
	$(call PROMPT, $@)
	go install

#: install commit-msg hook for Conventional Commits (.git/hooks on this clone)
install-git-hooks:
	$(call PROMPT, $@)
	@test -d .git || (echo "error: not a git repository (no .git directory)" >&2; exit 1)
	cp scripts/git-hooks/commit-msg .git/hooks/commit-msg
	chmod +x .git/hooks/commit-msg
	@echo "Installed .git/hooks/commit-msg"

#: run linters
lint:
	$(call PROMPT, $@)
	golangci-lint run
	markdownlint '**/*.md'

#: run all tests
test:
	$(call PROMPT, $@)
	go test ./...

#: validate .goreleaser.yaml (needs goreleaser on PATH)
goreleaser-check:
	$(call PROMPT, $@)
	goreleaser check

#: print Makefile targets and short descriptions
help:
	@echo "make targets:\n"
	@awk '/^#:[[:space:]]/ { sub(/^#:[[:space:]]*/, ""); desc=$$0; next } \
		/^[[:space:]]*$$/ { next } \
		/^#/ { next } \
		/^[a-zA-Z][a-zA-Z0-9_.-]*:/ { \
			if (desc != "") { \
				split($$0, a, ":"); \
				tgt=a[1]; \
				gsub(/^[[:space:]]+|[[:space:]]+$$/, "", tgt); \
				printf "  %-18s %s\n", tgt, desc; \
				desc="" \
			} \
		}' $(firstword $(MAKEFILE_LIST))
