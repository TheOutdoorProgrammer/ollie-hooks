BIN := $(HOME)/bin/ollie-hooks
PKG := ./cmd/ollie-hooks

.PHONY: build test vet fmt install docs

build:
	go build -o ollie-hooks $(PKG)

vet:
	go vet ./...

fmt:
	gofmt -l -w .

test:
	go test -race ./...

# docs are generated from the registered rules, never hand-edited: the docs
# and the code cannot disagree if only one of them is written by a person.
docs:
	go run ./internal/docsgen

# install refuses to ship a binary that doesn't vet and pass its tests.
install: vet test
	mkdir -p $(HOME)/bin
	go build -o $(BIN) $(PKG)
	@echo "installed $(BIN)"
