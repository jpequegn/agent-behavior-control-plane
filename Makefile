.PHONY: benchmark build demo fmt run test vet

fmt:
	gofmt -w ./cmd ./internal

vet:
	go vet ./...

test:
	go test ./...

benchmark:
	go test -bench=. -run='^$$' ./internal/...

build:
	go build -o bin/abcp ./cmd/abcp

run:
	go run ./cmd/abcp serve

demo:
	go run ./cmd/abcp demo incident
