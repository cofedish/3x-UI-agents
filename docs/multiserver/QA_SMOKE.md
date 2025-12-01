# Multi-Server mTLS Smoke Test Guide

## Purpose

This document describes how to run end-to-end smoke tests for the 3x-ui multi-server architecture with mTLS authentication. The smoke test validates that all core functionality works correctly before deploying to production.

**What is tested:**
- ✅ mTLS enforcement on agent (certificate validation)
- ✅ Agent API endpoints accessibility
- ✅ Controller-agent integration via RemoteConnector
- ✅ Server management (create, list, health check)
- ✅ Inbound/client CRUD operations
- ✅ Xray control (start, stop, restart)
- ✅ Log reading and geofile updates

---

## Prerequisites

### System Requirements

**Required Software:**
- `bash` (4.0+)
- `curl` (with TLS 1.3 support)
- `jq` (JSON processor)
- `openssl` (1.1.1+)

**Install on Debian/Ubuntu:**
```bash
sudo apt-get update
sudo apt-get install -y curl jq openssl bash
```

**Install on macOS:**
```bash
brew install curl jq openssl bash
```

### Build Requirements

**For building x-ui:**
- Go 1.21+ installed
- Git

---

## Test Environment Setup

### Option 1: Local Testing (Recommended for Development)

#### Step 1: Generate Test Certificates

```bash
cd scripts/certs
./gen-test-fixture.sh

# This creates:
# test/fixtures/mtls/
#   ├── ca/ca.{key,crt}
#   ├── controller/controller.{key,crt,ca.crt}
#   ├── agent1/agent.{key,crt,ca.crt}
#   └── agent2/agent.{key,crt,ca.crt}
```

**Verify certificates:**
```bash
# Check CA
openssl x509 -in test/fixtures/mtls/ca/ca.crt -text -noout | grep -A2 Subject

# Verify controller cert
openssl verify -CAfile test/fixtures/mtls/ca/ca.crt \
    test/fixtures/mtls/controller/controller.crt

# Verify agent1 cert
openssl verify -CAfile test/fixtures/mtls/ca/ca.crt \
    test/fixtures/mtls/agent1/agent.crt
```

#### Step 2: Build Application

```bash
# Build x-ui binary
cd ../..
go build -o x-ui main.go

# Verify binary
./x-ui version
```

#### Step 3: Start Agent (Terminal 1)

```bash
# Export configuration
export AGENT_SERVER_ID=test-agent-1
export AGENT_SERVER_NAME="Test Agent 1"
export AGENT_LISTEN_ADDR=127.0.0.1:2054
export AGENT_AUTH_TYPE=mtls
export AGENT_CERT_FILE=./test/fixtures/mtls/agent1/agent.crt
export AGENT_KEY_FILE=./test/fixtures/mtls/agent1/agent.key
export AGENT_CA_FILE=./test/fixtures/mtls/agent1/ca.crt
export AGENT_LOG_LEVEL=debug

# Start agent
./x-ui agent
```

**Expected output:**
```
=== Starting 3x-ui Agent ===
Agent ID: test-agent-1
Listen Address: 127.0.0.1:2054
Auth Type: mtls
Starting agent API server...
Starting mTLS server (TLS 1.3 + client certificate required)...
```

#### Step 4: Start Controller (Terminal 2) - Optional

```bash
# Export configuration
export CONTROLLER_MODE=true
export DB_PATH=./test/test-controller.db

# Start controller
./x-ui run
```

#### Step 5: Run Smoke Tests (Terminal 3)

```bash
# Set test environment
export AGENT1_URL=https://127.0.0.1:2054
export TEST_CERTS_DIR=./test/fixtures/mtls

# Run smoke tests
./scripts/test/multiserver-smoke.sh
```

---

### Option 2: Docker Testing (Recommended for CI/CD)

**Coming soon** - Docker Compose setup for automated testing.

---

## Running Smoke Tests

### Basic Test Run

```bash
# Run with defaults
./scripts/test/multiserver-smoke.sh
```

### Advanced Options

```bash
# Test with custom agent URL
export AGENT1_URL=https://agent.example.com:2054
./scripts/test/multiserver-smoke.sh

# Test with two agents
export AGENT1_URL=https://agent1.example.com:2054
export AGENT2_URL=https://agent2.example.com:2054
./scripts/test/multiserver-smoke.sh

# Keep artifacts for debugging
export SKIP_CLEANUP=1
./scripts/test/multiserver-smoke.sh

# Use custom certificate directory
export TEST_CERTS_DIR=/path/to/certs
./scripts/test/multiserver-smoke.sh
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `AGENT1_URL` | `https://localhost:2054` | Agent 1 API endpoint |
| `AGENT2_URL` | _(empty)_ | Agent 2 API endpoint (optional) |
| `CONTROLLER_URL` | `https://localhost:2053` | Controller API endpoint |
| `TEST_CERTS_DIR` | `test/fixtures/mtls` | Certificate directory |
| `SKIP_CLEANUP` | `0` | Set to `1` to keep test artifacts |

