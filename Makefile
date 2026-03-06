CONFIG ?= config.yaml
PORT ?= 8000

.PHONY: run build test audit-regex

run:
	go run ./cmd/notediscovery -config $(CONFIG) -port $(PORT)

build:
	go build ./...

test:
	go test ./...

audit-regex:
	@echo "Checking for RE2-incompatible regex constructs in Go code..."
	@! rg -n '\(\?=|\(\?!|\(\?<=|\(\?<!|\\[1-9]' --glob '*.go' internal cmd | rg 'regexp\.'
