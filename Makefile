.DEFAULT_GOAL := help
BIN := diagkit
SEED ?= 42
SCENARIO ?= payments-outage
BUNDLE ?= incident-bundle.json

.PHONY: help build test test-go test-py lint demo clean

help: ## show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  %-12s %s\n", $$1, $$2}'

build: ## build the Go collector binary
	go build -o $(BIN) ./cmd/diagkit

test: test-go test-py ## run all tests

test-go: ## run Go tests with the race detector
	go vet ./...
	go test ./... -race

test-py: ## run Python tests
	cd py && uv run --with click --with pytest pytest -q

lint: ## run gofmt check and ruff
	gofmt -l . | (! grep .) || (echo "gofmt: files need formatting" && exit 1)
	cd py && uv run --with ruff ruff check .

demo: build ## collect a seeded incident and print the ranked root cause
	./$(BIN) collect --seed $(SEED) --scenario $(SCENARIO) --out - | \
		( cd py && uv run --with click python -m diagkit_rca analyze - )

clean: ## remove build artifacts
	go clean
	rm -f $(BIN) $(BUNDLE)
