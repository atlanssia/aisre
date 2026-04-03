.PHONY: build build-release test test-contract test-api test-e2e lint clean dev frontend \
       test-cover test-cover-html test-coverage-check

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

# Run tests with coverage and print summary
test-cover:
	go test -coverprofile=coverage.out ./internal/... ./test/...
	go tool cover -func=coverage.out

# Open HTML coverage report in browser
test-cover-html: test-cover
	go tool cover -html=coverage.out

# Run tests with per-package coverage and enforce minimum thresholds
# Thresholds (from CLAUDE.md):
#   Contract (test/contract/): 100%
#   API handlers (internal/api/): 90%
#   Analysis (internal/analysis/): 85%
#   Adapter (internal/adapter/openobserve/): 90%
#   Store (internal/store/): 80%
#   Tool (internal/tool/): 80%
test-coverage-check:
	@echo "==> Running per-package coverage checks..."
	@FAIL=0; \
	packages="\
		./test/contract/:100:Contract\
		./internal/api/:90:API_handlers\
		./internal/analysis/:85:Analysis\
		./internal/adapter/openobserve/:90:Adapter\
		./internal/store/:80:Store\
		./internal/tool/:80:Tool\
	"; \
	for entry in $$packages; do \
		pkg=$$(echo "$$entry" | cut -d: -f1); \
		threshold=$$(echo "$$entry" | cut -d: -f2); \
		label=$$(echo "$$entry" | cut -d: -f3); \
		printf "  %-30s " "$$label ($$pkg)"; \
		output=$$(go test -coverprofile=coverage_$${label}.out "$$pkg" 2>&1); \
		if echo "$$output" | grep -q "no test files"; then \
			echo "SKIP (no test files)"; \
			rm -f coverage_$${label}.out; \
			continue; \
		fi; \
		if ! echo "$$output" | grep -q "coverage:"; then \
			echo "NO COVERAGE DATA"; \
			echo "$$output"; \
			FAIL=1; \
			rm -f coverage_$${label}.out; \
			continue; \
		fi; \
		pct=$$(echo "$$output" | grep 'coverage:' | sed 's/.*coverage: \([0-9.]*\)%.*/\1/'); \
		result=$$(echo "$$pct >= $$threshold" | bc 2>/dev/null); \
		if [ "$$result" = "1" ]; then \
			echo "$$pct% (min $$threshold%) PASS"; \
		else \
			echo "$$pct% (min $$threshold%) FAIL"; \
			FAIL=1; \
		fi; \
		rm -f coverage_$${label}.out; \
	done; \
	echo ""; \
	if [ "$$FAIL" = "1" ]; then \
		echo "Coverage check FAILED: one or more packages below threshold."; \
		exit 1; \
	else \
		echo "All coverage checks PASSED."; \
	fi
