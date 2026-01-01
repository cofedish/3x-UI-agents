# Authentication Guide

3X-UI supports two authentication methods for agent-to-panel communication.

## mTLS (Mutual TLS)

**Recommended for production environments.**

### Overview

- **Security**: Mutual TLS with client certificate validation
- **Protocol**: HTTPS
- **Use case**: Production deployments, secure remote servers
- **Setup complexity**: Moderate (certificates managed automatically)

### How It Works

1. Agent generates self-signed CA and certificates during installation
2. Agent runs HTTPS server with client certificate requirement
3. Panel authenticates using client certificate
4. Both sides verify each other's identity

### Certificate Structure

```
/etc/x-ui/agent/certs/
├── ca.crt          # Certificate Authority
├── ca.key          # CA private key
├── agent.crt       # Agent server certificate
├── agent.key       # Agent server private key
├── client.crt      # Client certificate (for panel)
└── client.key      # Client private key (for panel)
```

### Certificate Bundle Format

When adding an mTLS agent to the panel, paste this bundle:

```
-----BEGIN CERTIFICATE-----
[Agent Certificate Content]
-----END CERTIFICATE-----
-----BEGIN PRIVATE KEY-----
[Agent Private Key Content]
-----END PRIVATE KEY-----
-----BEGIN CERTIFICATE-----
[CA Certificate Content]
-----END CERTIFICATE-----
```

### Manual Certificate Generation

If you need to regenerate certificates:

```bash
cd /etc/x-ui/agent

# Generate CA
openssl genrsa -out certs/ca.key 4096
openssl req -new -x509 -days 365 -key certs/ca.key \
  -subj "/C=US/ST=State/L=City/O=Org/CN=3X-UI-Agent-CA" \
  -out certs/ca.crt

# Generate agent server cert
openssl genrsa -out certs/agent.key 4096
openssl req -new -key certs/agent.key \
  -subj "/C=US/ST=State/L=City/O=Org/CN=$AGENT_HOST_IP" \
  -out certs/agent.csr

# Sign with CA
openssl x509 -req -in certs/agent.csr -CA certs/ca.crt -CAkey certs/ca.key \
  -CAcreateserial -out certs/agent.crt -days 365 \
  -extfile <(printf "subjectAltName=IP:$AGENT_HOST_IP")

# Generate client cert (for panel)
openssl genrsa -out certs/client.key 4096
openssl req -new -key certs/client.key \
  -subj "/C=US/ST=State/L=City/O=Org/CN=panel-client" \
  -out certs/client.csr
openssl x509 -req -in certs/client.csr -CA certs/ca.crt -CAkey certs/ca.key \
  -CAcreateserial -out certs/client.crt -days 365
```

### Verifying Certificates

```bash
# Check certificate details
openssl x509 -in /etc/x-ui/agent/certs/agent.crt -text -noout

# Verify certificate chain
openssl verify -CAfile /etc/x-ui/agent/certs/ca.crt \
  /etc/x-ui/agent/certs/agent.crt
```

## JWT (JSON Web Tokens)

**Recommended for testing and development.**

### Overview

- **Security**: Token-based authentication
- **Protocol**: HTTP (no TLS overhead)
- **Use case**: Development, testing, quick setup, local networks
- **Setup complexity**: Simple (just copy token)

### How It Works

1. Agent generates a random JWT secret during installation
2. Panel sends this token in `Authorization: Bearer <token>` header
3. Agent validates token on each request

### JWT Token Format

The token is a simple random string:
```
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U
```

### Finding Your JWT Token

On the agent server:

```bash
# Check systemd service
sudo systemctl cat x-ui-agent | grep JWT_SECRET

# Or check config file
cat /etc/x-ui/agent/config.json | grep jwt
```

### Rotating JWT Token

To generate a new token:

1. Generate new token:
   ```bash
   NEW_TOKEN=$(openssl rand -base64 32)
   ```

2. Update agent service:
   ```bash
   sudo nano /etc/systemd/system/x-ui-agent.service
   # Change AGENT_JWT_SECRET value
   ```

3. Reload and restart:
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl restart x-ui-agent
   ```

4. Update token in panel

## Security Comparison

| Feature | mTLS | JWT |
|---------|------|-----|
| Encryption | HTTPS (TLS 1.3) | HTTP (no encryption) |
| Authentication | Mutual certificates | Bearer token |
| Setup complexity | Moderate | Simple |
| Production ready | ✅ Yes | ⚠️ Development only |
| Performance | Slightly slower (TLS handshake) | Faster |
| Certificate expiry | 365 days (renewable) | N/A |
| Token rotation | N/A | Manual |

## Best Practices

### For mTLS

1. **Use long certificate validity** (365+ days) to reduce rotation burden
2. **Store certificate backups** securely
3. **Monitor certificate expiry** dates
4. **Use unique CA per agent** for isolation

### For JWT

1. **Only use over private networks** or VPNs
2. **Never expose JWT agents to public internet**
3. **Rotate tokens periodically** (monthly recommended)
4. **Use strong random tokens** (32+ bytes)

## Migration Between Auth Types

### JWT to mTLS

1. Install new agent with mTLS on same server (different port temporarily)
2. Add new mTLS agent to panel
3. Migrate inbounds/clients to new server entry
4. Stop and remove old JWT agent

### mTLS to JWT

Similar process in reverse:

1. Install new agent with JWT
2. Add to panel
3. Migrate configuration
4. Remove old mTLS agent

## Troubleshooting

See [Troubleshooting Guide](Troubleshooting.md#certificate-issues-mtls) for authentication-related issues.
