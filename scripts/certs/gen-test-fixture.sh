#!/bin/bash
#
# Generate Test Certificates Fixture for Smoke Tests
#
# This script creates a complete set of certificates for testing
# the multi-server mTLS implementation. It's idempotent and creates
# all necessary certificates in test/fixtures/mtls/.
#
# Usage:
#   ./gen-test-fixture.sh [output_dir]
#
# Arguments:
#   output_dir - Directory to store test certificates (default: ../../test/fixtures/mtls)
#

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="${1:-${SCRIPT_DIR}/../../test/fixtures/mtls}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Generating Test Certificate Fixture ===${NC}"
echo ""
echo "Output directory: ${OUTPUT_DIR}"
echo ""

# Create directory structure
mkdir -p "${OUTPUT_DIR}"/{ca,controller,agent1,agent2}

# Check if openssl is available
if ! command -v openssl &> /dev/null; then
    echo -e "${RED}Error: openssl is not installed${NC}"
    exit 1
fi

# Generate CA
echo -e "${BLUE}[1/4] Generating CA certificate...${NC}"
CA_KEY="${OUTPUT_DIR}/ca/ca.key"
CA_CERT="${OUTPUT_DIR}/ca/ca.crt"

if [ -f "${CA_KEY}" ]; then
    echo -e "${YELLOW}CA already exists, skipping...${NC}"
else
    openssl genrsa -out "${CA_KEY}" 4096 2>/dev/null
    chmod 600 "${CA_KEY}"

    openssl req -new -x509 -days 3650 -key "${CA_KEY}" -out "${CA_CERT}" \
        -subj "/C=US/ST=Test/L=Test/O=3x-ui-test/OU=Testing/CN=test-ca" \
        2>/dev/null
    chmod 644 "${CA_CERT}"
    echo -e "${GREEN}✓ CA certificate generated${NC}"
fi

# Generate Controller certificate
echo -e "${BLUE}[2/4] Generating Controller certificate...${NC}"
CONTROLLER_KEY="${OUTPUT_DIR}/controller/controller.key"
CONTROLLER_CSR="${OUTPUT_DIR}/controller/controller.csr"
CONTROLLER_CERT="${OUTPUT_DIR}/controller/controller.crt"
CONTROLLER_EXT="${OUTPUT_DIR}/controller/controller.ext"

if [ -f "${CONTROLLER_KEY}" ]; then
    echo -e "${YELLOW}Controller cert already exists, skipping...${NC}"
else
    openssl genrsa -out "${CONTROLLER_KEY}" 2048 2>/dev/null
    chmod 600 "${CONTROLLER_KEY}"

    openssl req -new -key "${CONTROLLER_KEY}" -out "${CONTROLLER_CSR}" \
        -subj "/C=US/ST=Test/L=Test/O=3x-ui-test/OU=Controller/CN=test-controller" \
        2>/dev/null

    cat > "${CONTROLLER_EXT}" <<EOF
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid,issuer
EOF

    openssl x509 -req -in "${CONTROLLER_CSR}" \
        -CA "${CA_CERT}" -CAkey "${CA_KEY}" -CAcreateserial \
        -out "${CONTROLLER_CERT}" -days 365 \
        -extfile "${CONTROLLER_EXT}" \
        2>/dev/null

    chmod 644 "${CONTROLLER_CERT}"
    rm -f "${CONTROLLER_CSR}" "${CONTROLLER_EXT}"
    echo -e "${GREEN}✓ Controller certificate generated${NC}"
fi

# Copy CA to controller dir
cp "${CA_CERT}" "${OUTPUT_DIR}/controller/ca.crt"

# Generate Agent1 certificate
echo -e "${BLUE}[3/4] Generating Agent1 certificate...${NC}"
AGENT1_KEY="${OUTPUT_DIR}/agent1/agent.key"
AGENT1_CSR="${OUTPUT_DIR}/agent1/agent.csr"
AGENT1_CERT="${OUTPUT_DIR}/agent1/agent.crt"
AGENT1_EXT="${OUTPUT_DIR}/agent1/agent.ext"

if [ -f "${AGENT1_KEY}" ]; then
    echo -e "${YELLOW}Agent1 cert already exists, skipping...${NC}"
