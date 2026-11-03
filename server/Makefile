.PHONY: build build-local vet test run-dev

# Linux x86_64 static binary for the production server.
build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o bin/ngajio .

# Binary for this machine.
build-local:
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o bin/ngajio-local .

vet:
	go vet ./...

test:
	go test -race ./...

# Local server with the dev login on 127.0.0.1:8787, data in /tmp/ngajio-dev.
run-dev:
	ENV=dev DEV_LOGIN=1 LISTEN=127.0.0.1:8787 DB_PATH=/tmp/ngajio-dev/ngajio.db COOKIE_SECURE=false go run .
