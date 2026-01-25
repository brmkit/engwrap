BINARY_NAME=engwrap
MAIN_PATH=./cmd/engwrap
BUILD_DIR=./build

.PHONY: all clean darwin linux windows deps check

all: deps darwin linux windows

check:
	@echo "Checking prerequisites"
	@command -v go >/dev/null 2>&1 || { echo "Error: go is not installed. Please install Go first."; exit 1; }
	@echo "Go version: $$(go version)"

deps: check
	@echo "Updating dependencies"
	@go mod tidy
	@go mod download
	@echo "Dependencies updated"

darwin: deps
	@echo "Building for macOS"
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	GOOS=darwin GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)

linux: deps
	@echo "Building for Linux"
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	GOOS=linux GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PATH)

windows: deps
	@echo "Building for Windows"
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)
	GOOS=windows GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-windows-arm64.exe $(MAIN_PATH)

clean:
	@echo "Cleaning build directory"
	@rm -rf $(BUILD_DIR)

