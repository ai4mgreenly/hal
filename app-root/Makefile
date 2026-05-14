.PHONY: build test install

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o hal .

test:
	go test ./...

install: build
	mkdir -p $(HOME)/.local/bin
	install -m 0755 hal $(HOME)/.local/bin/hal
