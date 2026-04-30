.PHONY: help all build install install-git-hooks lint lint-go lint-markdown test tidy lint-goreleaser dist

#: lint, test, and build (default)
all: lint test tidy build

define PROMPT
	@echo
	@echo "**********************************************************"
	@echo "*"
	@echo "*   $(1)"
	@echo "*"
	@echo "**********************************************************"
	@echo
endef

#: compile for the current platform
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

#: run all linters
lint: lint-go lint-markdown lint-goreleaser

#: run Go linters
lint-go:
	$(call PROMPT, $@)
	golangci-lint run

#: run Markdown linters
lint-markdown:
	$(call PROMPT, $@)
	markdownlint '**/*.md'

#: validate .goreleaser.yaml (needs goreleaser on PATH)
lint-goreleaser:
	$(call PROMPT, $@)
	goreleaser check

#: run all tests
test:
	$(call PROMPT, $@)
	go test ./...

#: tidy go.mod and go.sum
tidy:
	$(call PROMPT, $@)
	go mod tidy

#: install commit-msg hook for Conventional Commits (.git/hooks on this clone)
install-git-hooks:
	$(call PROMPT, $@)
	@test -d .git || (echo "error: not a git repository (no .git directory)" >&2; exit 1)
	cp scripts/git-hooks/commit-msg .git/hooks/commit-msg
	chmod +x .git/hooks/commit-msg
	@echo "Installed .git/hooks/commit-msg"

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
