build-cli:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o mocks/goont cmd/cli/main.go

build-server:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o mocks/goont-server cmd/server/main.go
