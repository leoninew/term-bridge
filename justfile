set shell := ["bash", "-cu"]

install:
    go mod download
    go mod tidy
    cd web && yarn install

check:
    go fmt ./cmd/... ./internal/...
    go vet ./cmd/... ./internal/...
    if command -v golangci-lint >/dev/null 2>&1; then golangci-lint run ./cmd/... ./internal/...; else echo "golangci-lint not found; skipped"; fi
    go test ./cmd/... ./internal/...
    cd web && yarn format
    cd web && yarn typecheck
    cd web && yarn lint

build:
    mkdir -p bin
    go build -o bin/termbridge.exe ./cmd/termbridge

exec *args:
    go run cmd/termbridge/main.go exec -- {{args}}

backend:
    go run cmd/termbridge/main.go web --host 127.0.0.1 --port 9010 --dev

frontend:
    cd web && yarn dev

clean:
    rm -rf bin
