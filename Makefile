all: check build test 

build:
	go build -o bin/etcdv3-gateway main.go

test:
	go test -race ./...

check:
	go tool vet . 
	go tool vet --shadow . 
	golint ./... 
	gofmt -s -l .