else
    openssl genrsa -out "${AGENT1_KEY}" 2048 2>/dev/null
    chmod 600 "${AGENT1_KEY}"

    openssl req -new -key "${AGENT1_KEY}" -out "${AGENT1_CSR}" \
        -subj "/C=US/ST=Test/L=Test/O=3x-ui-test/OU=Agents/CN=test-agent-1" \
        2>/dev/null

    cat > "${AGENT1_EXT}" <<EOF
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid,issuer
subjectAltName = IP:127.0.0.1,DNS:localhost
EOF

    openssl x509 -req -in "${AGENT1_CSR}" \
        -CA "${CA_CERT}" -CAkey "${CA_KEY}" -CAcreateserial \
        -out "${AGENT1_CERT}" -days 365 \
        -extfile "${AGENT1_EXT}" \
        2>/dev/null

    chmod 644 "${AGENT1_CERT}"
    rm -f "${AGENT1_CSR}" "${AGENT1_EXT}"
    echo -e "${GREEN}✓ Agent1 certificate generated${NC}"
fi

# Copy CA to agent1 dir
cp "${CA_CERT}" "${OUTPUT_DIR}/agent1/ca.crt"

# Generate Agent2 certificate
echo -e "${BLUE}[4/4] Generating Agent2 certificate...${NC}"
AGENT2_KEY="${OUTPUT_DIR}/agent2/agent.key"
AGENT2_CSR="${OUTPUT_DIR}/agent2/agent.csr"
AGENT2_CERT="${OUTPUT_DIR}/agent2/agent.crt"
AGENT2_EXT="${OUTPUT_DIR}/agent2/agent.ext"

if [ -f "${AGENT2_KEY}" ]; then
    echo -e "${YELLOW}Agent2 cert already exists, skipping...${NC}"
else
    openssl genrsa -out "${AGENT2_KEY}" 2048 2>/dev/null
    chmod 600 "${AGENT2_KEY}"

    openssl req -new -key "${AGENT2_KEY}" -out "${AGENT2_CSR}" \
        -subj "/C=US/ST=Test/L=Test/O=3x-ui-test/OU=Agents/CN=test-agent-2" \
        2>/dev/null

    cat > "${AGENT2_EXT}" <<EOF
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid,issuer
subjectAltName = IP:127.0.0.2,DNS:localhost
EOF

    openssl x509 -req -in "${AGENT2_CSR}" \
        -CA "${CA_CERT}" -CAkey "${CA_KEY}" -CAcreateserial \
        -out "${AGENT2_CERT}" -days 365 \
        -extfile "${AGENT2_EXT}" \
        2>/dev/null

    chmod 644 "${AGENT2_CERT}"
    rm -f "${AGENT2_CSR}" "${AGENT2_EXT}"
    echo -e "${GREEN}✓ Agent2 certificate generated${NC}"
fi

# Copy CA to agent2 dir
cp "${CA_CERT}" "${OUTPUT_DIR}/agent2/ca.crt"

# Create summary
echo ""
echo -e "${GREEN}=== Certificate Fixture Generated Successfully ===${NC}"
echo ""
echo "Certificate structure:"
echo "  ${OUTPUT_DIR}/"
echo "    ├── ca/"
echo "    │   ├── ca.key  (KEEP SECURE!)"
echo "    │   └── ca.crt"
echo "    ├── controller/"
echo "    │   ├── controller.key"
echo "    │   ├── controller.crt"
echo "    │   └── ca.crt"
echo "    ├── agent1/"
echo "    │   ├── agent.key"
echo "    │   ├── agent.crt"
echo "    │   └── ca.crt"
echo "    └── agent2/"
echo "        ├── agent.key"
echo "        ├── agent.crt"
echo "        └── ca.crt"
echo ""
echo "These certificates are for TESTING ONLY!"
echo "Do not use in production."
echo ""

# Verify certificates
echo -e "${BLUE}Verifying certificates...${NC}"
if openssl verify -CAfile "${CA_CERT}" "${CONTROLLER_CERT}" &>/dev/null; then
    echo -e "${GREEN}✓ Controller certificate valid${NC}"
fi
if openssl verify -CAfile "${CA_CERT}" "${AGENT1_CERT}" &>/dev/null; then
    echo -e "${GREEN}✓ Agent1 certificate valid${NC}"
fi
if openssl verify -CAfile "${CA_CERT}" "${AGENT2_CERT}" &>/dev/null; then
    echo -e "${GREEN}✓ Agent2 certificate valid${NC}"
fi

echo ""
echo "Next steps:"
echo "  1. Use these certificates with smoke test script"
echo "  2. Export paths:"
echo "     export TEST_CERTS_DIR=${OUTPUT_DIR}"
echo ""
