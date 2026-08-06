BIN := $(HOME)/bin/ollie-hooks
PKG := ./cmd/ollie-hooks

.PHONY: build test vet fmt install

build:
	go build -o ollie-hooks $(PKG)

vet:
	go vet ./...

fmt:
	gofmt -l -w .

test:
	go test -race ./...

# install refuses to ship a binary that doesn't vet and pass its tests.
install: vet test
	mkdir -p $(HOME)/bin
	go build -o $(BIN) $(PKG)
	@echo "installed $(BIN)"
