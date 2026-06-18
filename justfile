set shell := ["bash", "-cu"]

install:
    go mod download
    go mod tidy

fmt:
    go fmt ./...

lint:
    go vet ./...

lint-extra:
    if command -v golangci-lint >/dev/null 2>&1; then golangci-lint run ./...; else echo "golangci-lint not found; skipped"; fi

test:
    go test ./...

check:
    go fmt ./...
    go vet ./...
    go test ./...

build:
    mkdir -p bin
    go build -o bin/termbridge.exe ./cmd/termbridge

dev *args:
    go run cmd/termbridge/main.go exec -- {{args}}

clean:
    rm -rf bin
