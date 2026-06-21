set shell := ["bash", "-cu"]

# Show available tasks
_default:
    just --list

# --- Setup ---

# Install Go and web dependencies
install:
    go mod download
    go mod tidy
    cd web && yarn install

# Remove build outputs
clean:
    rm -rf bin web/dist

# --- Checks ---

# Run full project checks
check: fmt vet test web-check

# Format Go code
fmt:
    go fmt ./cmd/... ./internal/...

# Vet Go code
vet:
    go vet ./cmd/... ./internal/...

# Run Go tests
test:
    go test ./...

# Run web typecheck, lint, and production build
web-check: web-typecheck web-lint web-build

# Run web typecheck
web-typecheck:
    cd web && yarn typecheck

# Run web lint
web-lint:
    cd web && yarn lint

# Build web frontend
web-build:
    cd web && yarn build

# Build termbridge binary
build:
    mkdir -p bin
    go build -o bin/termbridge ./cmd/termbridge

# --- Local development ---

# Run local command through termbridge exec
exec *args:
    go run cmd/termbridge/main.go exec -- {{args}}

# Start local web backend with Air on localhost:9010
web-backend:
    air

# Start web frontend on localhost:9011, proxying to local web backend localhost:9010
web-frontend:
    cd web && yarn dev

# Start Gateway service on 127.0.0.1:8080
gateway host="127.0.0.1" port="8080":
    go run cmd/termbridge/main.go gateway --host {{host}} --port {{port}} --dev

# Start Agent and connect it to Gateway
agent gateway_url="http://127.0.0.1:8080" device_name="local-dev":
    go run cmd/termbridge/main.go agent --gateway-url {{gateway_url}} --device-name {{device_name}}

# --- M6 Gateway verification ---

# Start frontend on localhost:9011, proxying /api and WS to Gateway 127.0.0.1:8080
m6-frontend gateway_url="http://127.0.0.1:8080":
    cd web && TERMBRIDGE_WEB_BACKEND={{gateway_url}} yarn dev

# Start Agent for M6 Gateway verification
m6-agent gateway_url="http://127.0.0.1:8080" device_name="local-dev":
    go run cmd/termbridge/main.go agent --gateway-url {{gateway_url}} --device-name {{device_name}}

# Start Agent with a dev-only running shell session for M6 terminal attach verification
m6-agent-with-session gateway_url="http://127.0.0.1:8080" device_name="local-dev":
    go run cmd/termbridge/main.go agent --gateway-url {{gateway_url}} --device-name {{device_name}} --seed-session-command "printf 'POMELO_M6_ATTACH_READY\\n'; exec sh"

# Run focused Gateway/Agent/tunnel tests
m6-test:
    go test ./internal/tunnel ./internal/gatewayauth ./internal/gateway ./internal/agent ./internal/cli ./internal/app

# Run focused race tests for multiplexed tunnel paths
m6-race:
    go test -race ./internal/tunnel ./internal/gateway ./internal/agent

# Run M6 automated verification checks
m6-check: m6-test web-build

# Validate M6 Pomelo PW flow
m6-pw-validate:
    if [ -f .pomelo-pw/m6-gateway-web-terminal.yaml ]; then pomelo-pw validate .pomelo-pw/m6-gateway-web-terminal.yaml; else echo ".pomelo-pw/m6-gateway-web-terminal.yaml not found; skipped"; fi

# Run M6 Pomelo PW flow against localhost:9011
m6-pw-run:
    if [ -f .pomelo-pw/m6-gateway-web-terminal.yaml ]; then pomelo-pw run .pomelo-pw/m6-gateway-web-terminal.yaml -o .pomelo-pw/output-m6 --headless -v; else echo ".pomelo-pw/m6-gateway-web-terminal.yaml not found; skipped"; fi

