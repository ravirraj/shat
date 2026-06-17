.PHONY: build build-server build-client test test-race bench lint clean docker-build run-server run-client

build: build-server build-client

build-server:
	go build -o bin/server ./cmd/server

build-client:
	go build -o bin/client ./cmd/client

test:
	go test ./... -count=1

test-race:
	go test -race ./... -count=1

bench:
	go test -bench=. -benchmem ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/
	rm -f shat.db

docker-build:
	docker build -t shat-server .

run-server: build-server
	./bin/server

run-client: build-client
	./bin/client localhost:8000
