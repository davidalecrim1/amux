BIN := bin/amux
PREFIX ?= $(HOME)/.local

.PHONY: build install test fmt vet clean

build:
	@mkdir -p bin
	go build -o $(BIN) ./cmd/amux

install: build
	@mkdir -p $(PREFIX)/bin
	install -m 0755 $(BIN) $(PREFIX)/bin/amux
	@echo "installed $(PREFIX)/bin/amux"

test:
	go test ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './bin/*')

vet:
	go vet ./...

clean:
	rm -rf bin
