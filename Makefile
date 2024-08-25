help: ## Displays help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-z0-9A-Z_-]+:.*?##/ { printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

include .bingo/Variables.mk

FILES_TO_FMT  ?= $(shell find . -path ./vendor -prune -o -name '*.go' -print)
FILES_TO_TEST ?= $(shell go list ./... | grep -v /vendor/)

.PHONY: all
all: go-format go-lint go-test go-bench go-build

.PHONY: install
install: $(BINGO)
	$(BINGO) get

.PHONY: format
format: ## Runs all format targets.
format: go-format

.PHONY: lint
lint: ## Runs all lint targets.
lint: go-lint

.PHONY: test
test: ## Runs all test targets.
test: go-test

.PHONY: bench
bench: ## Runs all bench targets.
bench: go-bench

.PHONY: build
build: ## Runs all build targets.
build: go-build

.PHONY: go-format
go-format: ## Formats Go code including imports.
go-format: $(GOIMPORTS) $(GOFUMPT)
	@echo ">> formatting Go code"
	@$(GOFUMPT) -s -w $(FILES_TO_FMT)
	@$(GOIMPORTS) -w $(FILES_TO_FMT)

.PHONY: go-lint
go-lint: ## Lints Go code.
go-lint: $(GOLANGCI_LINT)
	@echo ">> linting Go code"
	@$(GOLANGCI_LINT) run

.PHONY: go-test
go-test: ## Tests Go code.
go-test:
	@echo ">> running Go tests"
	@$(GO) test -race $(FILES_TO_TEST);

.PHONY: go-test-action
go-test-action:
go-test-action: $(GOTEST2ACTION)
	$(GO) test -race -covermode=atomic -json $(FILES_TO_TEST) | $(GOTEST2ACTION) \
		--passthrough \
		--root-pkg github.com/MacroPower/go_template

.PHONY: go-bench
go-bench: ## Benchmarks Go code.
go-bench: $(BENCHSTAT)
	@echo ">> running Go benchmarks"
	@mkdir -p .test
	@if [ -f .test/new.txt ]; then mv .test/new.txt .test/old.txt; fi
	@$(GO) test -bench=. -benchmem -count=3 $(FILES_TO_TEST) | tee .test/new.txt
	@$(BENCHSTAT) .test/old.txt .test/new.txt

.PHONY: go-build
go-build: ## Builds Go executables.
go-build: HOSTNAME:=$(shell hostname)
go-build: $(GORELEASER)
	@echo ">> building Go executables"
	HOSTNAME=$(HOSTNAME) $(GORELEASER) build --snapshot --clean
