BINARY=justdoit
PREFIX=/opt/homebrew/bin
CONFIG_DIR=$(HOME)/.config/justdoit

.PHONY: build install setup reset tidy lint test

build:
	go build -o $(BINARY) ./cmd/justdoit

install:
	go build -o $(PREFIX)/$(BINARY) ./cmd/justdoit

setup:
	$(PREFIX)/$(BINARY) setup

reset:
	rm -f $(CONFIG_DIR)/config.json $(CONFIG_DIR)/token.json

tidy:
	go mod tidy

lint:
	golangci-lint run

test:
	go test ./...
