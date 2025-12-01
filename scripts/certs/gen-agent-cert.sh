#!/bin/bash
#
# Generate Agent Certificate for 3x-ui Agent mTLS
#
# This script creates an agent certificate and private key signed by the CA.
# The agent uses this certificate to authenticate itself when serving the API.
#
# Usage:
#   ./gen-agent-cert.sh <agent_id> [output_dir] [ca_dir] [hostname]
#
# Arguments:
#   agent_id   - Unique agent identifier (e.g., agent-01, prod-server-1)
#   output_dir - Directory to store agent cert files (default: ./certs)
#   ca_dir     - Directory containing CA files (default: ./certs)
#   hostname   - Agent hostname/IP for SAN (optional, e.g., agent.example.com or 192.168.1.10)
#
# Outputs:
#   agent-<id>.key - Agent private key
#   agent-<id>.crt - Agent certificate signed by CA
#   agent-<id>.csr - Certificate signing request (can be deleted after)
#

set -e

# Configuration
AGENT_ID="${1}"
OUTPUT_DIR="${2:-./certs}"
CA_DIR="${3:-./certs}"
HOSTNAME="${4:-}"
CA_KEY="${CA_DIR}/ca.key"
CA_CERT="${CA_DIR}/ca.crt"
DAYS_VALID=365  # 1 year (rotate yearly for better security)

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=== 3x-ui Agent Certificate Generator ==="
echo ""

# Validate agent ID
if [ -z "${AGENT_ID}" ]; then
    echo -e "${RED}Error: Agent ID is required${NC}"
    echo "Usage: $0 <agent_id> [output_dir] [ca_dir] [hostname]"
    echo "Example: $0 agent-01"
    echo "Example: $0 prod-server-1 ./certs ./certs agent.example.com"
    exit 1
fi

# Check if openssl is available
if ! command -v openssl &> /dev/null; then
    echo -e "${RED}Error: openssl is not installed${NC}"
    exit 1
fi

# Create output directory
mkdir -p "${OUTPUT_DIR}"

# Check if CA exists
if [ ! -f "${CA_KEY}" ] || [ ! -f "${CA_CERT}" ]; then
    echo -e "${RED}Error: CA files not found in ${CA_DIR}${NC}"
    echo "Please generate CA first: ./gen-ca.sh"
    exit 1
fi

# Output files
AGENT_KEY="${OUTPUT_DIR}/agent-${AGENT_ID}.key"
AGENT_CSR="${OUTPUT_DIR}/agent-${AGENT_ID}.csr"
AGENT_CERT="${OUTPUT_DIR}/agent-${AGENT_ID}.crt"
AGENT_EXT="${OUTPUT_DIR}/agent-${AGENT_ID}.ext"

# Check if agent cert already exists
if [ -f "${AGENT_KEY}" ] || [ -f "${AGENT_CERT}" ]; then
    echo -e "${YELLOW}Warning: Agent certificate already exists for ${AGENT_ID}${NC}"
    read -p "Overwrite? [y/N]: " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Aborted."
        exit 1
    fi
    echo ""
fi

# Generate agent private key
echo "Generating agent private key for ${AGENT_ID}..."
openssl genrsa -out "${AGENT_KEY}" 2048 2>/dev/null
chmod 600 "${AGENT_KEY}"

# Generate certificate signing request (CSR)
echo "Generating certificate signing request..."
openssl req -new -key "${AGENT_KEY}" -out "${AGENT_CSR}" \
    -subj "/C=US/ST=State/L=City/O=3x-ui/OU=Agents/CN=agent-${AGENT_ID}" \
    2>/dev/null

# Create extension file for SAN (Subject Alternative Name)
cat > "${AGENT_EXT}" <<EOF
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid,issuer
EOF

# Add SAN if hostname is provided
if [ -n "${HOSTNAME}" ]; then
    # Detect if hostname is IP or domain
    if [[ "${HOSTNAME}" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        echo "subjectAltName = IP:${HOSTNAME}" >> "${AGENT_EXT}"
        echo "  Using IP SAN: ${HOSTNAME}"
    else
        echo "subjectAltName = DNS:${HOSTNAME}" >> "${AGENT_EXT}"
        echo "  Using DNS SAN: ${HOSTNAME}"
    fi
fi

# Sign the certificate with CA
echo "Signing certificate with CA (valid for ${DAYS_VALID} days)..."
openssl x509 -req -in "${AGENT_CSR}" \
    -CA "${CA_CERT}" -CAkey "${CA_KEY}" -CAcreateserial \
    -out "${AGENT_CERT}" -days ${DAYS_VALID} \
    -extfile "${AGENT_EXT}" \
    2>/dev/null

chmod 644 "${AGENT_CERT}"

# Clean up
rm -f "${AGENT_CSR}" "${AGENT_EXT}"

echo ""
echo -e "${GREEN}=== Agent Certificate Generation Complete ===${NC}"
echo ""
echo "Generated files:"
echo "  Agent Private Key: ${AGENT_KEY}"
echo "  Agent Certificate: ${AGENT_CERT}"
echo ""
echo -e "${YELLOW}Deployment Instructions:${NC}"
echo "  1. Copy these files to the agent server:"
echo "     - ${AGENT_KEY} -> /etc/x-ui-agent/certs/agent.key"
echo "     - ${AGENT_CERT} -> /etc/x-ui-agent/certs/agent.crt"
echo "     - ${CA_CERT} -> /etc/x-ui-agent/certs/ca.crt"
echo ""
echo "  2. Set environment variables on agent:"
echo "     export AGENT_AUTH_TYPE=mtls"
echo "     export AGENT_CERT_FILE=/etc/x-ui-agent/certs/agent.crt"
echo "     export AGENT_KEY_FILE=/etc/x-ui-agent/certs/agent.key"
echo "     export AGENT_CA_FILE=/etc/x-ui-agent/certs/ca.crt"
echo ""
echo "  3. Ensure secure permissions:"
echo "     chmod 600 /etc/x-ui-agent/certs/agent.key"
echo "     chmod 644 /etc/x-ui-agent/certs/agent.crt"
echo "     chmod 644 /etc/x-ui-agent/certs/ca.crt"
echo ""

# Display certificate info
echo "Certificate Details:"
openssl x509 -in "${AGENT_CERT}" -noout -subject -issuer -dates
echo ""
