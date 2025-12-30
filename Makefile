.PHONY: test test-coverage lint build

test:
	go test ./... -v -race

test-coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated at coverage.html"
	@go tool cover -func=coverage.out

lint:
	golangci-lint run ./...

build:
	go build -o checkify ./cmd
