.PHONY: all deps build test lint docker-build clean

all: build

deps:
	go mod tidy

build: deps
	go build -o bin/fault-cli ./cmd/fault-cli

test:
	go test -v -race -coverprofile=coverage.out ./...

lint:
	golangci-lint run

docker-build:
	docker build -t aashiruu/fault-sentinel:latest .

clean:
	rm -rf bin/ coverage.out
