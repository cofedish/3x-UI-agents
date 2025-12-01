#!/bin/bash
#
# Generate Controller Certificate for 3x-ui Controller mTLS
#
# This script creates a controller client certificate and private key signed by the CA.
# The controller uses this certificate to authenticate when making requests to agents.
#
# Usage:
#   ./gen-controller-cert.sh [output_dir] [ca_dir]
#
# Arguments:
#   output_dir - Directory to store controller cert files (default: ./certs)
#   ca_dir     - Directory containing CA files (default: ./certs)
#
# Outputs:
#   controller.key - Controller private key
#   controller.crt - Controller certificate signed by CA
#   controller.csr - Certificate signing request (can be deleted after)
#

set -e

# Configuration
OUTPUT_DIR="${1:-./certs}"
CA_DIR="${2:-./certs}"
CA_KEY="${CA_DIR}/ca.key"
CA_CERT="${CA_DIR}/ca.crt"
DAYS_VALID=365  # 1 year

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=== 3x-ui Controller Certificate Generator ==="
echo ""

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
CONTROLLER_KEY="${OUTPUT_DIR}/controller.key"
CONTROLLER_CSR="${OUTPUT_DIR}/controller.csr"
CONTROLLER_CERT="${OUTPUT_DIR}/controller.crt"
CONTROLLER_EXT="${OUTPUT_DIR}/controller.ext"

# Check if controller cert already exists
if [ -f "${CONTROLLER_KEY}" ] || [ -f "${CONTROLLER_CERT}" ]; then
    echo -e "${YELLOW}Warning: Controller certificate already exists${NC}"
    read -p "Overwrite? [y/N]: " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Aborted."
        exit 1
    fi
    echo ""
fi

# Generate controller private key
echo "Generating controller private key..."
openssl genrsa -out "${CONTROLLER_KEY}" 2048 2>/dev/null
chmod 600 "${CONTROLLER_KEY}"

# Generate certificate signing request (CSR)
echo "Generating certificate signing request..."
openssl req -new -key "${CONTROLLER_KEY}" -out "${CONTROLLER_CSR}" \
    -subj "/C=US/ST=State/L=City/O=3x-ui/OU=Controller/CN=3x-ui-controller" \
    2>/dev/null

# Create extension file (client auth)
cat > "${CONTROLLER_EXT}" <<EOF
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid,issuer
EOF

# Sign the certificate with CA
echo "Signing certificate with CA (valid for ${DAYS_VALID} days)..."
openssl x509 -req -in "${CONTROLLER_CSR}" \
    -CA "${CA_CERT}" -CAkey "${CA_KEY}" -CAcreateserial \
    -out "${CONTROLLER_CERT}" -days ${DAYS_VALID} \
    -extfile "${CONTROLLER_EXT}" \
    2>/dev/null

chmod 644 "${CONTROLLER_CERT}"

# Clean up
rm -f "${CONTROLLER_CSR}" "${CONTROLLER_EXT}"

echo ""
echo -e "${GREEN}=== Controller Certificate Generation Complete ===${NC}"
echo ""
echo "Generated files:"
echo "  Controller Private Key: ${CONTROLLER_KEY}"
echo "  Controller Certificate: ${CONTROLLER_CERT}"
echo ""
echo -e "${YELLOW}Deployment Instructions:${NC}"
echo "  1. Copy these files to the controller server:"
echo "     - ${CONTROLLER_KEY} -> /etc/x-ui/certs/controller.key"
echo "     - ${CONTROLLER_CERT} -> /etc/x-ui/certs/controller.crt"
echo "     - ${CA_CERT} -> /etc/x-ui/certs/ca.crt"
echo ""
echo "  2. When adding a remote agent in the UI, configure mTLS auth with:"
echo "     Auth Type: mTLS"
echo "     Auth Data (JSON):"
echo '     {'
echo '       "certFile": "/etc/x-ui/certs/controller.crt",'
echo '       "keyFile": "/etc/x-ui/certs/controller.key",'
echo '       "caFile": "/etc/x-ui/certs/ca.crt"'
echo '     }'
echo ""
echo "  3. Ensure secure permissions on controller:"
echo "     chmod 600 /etc/x-ui/certs/controller.key"
echo "     chmod 644 /etc/x-ui/certs/controller.crt"
echo "     chmod 644 /etc/x-ui/certs/ca.crt"
echo ""

# Display certificate info
echo "Certificate Details:"
openssl x509 -in "${CONTROLLER_CERT}" -noout -subject -issuer -dates
echo ""
