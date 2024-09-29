lint: ## Run static analysis checks
	staticcheck ./...
	go fmt ./...

test: ## Run unittest
	go test -v ./...

ready: lint test ## Runs all checks before commit

all: help

help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

PHONY: all help lint test ready
