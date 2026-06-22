set shell := ["bash", "-cu"]

# Show available tasks
_default:
    just --list

# --- Setup ---

# Install web and Go dependencies
install:
    cd web && yarn install
    go mod download
    go mod tidy

# Remove build outputs
clean:
    rm -rf web/dist bin

# --- Checks ---

# Run frontend and backend checks
check:
    cd web && yarn typecheck
    cd web && yarn lint
    cd web && yarn format:check
    cd web && yarn test
    go fmt ./cmd/... ./internal/...
    go vet ./cmd/... ./internal/...
    go test ./cmd/... ./internal/...

# Run frontend and backend unit tests
test:
    cd web && yarn test
    go test ./cmd/... ./internal/...

# Build web frontend and termbridge binary
build:
    cd web && yarn build
    mkdir -p bin
    go build -o bin/termbridge ./cmd/termbridge

# --- Local development ---

# Run local command through termbridge exec
exec *args:
    go run cmd/termbridge/main.go exec -- {{args}}

# Start web frontend on localhost:9011, proxying to serve backend
web:
    cd web && yarn dev

# Start unified backend with Air hot reload
serve:
    air -c .air.toml
