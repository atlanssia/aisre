.PHONY: build test test-contract test-api test-e2e lint clean

# Build the server binary
build:
	go build -o bin/aisre ./cmd/server

# Run all tests
test:
	go test ./...

# Run contract tests
test-contract:
	go test ./test/contract/...

# Run API handler tests
test-api:
	go test ./test/api/...

# Run adapter tests
test-adapter:
	go test ./test/adapter/...

# Run analysis engine tests
test-analysis:
	go test ./test/analysis/...

# Run end-to-end tests
test-e2e:
	go test ./test/e2e/...

# Run linter
lint:
	golangci-lint run

# Clean build artifacts
clean:
	rm -rf bin/ data/

# Run development server
dev: build
	./bin/aisre --config configs/local.yaml
