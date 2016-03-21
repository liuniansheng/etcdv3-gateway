all: build test

build:
	go build -o bin/etcdv3-gateway main.go

test:
	go test -race ./...