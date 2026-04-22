.PHONY: build test lint clean

BINARY_NAME=takeout-helper
BUILD_DIR=bin

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/takeout-helper

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -rf $(BUILD_DIR)
