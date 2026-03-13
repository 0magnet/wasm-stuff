GOROOT := $(shell go env GOROOT)
TINYGOROOT := $(shell tinygo env TINYGOROOT)
PORT ?= 8080

.PHONY: all update generate build pages clean tidy

all: update generate build pages

update:
	go get -v -u
	go mod tidy
	go mod vendor

generate:
	cp "$(GOROOT)/lib/wasm/wasm_exec.js" wasm_exec.js
	cp "$(TINYGOROOT)/targets/wasm_exec.js" tinygo_wasm_exec.js
	GOOS=js GOARCH=wasm go build -ldflags="-s -w" -o b.wasm ./cmd/wasm
	tinygo build -target wasm -no-debug -o b-tiny.wasm ./cmd/wasm

build: generate
	go build -o wasm-stuff .

pages: build
	@echo "Starting server to generate pages..."
	@./wasm-stuff -p $(PORT) & PID=$$!; \
	sleep 2; \
	curl -sf http://127.0.0.1:$(PORT)/index.html -o index.html && \
	mkdir -p tinygo && \
	curl -sf http://127.0.0.1:$(PORT)/tinygo/index.html -o tinygo/index.html && \
	echo "Generated: index.html, tinygo/index.html"; \
	kill $$PID 2>/dev/null

tidy:
	go mod tidy
	go mod vendor

clean:
	rm -rf b.wasm b-tiny.wasm wasm_exec.js tinygo_wasm_exec.js wasm-stuff index.html tinygo/
