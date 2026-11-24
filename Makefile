.PHONY: build install test

build:
	mkdir -p bin
	go build -o bin/zero ./cmd/zero
	go build -o bin/zero-cp ./cmd/zero-cp

install: build
	sudo bin/zero install

test:
	go vet ./...
	go test -count=1 ./...
