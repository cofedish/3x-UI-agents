#!/bin/bash
#
# Generate Certificate Authority (CA) for 3x-ui Agent mTLS
#
# This script creates a root CA certificate and private key used to sign
# both agent and controller certificates for mutual TLS authentication.
#
# Usage:
#   ./gen-ca.sh [output_dir] [ca_name]
#
# Arguments:
#   output_dir - Directory to store CA files (default: ./certs)
#   ca_name    - Common Name for CA (default: 3x-ui-agents-ca)
#
# Outputs:
#   ca.key - CA private key (KEEP SECURE!)
#   ca.crt - CA certificate (distribute to agents and controller)
#

set -e

# Configuration
OUTPUT_DIR="${1:-./certs}"
CA_NAME="${2:-3x-ui-agents-ca}"
CA_KEY="${OUTPUT_DIR}/ca.key"
CA_CERT="${OUTPUT_DIR}/ca.crt"
DAYS_VALID=3650  # 10 years

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=== 3x-ui Agent CA Generator ==="
echo ""

# Check if openssl is available
if ! command -v openssl &> /dev/null; then
    echo -e "${RED}Error: openssl is not installed${NC}"
    echo "Please install openssl: apt-get install openssl (Debian/Ubuntu) or yum install openssl (CentOS/RHEL)"
    exit 1
fi

# Create output directory
mkdir -p "${OUTPUT_DIR}"

# Check if CA already exists
if [ -f "${CA_KEY}" ] || [ -f "${CA_CERT}" ]; then
    echo -e "${YELLOW}Warning: CA files already exist in ${OUTPUT_DIR}${NC}"
    echo "Existing files:"
    [ -f "${CA_KEY}" ] && echo "  - ca.key"
    [ -f "${CA_CERT}" ] && echo "  - ca.crt"
    echo ""
    read -p "Do you want to overwrite them? This will invalidate all existing certificates! [y/N]: " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Aborted."
        exit 1
    fi
    echo ""
fi

# Generate CA private key (4096-bit RSA for long-term security)
echo "Generating CA private key..."
openssl genrsa -out "${CA_KEY}" 4096 2>/dev/null

# Set secure permissions on private key
chmod 600 "${CA_KEY}"

# Generate CA certificate
echo "Generating CA certificate (valid for ${DAYS_VALID} days)..."
openssl req -new -x509 -days ${DAYS_VALID} -key "${CA_KEY}" -out "${CA_CERT}" \
    -subj "/C=US/ST=State/L=City/O=3x-ui/OU=Agents/CN=${CA_NAME}" \
    2>/dev/null

# Set readable permissions on certificate
chmod 644 "${CA_CERT}"

echo ""
echo -e "${GREEN}=== CA Generation Complete ===${NC}"
echo ""
echo "Generated files:"
echo "  CA Private Key: ${CA_KEY}"
echo "  CA Certificate: ${CA_CERT}"
echo ""
echo -e "${YELLOW}IMPORTANT SECURITY NOTES:${NC}"
echo "  1. Keep ca.key SECURE and PRIVATE - anyone with this can sign certificates!"
echo "  2. Distribute ca.crt to all agents and the controller"
echo "  3. Store ca.key in a secure location with restricted access (chmod 600)"
echo "  4. Consider using a password-protected key for production: openssl genrsa -aes256"
echo ""
echo "Next steps:"
echo "  1. Generate agent certificates: ./gen-agent-cert.sh <agent-id>"
echo "  2. Generate controller certificate: ./gen-controller-cert.sh"
echo ""

# Display CA certificate info
echo "CA Certificate Details:"
openssl x509 -in "${CA_CERT}" -noout -subject -issuer -dates
echo ""
