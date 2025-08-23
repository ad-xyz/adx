# LuxFi ADX Makefile
# High-Performance CTV Ad Exchange with GPU Acceleration

SHELL := /bin/bash
.PHONY: all build test clean help detect benchmark demo

# Version and build info
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Go parameters
GO := go
GOBUILD := $(GO) build
GOCLEAN := $(GO) clean
GOTEST := $(GO) test
GOGET := $(GO) get
GOMOD := $(GO) mod
GOVET := $(GO) vet
GOFMT := gofmt

# Build parameters
CGO_ENABLED ?= 0
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

# Binary output
BINARY_NAME := adx-exchange
MINER_BINARY := adx-miner
BINARY_DIR := bin

# Test parameters
TEST_TIMEOUT := 30s
BENCH_TIME := 10s

# Default target - build, test, then benchmark
all: build test bench
	@echo "✅ ADX build, test, and benchmark complete!"

help:
	@echo "LuxFi ADX Makefile Commands:"
	@echo ""
	@echo "Development:"
	@echo "  make build         - Build all binaries"
	@echo "  make test          - Run all tests"
	@echo "  make bench         - Run benchmarks"
	@echo "  make clean         - Clean build artifacts"
	@echo ""
	@echo "Running:"
	@echo "  make run-exchange  - Run ADX exchange"
	@echo "  make run-miner     - Run home miner"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-build  - Build Docker images"
	@echo "  make docker-run    - Run with Docker Compose"
	@echo "  make docker-clean  - Clean Docker resources"

# Build targets
build: build-exchange build-miner
	@echo "✅ All binaries built successfully"

build-exchange:
	@echo "🔨 Building ADX exchange..."
	@mkdir -p $(BINARY_DIR)
	@CGO_ENABLED=$(CGO_ENABLED) $(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME) ./cmd/adx-exchange

build-miner:
	@echo "🔨 Building ADX miner..."
	@mkdir -p $(BINARY_DIR)
	@CGO_ENABLED=$(CGO_ENABLED) $(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(MINER_BINARY) ./cmd/adx-miner

# Test targets
test:
	@echo "🧪 Running tests..."
	@$(GOTEST) -v -timeout $(TEST_TIMEOUT) ./pkg/vast/... ./pkg/rtb/... ./pkg/miner/...

test-coverage:
	@echo "📊 Running tests with coverage..."
	@$(GOTEST) -v -coverprofile=coverage.out -timeout $(TEST_TIMEOUT) ./...
	@$(GO) tool cover -html=coverage.out -o coverage.html

bench:
	@echo "⚡ Running benchmarks..."
	@$(GOTEST) -bench=. -benchtime=$(BENCH_TIME) ./pkg/rtb/...

# Code quality
fmt:
	@echo "📝 Formatting code..."
	@$(GOFMT) -w .

vet:
	@echo "🔍 Running go vet..."
	@$(GOVET) ./...

lint:
	@echo "🔍 Running linter..."
	@golangci-lint run

# Running targets
run-exchange:
	@echo "🚀 Starting ADX exchange..."
	@$(BINARY_DIR)/$(BINARY_NAME)

run-miner:
	@echo "⛏️ Starting ADX miner..."
	@$(BINARY_DIR)/$(MINER_BINARY) --tunnel localxpose --cache-size 10GB

# Docker targets
docker-build:
	@echo "🐳 Building Docker images..."
	@docker build -t luxfi/adx-exchange:$(VERSION) -f docker/exchange/Dockerfile .
	@docker build -t luxfi/adx-miner:$(VERSION) -f docker/miner/Dockerfile .

docker-run:
	@echo "🐳 Starting ADX with Docker Compose..."
	@docker-compose up -d

docker-stop:
	@echo "🛑 Stopping Docker services..."
	@docker-compose down

docker-clean:
	@echo "🧹 Cleaning Docker resources..."
	@docker-compose down -v
	@docker rmi luxfi/adx-exchange:$(VERSION) luxfi/adx-miner:$(VERSION) 2>/dev/null || true

# FoundationDB setup
setup-fdb:
	@echo "📀 Setting up FoundationDB..."
	@wget https://github.com/apple/foundationdb/releases/download/7.3.27/foundationdb-clients_7.3.27-1_amd64.deb
	@sudo dpkg -i foundationdb-clients_7.3.27-1_amd64.deb
	@rm foundationdb-clients_7.3.27-1_amd64.deb

# Clean target
clean:
	@echo "🧹 Cleaning build artifacts..."
	@$(GOCLEAN)
	@rm -rf $(BINARY_DIR)
	@rm -f coverage.out coverage.html

# Installation
install:
	@echo "📦 Installing ADX binaries..."
	@$(GO) install -v ./cmd/adx-exchange
	@$(GO) install -v ./cmd/adx-miner

# Dependencies
deps:
	@echo "📦 Downloading dependencies..."
	@$(GOMOD) download
	@$(GOMOD) tidy

# Engine-specific builds
detect:
	@echo "=== System Detection ==="
	@echo "OS: $$(uname -s)"
	@echo "Arch: $$(uname -m)"
	@echo "CPU Cores: $$(sysctl -n hw.ncpu 2>/dev/null || nproc 2>/dev/null || echo 'unknown')"
	@if [ "$$(uname -s)" = "Darwin" ]; then \
		echo "Metal: Available ✓"; \
		system_profiler SPDisplaysDataType 2>/dev/null | grep "Chipset Model" || echo "GPU: Apple Silicon"; \
	fi
	@which nvcc >/dev/null 2>&1 && echo "CUDA: Available ✓" || echo "CUDA: Not found"
	@echo "======================="

build-go:
	@echo "🔨 Building pure Go engine..."
	@mkdir -p $(BINARY_DIR)
	@$(GOBUILD) -tags purego $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME)-go ./cmd/adx-exchange
	@echo "✓ Pure Go build complete"

