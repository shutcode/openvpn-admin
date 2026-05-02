.PHONY: build run clean dev

build:
	go build -o bin/openvpn-dashboard main.go

run:
	go run main.go

dev:
	PORT=8080 go run main.go

clean:
	rm -rf bin/

test:
	go test ./...

.DEFAULT_GOAL := build