---

## Understanding Test Results

### Successful Test Output

```
╔════════════════════════════════════════════════════════════════╗
║        3x-ui Multi-Server mTLS Smoke Test Suite                ║
╚════════════════════════════════════════════════════════════════╝

▶ Checking prerequisites...
[PASS] All required commands available
[PASS] All required certificates found

▶ Test 4.1: mTLS Enforcement on Agent
[INFO] Test 4.1.A: Request without client certificate
[PASS] Agent correctly rejected request without client certificate
[INFO] Test 4.1.C: Request with correct controller certificate
[PASS] Agent /health endpoint accessible with mTLS
[PASS] Agent /info endpoint accessible with mTLS
[PASS] TLS 1.3 is being used

▶ Test 4.2: Agent API Endpoints
[PASS] Endpoint /api/v1/health returned 200 with valid JSON
[PASS] Endpoint /api/v1/info returned 200 with valid JSON
...

╔════════════════════════════════════════════════════════════════╗
║                      Test Summary                              ║
╚════════════════════════════════════════════════════════════════╝

Total tests:  15
Passed:       15
Failed:       0

✓ All tests passed!

Artifacts saved to: logs/smoke/artifacts
```

### Failed Test Output

```
[FAIL] Agent accepted request without client certificate (SECURITY ISSUE!)
```

**Action:** This indicates a critical security issue. Agent must be configured to require client certificates.

---

## Test Artifacts

After running tests, artifacts are saved to `logs/smoke/artifacts/`:

```
logs/smoke/artifacts/
├── test-4.1.A-output.txt          # Request without cert (should fail)
├── test-4.1.C-health.json         # /health response
├── test-4.1.C-info.json           # /info response
├── test-4.1.D-tls-version.txt     # TLS version info
├── test-4.2-api-v1-health.json    # Health endpoint test
├── test-4.2-api-v1-info.json      # Info endpoint test
└── ...
```

**Inspecting artifacts:**
```bash
# View health check response
cat logs/smoke/artifacts/test-4.1.C-health.json | jq '.'

# View server info
cat logs/smoke/artifacts/test-4.1.C-info.json | jq '.data'

# Check TLS version
cat logs/smoke/artifacts/test-4.1.D-tls-version.txt
```

---

## Manual Testing

### Manual mTLS Test

```bash
# Test without client certificate (should fail)
curl -vk --cacert test/fixtures/mtls/ca/ca.crt \
    https://localhost:2054/api/v1/health

# Test with client certificate (should succeed)
curl -vk \
    --cacert test/fixtures/mtls/ca/ca.crt \
    --cert test/fixtures/mtls/controller/controller.crt \
    --key test/fixtures/mtls/controller/controller.key \
    https://localhost:2054/api/v1/health | jq '.'
```

### Manual Endpoint Testing

```bash
# Export cert paths for convenience
export CA_CERT=test/fixtures/mtls/ca/ca.crt
export CLIENT_CERT=test/fixtures/mtls/controller/controller.crt
export CLIENT_KEY=test/fixtures/mtls/controller/controller.key

# Test /info endpoint
curl -sk --cacert $CA_CERT --cert $CLIENT_CERT --key $CLIENT_KEY \
    https://localhost:2054/api/v1/info | jq '.'

# Test /inbounds endpoint
curl -sk --cacert $CA_CERT --cert $CLIENT_CERT --key $CLIENT_KEY \
    https://localhost:2054/api/v1/inbounds | jq '.'

# Test /system/stats endpoint
curl -sk --cacert $CA_CERT --cert $CLIENT_CERT --key $CLIENT_KEY \
    https://localhost:2054/api/v1/system/stats | jq '.'
```

---

## Troubleshooting

### Issue: "Missing required commands"

**Error:**
```
[FAIL] Missing required commands: jq
```

**Solution:**
```bash
sudo apt-get install jq
```

---

### Issue: "Missing certificates"

**Error:**
```
[FAIL] Missing certificates: test/fixtures/mtls/ca/ca.crt
```

**Solution:**
```bash
cd scripts/certs
./gen-test-fixture.sh
```

---

### Issue: "Connection refused"

**Error:**
```
curl: (7) Failed to connect to localhost port 2054: Connection refused
```

**Solution:**
- Verify agent is running: `ps aux | grep "x-ui agent"`
- Check agent logs for startup errors
- Verify port is correct: `netstat -tlnp | grep 2054`

---

### Issue: "TLS handshake failed"

**Error:**
```
curl: (35) error:14094410:SSL routines:ssl3_read_bytes:sslv3 alert handshake failure
```

**Possible causes:**
1. **Agent not requiring client cert** - Check agent TLS config
2. **Wrong client certificate** - Verify you're using controller cert, not agent cert
3. **Certificate not signed by CA** - Regenerate certificates

