.PHONY: build fmt run test vet

fmt:
	gofmt -w ./cmd ./internal

vet:
	go vet ./...

test:
	go test ./...

build:
	go build -o bin/abcp ./cmd/abcp

run:
	go run ./cmd/abcp serve
