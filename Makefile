.PHONY: all build install lint test

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

build:
	$(call PROMPT, $@)
	go build -o dirtygit .

install:
	$(call PROMPT, $@)
	go install

lint:
	$(call PROMPT, $@)
	golangci-lint run

test:
	$(call PROMPT, $@)
	go test ./...