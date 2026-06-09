.PHONY: build run test test-cover lint fmt vet check ci clean

BINARY_NAME=gah-server
BUILD_DIR=./bin

build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/api

run: build
	$(BUILD_DIR)/$(BINARY_NAME)

test:
	go test ./... -v -race

test-cover:
	go test ./... -v -race -coverprofile=coverage.out
	go tool cover -func=coverage.out

lint:
	golangci-lint run ./...

fmt:
	gofmt -l -d .

fmt-fix:
	gofmt -w .

vet:
	go vet ./...

check: fmt vet lint test

ci: check

clean:
	rm -rf $(BUILD_DIR) coverage.out