build-cpu:
	@echo "🔨 Building CPU-optimized engine..."
	@mkdir -p $(BINARY_DIR)
	@CGO_ENABLED=1 $(GOBUILD) -tags "cgo cpu" $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME)-cpu ./cmd/adx-exchange
	@echo "✓ CPU-optimized build complete"

build-gpu:
	@echo "🔨 Building GPU-accelerated engine..."
	@mkdir -p $(BINARY_DIR)
	@if [ "$$(uname -s)" = "Darwin" ]; then \
		CGO_ENABLED=1 $(GOBUILD) -tags "cgo gpu metal" $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME)-gpu ./cmd/adx-exchange; \
	else \
		CGO_ENABLED=1 $(GOBUILD) -tags "cgo gpu cuda" $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME)-gpu ./cmd/adx-exchange; \
	fi
	@echo "✓ GPU build complete"

build-all-engines: build-go build-cpu build-gpu
	@echo "✅ All engine implementations built"

benchmark:
	@echo "=== Benchmarking Matching Engines ==="
	@echo "1. Pure Go Implementation:"
	@$(GOTEST) -bench=BenchmarkPureGo -benchtime=$(BENCH_TIME) ./pkg/dex
	@echo ""
	@echo "2. CPU-Optimized (if available):"
	@CGO_ENABLED=1 $(GOTEST) -tags "cgo cpu" -bench=BenchmarkCPU -benchtime=$(BENCH_TIME) ./pkg/dex 2>/dev/null || echo "Not available"
	@echo ""
	@echo "3. GPU-Accelerated (if available):"
	@CGO_ENABLED=1 $(GOTEST) -tags "cgo gpu" -bench=BenchmarkGPU -benchtime=$(BENCH_TIME) ./pkg/dex 2>/dev/null || echo "Not available"

demo:
	@echo "=== ADXYZ Performance Progression Demo ==="
	@echo ""
	@echo "Stage 1: Pure Go Implementation"
	@echo "--------------------------------"
	@$(BINARY_DIR)/$(BINARY_NAME)-go -demo 2>/dev/null || echo "Not built"
	@echo ""
	@echo "Stage 2: CPU-Optimized C++ with SIMD"
	@echo "-------------------------------------"
	@$(BINARY_DIR)/$(BINARY_NAME)-cpu -demo 2>/dev/null || echo "Not built"
	@echo ""
	@echo "Stage 3: GPU-Accelerated (CUDA/Metal)"
	@echo "--------------------------------------"
	@$(BINARY_DIR)/$(BINARY_NAME)-gpu -demo 2>/dev/null || echo "Not built"

# CI targets
ci: deps fmt vet test build
	@echo "✅ CI pipeline complete"

# Development setup
dev-setup:
	@echo "🛠️ Setting up development environment..."
	@$(GOGET) -u github.com/golangci/golangci-lint/cmd/golangci-lint
	@$(GOGET) -u github.com/apple/foundationdb/bindings/go/src/fdb
	@$(GOGET) -u github.com/prebid/openrtb/v20
	@echo "✅ Development environment ready"

.DEFAULT_GOAL := help