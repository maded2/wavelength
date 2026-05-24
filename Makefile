# Wavelength — Multi-Platform Build

BINARY_NAME := wavelength
GO           := go
CMD          := cmd/server/main.go

# Platforms: GOOS/GOARCH pairs
LINUX_AMD64  := linux/amd64
LINUX_ARM64  := linux/arm64
DARWIN_AMD64 := darwin/amd64
DARWIN_ARM64 := darwin/arm64
WINDOWS_AMD64 := windows/amd64
WINDOWS_ARM64 := windows/arm64

# Detect current platform
GOOS_CURRENT  := $(shell go env GOOS)
GOARCH_CURRENT := $(shell go env GOARCH)

# Build output directory
OUT_DIR := dist

# Map platform to binary name
$(LINUX_AMD64)_BIN    := $(BINARY_NAME)-linux-amd64
$(LINUX_ARM64)_BIN    := $(BINARY_NAME)-linux-arm64
$(DARWIN_AMD64)_BIN   := $(BINARY_NAME)-darwin-amd64
$(DARWIN_ARM64)_BIN   := $(BINARY_NAME)-darwin-arm64
$(WINDOWS_AMD64)_BIN  := $(BINARY_NAME)-windows-amd64.exe
$(WINDOWS_ARM64)_BIN  := $(BINARY_NAME)-windows-arm64.exe

.PHONY: build run test clean build-all build-current

# Default: build for current platform
build: build-current

# Build for the host platform
build-current:
	$(GO) build -o $(BINARY_NAME) $(CMD)

# Run on current platform (Linux/macOS only)
run: build-current
	./$(BINARY_NAME) -config configs/config.json

# Run tests
test:
	$(GO) test ./...

# Clean all build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -rf $(OUT_DIR)

# Build for all supported platforms
build-all: $(OUT_DIR)
	@echo "Building for all platforms..."
	$(MAKE) _build-platform PLATFORM=$(LINUX_AMD64)
	$(MAKE) _build-platform PLATFORM=$(LINUX_ARM64)
	$(MAKE) _build-platform PLATFORM=$(DARWIN_AMD64)
	$(MAKE) _build-platform PLATFORM=$(DARWIN_ARM64)
	$(MAKE) _build-platform PLATFORM=$(WINDOWS_AMD64)
	$(MAKE) _build-platform PLATFORM=$(WINDOWS_ARM64)
	@echo "All builds complete. Outputs in $(OUT_DIR)/"

# Internal target: build for a single platform
_build-platform:
	@GOOS=$(word 1,$(subst /, ,$(PLATFORM))) \
	GOARCH=$(word 2,$(subst /, ,$(PLATFORM))) \
	$(GO) build -o $(OUT_DIR)/$($(PLATFORM)_BIN) $(CMD)
	@echo "  Built $(OUT_DIR)/$($(PLATFORM)_BIN)"

# Create output directory
$(OUT_DIR):
	mkdir -p $(OUT_DIR)

# Convenience targets for individual platforms
build-linux-amd64: $(OUT_DIR)
	$(MAKE) _build-platform PLATFORM=$(LINUX_AMD64)

build-linux-arm64: $(OUT_DIR)
	$(MAKE) _build-platform PLATFORM=$(LINUX_ARM64)

build-darwin-amd64: $(OUT_DIR)
	$(MAKE) _build-platform PLATFORM=$(DARWIN_AMD64)

build-darwin-arm64: $(OUT_DIR)
	$(MAKE) _build-platform PLATFORM=$(DARWIN_ARM64)

build-windows-amd64: $(OUT_DIR)
	$(MAKE) _build-platform PLATFORM=$(WINDOWS_AMD64)

build-windows-arm64: $(OUT_DIR)
	$(MAKE) _build-platform PLATFORM=$(WINDOWS_ARM64)
