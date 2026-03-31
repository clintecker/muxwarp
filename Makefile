APP_NAME   := muxwarp
BIN_DIR    := bin
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT     := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE       := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS    := -s -w \
              -X main.version=$(VERSION) \
              -X main.commit=$(COMMIT) \
              -X main.date=$(DATE)

.PHONY: all build build-pi install lint test clean run check hooks

all: lint test build

build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(APP_NAME) ./cmd/muxwarp

build-pi:
	@mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(APP_NAME)-linux-arm64 ./cmd/muxwarp

install: build
	go install -ldflags "$(LDFLAGS)" ./cmd/muxwarp

lint:
	golangci-lint run ./...

test:
	go test -race -count=1 ./...

clean:
	rm -rf $(BIN_DIR)

run: build
	./$(BIN_DIR)/$(APP_NAME)

check: lint test
	@echo "All checks passed"

hooks:
	git config core.hooksPath .githooks
	@echo "Git hooks installed"
