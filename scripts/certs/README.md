# mTLS Certificate Generation Scripts

These scripts generate certificates for mutual TLS (mTLS) authentication between the 3x-ui controller and remote agents.

## Quick Start

### 1. Generate CA (Certificate Authority)

```bash
cd scripts/certs
./gen-ca.sh
```

This creates:
- `ca.key` - CA private key (**KEEP SECURE!**)
- `ca.crt` - CA certificate (distribute to all agents and controller)

### 2. Generate Agent Certificate

For each agent server, generate a unique certificate:

```bash
./gen-agent-cert.sh agent-01
./gen-agent-cert.sh prod-server-ny
./gen-agent-cert.sh eu-gateway-1 ./certs ./certs agent.example.com
```

This creates:
- `agent-<id>.key` - Agent private key
- `agent-<id>.crt` - Agent certificate

### 3. Generate Controller Certificate

```bash
./gen-controller-cert.sh
```

This creates:
- `controller.key` - Controller private key
- `controller.crt` - Controller certificate

## Deployment

### Agent Deployment

1. Copy certificates to agent server:
```bash
scp ca.crt agent-01.key agent-01.crt user@agent-server:/tmp/
```

2. On agent server, move to proper location:
```bash
sudo mkdir -p /etc/x-ui-agent/certs
sudo mv /tmp/agent-01.key /etc/x-ui-agent/certs/agent.key
sudo mv /tmp/agent-01.crt /etc/x-ui-agent/certs/agent.crt
sudo mv /tmp/ca.crt /etc/x-ui-agent/certs/ca.crt
sudo chmod 600 /etc/x-ui-agent/certs/agent.key
sudo chmod 644 /etc/x-ui-agent/certs/*.crt
```

3. Set environment variables (or add to systemd service):
```bash
export AGENT_AUTH_TYPE=mtls
export AGENT_CERT_FILE=/etc/x-ui-agent/certs/agent.crt
export AGENT_KEY_FILE=/etc/x-ui-agent/certs/agent.key
export AGENT_CA_FILE=/etc/x-ui-agent/certs/ca.crt
```

### Controller Deployment

1. Copy certificates to controller server:
```bash
sudo mkdir -p /etc/x-ui/certs
sudo cp controller.key controller.crt ca.crt /etc/x-ui/certs/
sudo chmod 600 /etc/x-ui/certs/controller.key
sudo chmod 644 /etc/x-ui/certs/*.crt
```

2. In 3x-ui UI, when adding a remote server:
   - **Auth Type**: mTLS
   - **Auth Data** (JSON):
   ```json
   {
     "certFile": "/etc/x-ui/certs/controller.crt",
     "keyFile": "/etc/x-ui/certs/controller.key",
     "caFile": "/etc/x-ui/certs/ca.crt"
   }
   ```

## Security Best Practices

### Certificate Lifetimes
- **CA Certificate**: 10 years (long-lived, rotate rarely)
- **Agent/Controller Certificates**: 1 year (rotate annually)

### Rotation Procedure

When certificates are about to expire:

1. **Generate new certificates** (keep same CA if still valid):
```bash
./gen-agent-cert.sh agent-01
./gen-controller-cert.sh
```

2. **Deploy new certificates** to servers (no downtime required - just replace files and restart services)

3. **Verify** new certificates are working

4. **Remove old certificates**

### CA Rotation (Rare)

If CA is compromised or expiring:

1. **Generate new CA**:
```bash
./gen-ca.sh ./certs-new new-ca-name
```

2. **Generate ALL new certificates** with new CA

3. **Deploy to ALL agents and controller** (requires coordination)

## Troubleshooting

### Certificate Verification Errors

Check certificate chain:
```bash
openssl verify -CAfile ca.crt agent-01.crt
openssl verify -CAfile ca.crt controller.crt
```

### View Certificate Details
```bash
openssl x509 -in agent-01.crt -text -noout
```

### Test mTLS Connection
```bash
curl --cert controller.crt --key controller.key --cacert ca.crt \
  https://agent-server:2054/api/v1/health
```

### Common Issues

1. **"certificate signed by unknown authority"**
   - Ensure `ca.crt` is the same on both controller and agent
   - Verify `AGENT_CA_FILE` path is correct

2. **"tls: bad certificate"**
   - Check that controller is using correct client cert
   - Verify cert/key pair match: `openssl x509 -noout -modulus -in cert.crt | md5sum` should match `openssl rsa -noout -modulus -in key.key | md5sum`

3. **"x509: certificate has expired"**
   - Regenerate certificates using the scripts above

## Architecture

```
┌─────────────┐                    ┌─────────────┐
│             │  mTLS Connection   │             │
│ Controller  │◄──────────────────►│   Agent     │
│             │                    │             │
└─────────────┘                    └─────────────┘
      │                                  │
      │ Uses:                            │ Uses:
      │ - controller.crt (client)        │ - agent.crt (server)
      │ - controller.key                 │ - agent.key
      │ - ca.crt (to verify agent)       │ - ca.crt (to verify controller)
      │                                  │
      └──────────────┬───────────────────┘
                     │
                     │ Both signed by:
                     ▼
              ┌─────────────┐
              │   CA (Root) │
              │   ca.crt    │
              │   ca.key    │
              └─────────────┘
```

## Files Overview

| File | Purpose | Distribute To | Security Level |
|------|---------|---------------|----------------|
| `ca.key` | CA private key | **NOWHERE** (keep offline) | **CRITICAL** |
| `ca.crt` | CA certificate | All agents + controller | Public |
| `agent-*.key` | Agent private key | Specific agent only | **SECRET** |
| `agent-*.crt` | Agent certificate | Specific agent only | Public |
| `controller.key` | Controller private key | Controller only | **SECRET** |
| `controller.crt` | Controller certificate | Controller only | Public |

## Advanced: Custom SAN (Subject Alternative Name)

For agents with specific hostnames or IPs:

```bash
# With DNS name
./gen-agent-cert.sh prod-01 ./certs ./certs agent.example.com

# With IP address
./gen-agent-cert.sh prod-02 ./certs ./certs 192.168.1.100
```

This allows the controller to verify the certificate against the hostname/IP.
