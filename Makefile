.PHONY: run test check build

run:
	go run ./cmd/ledgerlite

test:
	go test -race ./...

check:
	go vet ./...
	go test -race ./...

build:
	go build -o bin/ledgerlite ./cmd/ledgerlite