**Debug:**
```bash
# Test TLS handshake
openssl s_client -connect localhost:2054 \
    -CAfile test/fixtures/mtls/ca/ca.crt \
    -cert test/fixtures/mtls/controller/controller.crt \
    -key test/fixtures/mtls/controller/controller.key \
    -tls1_3
```

---

### Issue: "Agent accepted request without client certificate"

**Error:**
```
[FAIL] Agent accepted request without client certificate (SECURITY ISSUE!)
```

**This is a CRITICAL security issue!**

**Solution:**
1. Check agent configuration:
   ```bash
   echo $AGENT_AUTH_TYPE  # Should be "mtls"
   echo $AGENT_CA_FILE     # Should point to ca.crt
   ```

2. Verify agent code in `agent/api/router.go`:
   ```go
   tlsConfig := &tls.Config{
       ClientAuth: tls.RequireAndVerifyClientCert,  // MUST be present
       ClientCAs:  caCertPool,                      // MUST be set
   }
   ```

3. Check agent logs for TLS configuration errors

---

### Issue: "JSON parsing failed"

**Error:**
```
parse error: Invalid numeric literal at line 1, column 10
```

**Solution:**
- Agent returned non-JSON response (HTML error page?)
- Check raw response: `cat logs/smoke/artifacts/test-*.json`
- Verify endpoint exists and is accessible

---

## Test Checklist (Manual Verification)

Use this checklist when running tests manually:

### ✅ Prerequisites
- [ ] Go 1.21+ installed
- [ ] curl, jq, openssl installed
- [ ] Test certificates generated
- [ ] x-ui binary built

### ✅ mTLS Security
- [ ] Agent rejects requests without client certificate
- [ ] Agent rejects requests with invalid client certificate
- [ ] Agent accepts requests with valid controller certificate
- [ ] TLS 1.3 is being used
- [ ] No `InsecureSkipVerify` in RemoteConnector code

### ✅ Agent Endpoints
- [ ] `/api/v1/health` returns 200 + valid JSON
- [ ] `/api/v1/info` returns server information
- [ ] `/api/v1/inbounds` returns inbound list (may be empty)
- [ ] `/api/v1/system/stats` returns system statistics
- [ ] `/api/v1/xray/version` returns xray version

### ✅ Controller Integration (if running)
- [ ] Can create server with mTLS auth type
- [ ] Server list shows newly created server
- [ ] Health check shows server online
- [ ] Private keys NOT stored in database

### ✅ E2E Operations (if running)
- [ ] Create inbound on remote agent
- [ ] List inbounds shows new inbound
- [ ] Add client to inbound
- [ ] Update client settings
- [ ] Delete client
- [ ] Restart xray
- [ ] Fetch logs (with limits)
- [ ] Delete inbound

---

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Multi-Server Smoke Test

on: [push, pull_request]

jobs:
  smoke-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Install dependencies
        run: |
          sudo apt-get update
          sudo apt-get install -y curl jq openssl

      - name: Generate test certificates
        run: |
          cd scripts/certs
          ./gen-test-fixture.sh

      - name: Build x-ui
        run: go build -o x-ui main.go

      - name: Start agent
        run: |
          export AGENT_AUTH_TYPE=mtls
          export AGENT_CERT_FILE=./test/fixtures/mtls/agent1/agent.crt
          export AGENT_KEY_FILE=./test/fixtures/mtls/agent1/agent.key
          export AGENT_CA_FILE=./test/fixtures/mtls/agent1/ca.crt
          ./x-ui agent &
          sleep 3

      - name: Run smoke tests
        run: ./scripts/test/multiserver-smoke.sh

      - name: Upload artifacts
        if: always()
        uses: actions/upload-artifact@v3
        with:
          name: smoke-test-artifacts
          path: logs/smoke/artifacts/
```

---

## Next Steps

After smoke tests pass:

1. **Production Deployment**
   - Follow [DEPLOYMENT.md](./DEPLOYMENT.md)
   - Use production certificates (not test fixtures)
   - Enable firewall rules

2. **Monitoring**
   - Set up certificate expiry monitoring
   - Configure health check alerts
   - Monitor authentication failures

3. **Regular Testing**
   - Run smoke tests after each deployment
   - Test certificate rotation procedure
   - Verify backup/restore functionality

---

## Support

**Issues with smoke tests?**

1. Check [Troubleshooting](#troubleshooting) section
2. Review test artifacts in `logs/smoke/artifacts/`
3. Enable debug logging: `export AGENT_LOG_LEVEL=debug`
4. Open an issue with:
   - Test output
   - Agent/controller logs
   - Certificate verification results

**Questions?**

- See [AGENT.md](./AGENT.md) for agent configuration
- See [ARCHITECTURE.md](./ARCHITECTURE.md) for security model
- See [DEPLOYMENT.md](./DEPLOYMENT.md) for production deployment
