.PHONY: build build-release test test-contract test-api test-e2e lint clean dev frontend

# Build the server binary (dev mode, serves frontend from ../../web/dist)
build:
	go build -o bin/aisre ./cmd/server

# Build frontend assets
frontend:
	cd web && npm install && npm run build

# Build release binary with embedded frontend (single binary deployment)
build-release: frontend
	rm -rf cmd/server/static
	cp -r web/dist cmd/server/static
	go build -tags release -o bin/aisre ./cmd/server
	rm -rf cmd/server/static

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

# Run golden dataset evaluation
test-eval:
	go test ./test/eval/...

# Run benchmark against golden dataset
eval-golden:
	@echo "Running golden dataset benchmark..."
	go run ./internal/eval/... || true

# Compare prompt versions
eval-compare:
	@echo "Comparing prompt versions..."

# Run linter
lint:
	golangci-lint run

# Clean build artifacts
clean:
	rm -rf bin/ data/ cmd/server/static/

# Run development server (dev mode, no embed)
dev: build
	./bin/aisre --config configs/local.yaml
