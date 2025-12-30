.PHONY: test test-verbose test-coverage test-race test-short lint bench build clean

# Run tests with verbose output
test:
	go test ./... -v

# Run tests with verbose output and disable caching
test-verbose:
	go test ./... -v -count=1

# Run tests with coverage report
test-coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated at coverage.html"
	@go tool cover -func=coverage.out

# Run tests with race detector
test-race:
	go test ./... -race

# Run short tests only
test-short:
	go test ./... -short

# Run linter (requires golangci-lint)
lint:
	golangci-lint run ./...

# Run benchmarks
bench:
	go test ./... -bench=. -benchmem

# Build the binary
build:
	go build -o checkify ./cmd

# Clean build artifacts and coverage files
clean:
	rm -f checkify
	rm -f coverage.out coverage.html
	rm -f *.test
	find . -name '*.coverprofile' -delete
	find . -name 'profile.cov' -delete
