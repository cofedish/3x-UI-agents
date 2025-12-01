# Build and Test Guide

## Prerequisites

### Required Software

1. **Go 1.21+**
   - Download from: https://golang.org/dl/
   - Verify: `go version`

2. **Git**
   - Already installed (you have the repository)

3. **Testing Tools**
   - curl
   - jq
   - openssl

## Building from Source

### Step 1: Clone Repository (Already Done)

```bash
git clone https://github.com/cofedish/3xui-agents.git
cd 3xui-agents/3x-ui
git checkout feature/multiserver-controller-agent
```

### Step 2: Build Binary

```bash
# Build for current platform
go build -o x-ui main.go

# Verify binary
./x-ui --help
```

**Expected output:**
```
Usage: x-ui [command]

Commands:
    run            run web panel (default)
    agent          run as agent (for remote VPN servers)
    ...
```

### Step 3: Build for Linux (Cross-Compile from Windows)

```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o x-ui-linux-amd64 main.go

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -o x-ui-linux-arm64 main.go
```

## Testing Without Building

If Go is not available, you can still test the certificates and configuration:

### Certificate Validation Test

```bash
cd scripts/certs

# Generate test certificates
./gen-test-fixture.sh

# Verify certificates
cd ../../test/fixtures/mtls
openssl verify -CAfile ca/ca.crt controller/controller.crt
openssl verify -CAfile ca/ca.crt agent1/agent.crt
openssl verify -CAfile ca/ca.crt agent2/agent.crt

# Check certificate details
openssl x509 -in controller/controller.crt -text -noout | grep -A2 "Extended Key Usage"
openssl x509 -in agent1/agent.crt -text -noout | grep -A2 "Subject Alternative Name"
```

**Expected:**
- Controller cert should have `TLS Web Client Authentication`
- Agent cert should have `TLS Web Server Authentication`
- Agent cert should have SAN: `IP:127.0.0.1, DNS:localhost`

## Running Smoke Tests

### With Built Binary

```bash
# Terminal 1: Start agent
export AGENT_SERVER_ID=test-agent-1
export AGENT_LISTEN_ADDR=127.0.0.1:2054
export AGENT_AUTH_TYPE=mtls
export AGENT_CERT_FILE=./test/fixtures/mtls/agent1/agent.crt
export AGENT_KEY_FILE=./test/fixtures/mtls/agent1/agent.key
export AGENT_CA_FILE=./test/fixtures/mtls/agent1/ca.crt
./x-ui agent

# Terminal 2: Run smoke tests
export AGENT1_URL=https://127.0.0.1:2054
export TEST_CERTS_DIR=./test/fixtures/mtls
./scripts/test/multiserver-smoke.sh
```

### Manual API Testing (Without Agent Binary)

If you have a running agent instance elsewhere:

```bash
# Test mTLS enforcement
curl -vk --cacert test/fixtures/mtls/ca/ca.crt \
    https://<agent-ip>:2054/api/v1/health
# Should fail with TLS handshake error

# Test with client cert
curl -vk \
    --cacert test/fixtures/mtls/ca/ca.crt \
    --cert test/fixtures/mtls/controller/controller.crt \
    --key test/fixtures/mtls/controller/controller.key \
    https://<agent-ip>:2054/api/v1/health | jq '.'
# Should succeed
```

## Quick Start (Development)

If you have Go installed:

```bash
# 1. Generate certificates
cd scripts/certs && ./gen-test-fixture.sh && cd ../..

# 2. Build binary
go build -o x-ui main.go

# 3. Start agent in background
export AGENT_AUTH_TYPE=mtls
export AGENT_CERT_FILE=./test/fixtures/mtls/agent1/agent.crt
export AGENT_KEY_FILE=./test/fixtures/mtls/agent1/agent.key
export AGENT_CA_FILE=./test/fixtures/mtls/agent1/ca.crt
./x-ui agent > logs/agent.log 2>&1 &
AGENT_PID=$!

# 4. Wait for startup
sleep 2

# 5. Run tests
./scripts/test/multiserver-smoke.sh

# 6. Kill agent
kill $AGENT_PID
```

## Troubleshooting Build Issues

### Issue: "go: command not found"

**Solution:**
1. Install Go from https://golang.org/dl/
2. Add Go to PATH:
   ```bash
   export PATH=$PATH:/usr/local/go/bin
   ```

### Issue: "cannot find package"

**Solution:**
```bash
go mod download
go mod tidy
```

### Issue: Build errors with dependencies

**Solution:**
```bash
# Clean cache
go clean -cache -modcache

# Re-download
go mod download

# Try again
go build -o x-ui main.go
```

## Next Steps

Once you have a working binary:

1. Follow [QA_SMOKE.md](./QA_SMOKE.md) for detailed testing
2. Follow [AGENT.md](./AGENT.md) for production deployment
3. Follow [DEPLOYMENT.md](./DEPLOYMENT.md) for full setup
