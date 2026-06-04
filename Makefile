.PHONY: build test install
build:
	go build -o bin/claude-split .
test:
	go test ./...
install:
	go install .
