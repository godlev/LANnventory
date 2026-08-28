mod:
	rm go.mod || true && \
	rm go.sum || true && \
	go mod init github.com/godlev/LANnventory && \
	go mod tidy

run:
	cd cmd/LANnventory/ && \
	sudo \
	go run . #-n http://192.168.2.3:8850

fmt:
	go fmt ./...

lint:
	golangci-lint run
	golint ./...

check: fmt lint

swag:
	swag init -g main.go --dir cmd/LANnventory,internal/api,internal/models
