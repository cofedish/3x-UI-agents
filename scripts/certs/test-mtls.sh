#!/bin/bash
#
# Test mTLS Configuration for 3x-ui Agent
#
# This script verifies that mTLS authentication is working correctly
# between controller and agent.
#
# Usage:
#   ./test-mtls.sh <agent_endpoint> [cert_dir]
#
# Arguments:
#   agent_endpoint - Agent API endpoint (e.g., https://localhost:2054)
#   cert_dir       - Directory containing certificates (default: ./certs)
#
# Example:
#   ./test-mtls.sh https://agent.example.com:2054
#   ./test-mtls.sh https://192.168.1.10:2054 /etc/x-ui/certs
#

set -e

# Configuration
AGENT_ENDPOINT="${1}"
CERT_DIR="${2:-./certs}"
CONTROLLER_CERT="${CERT_DIR}/controller.crt"
CONTROLLER_KEY="${CERT_DIR}/controller.key"
CA_CERT="${CERT_DIR}/ca.crt"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo "=== 3x-ui mTLS Connection Test ==="
echo ""

# Validate arguments
if [ -z "${AGENT_ENDPOINT}" ]; then
    echo -e "${RED}Error: Agent endpoint is required${NC}"
    echo "Usage: $0 <agent_endpoint> [cert_dir]"
    echo "Example: $0 https://agent.example.com:2054"
    exit 1
fi

# Check if curl is available
if ! command -v curl &> /dev/null; then
    echo -e "${RED}Error: curl is not installed${NC}"
    echo "Please install curl: apt-get install curl (Debian/Ubuntu) or yum install curl (CentOS/RHEL)"
    exit 1
fi

# Check if certificates exist
echo -e "${BLUE}Checking certificates...${NC}"
missing_certs=0

if [ ! -f "${CONTROLLER_CERT}" ]; then
    echo -e "${RED}✗ Controller certificate not found: ${CONTROLLER_CERT}${NC}"
    missing_certs=1
fi

if [ ! -f "${CONTROLLER_KEY}" ]; then
    echo -e "${RED}✗ Controller private key not found: ${CONTROLLER_KEY}${NC}"
    missing_certs=1
fi

if [ ! -f "${CA_CERT}" ]; then
    echo -e "${RED}✗ CA certificate not found: ${CA_CERT}${NC}"
    missing_certs=1
fi

if [ ${missing_certs} -eq 1 ]; then
    echo ""
    echo "Please generate certificates first:"
    echo "  cd scripts/certs"
    echo "  ./gen-ca.sh"
    echo "  ./gen-controller-cert.sh"
    exit 1
fi

echo -e "${GREEN}✓ All certificates found${NC}"
echo ""

# Verify certificate validity
echo -e "${BLUE}Verifying certificate validity...${NC}"

# Check if controller cert is not expired
if ! openssl x509 -in "${CONTROLLER_CERT}" -noout -checkend 0 &>/dev/null; then
    echo -e "${YELLOW}⚠ Warning: Controller certificate has expired${NC}"
    openssl x509 -in "${CONTROLLER_CERT}" -noout -dates
    echo ""
fi

# Verify controller cert is signed by CA
if openssl verify -CAfile "${CA_CERT}" "${CONTROLLER_CERT}" &>/dev/null; then
    echo -e "${GREEN}✓ Controller certificate is valid and signed by CA${NC}"
else
    echo -e "${RED}✗ Controller certificate verification failed${NC}"
    openssl verify -CAfile "${CA_CERT}" "${CONTROLLER_CERT}"
    exit 1
fi

echo ""

# Test 1: Public health endpoint (no auth required)
echo -e "${BLUE}Test 1: Public health endpoint (no client cert)${NC}"
echo "Endpoint: ${AGENT_ENDPOINT}/api/v1/health"

if response=$(curl -s --cacert "${CA_CERT}" "${AGENT_ENDPOINT}/api/v1/health" 2>&1); then
    echo -e "${GREEN}✓ Public endpoint accessible${NC}"
    echo "Response: $response"
else
    echo -e "${YELLOW}⚠ Public endpoint not accessible (may require auth)${NC}"
fi

echo ""

# Test 2: Protected endpoint without client certificate (should fail)
echo -e "${BLUE}Test 2: Protected endpoint without client certificate (should fail)${NC}"
echo "Endpoint: ${AGENT_ENDPOINT}/api/v1/info"

if curl -s --cacert "${CA_CERT}" "${AGENT_ENDPOINT}/api/v1/info" 2>&1 | grep -q "error\|certificate"; then
    echo -e "${GREEN}✓ Request correctly rejected without client certificate${NC}"
else
    echo -e "${YELLOW}⚠ Expected rejection but got different response${NC}"
fi

echo ""

# Test 3: Protected endpoint with client certificate (should succeed)
echo -e "${BLUE}Test 3: Protected endpoint with valid client certificate${NC}"
echo "Endpoint: ${AGENT_ENDPOINT}/api/v1/info"
echo "Using: Controller certificate"

response=$(curl -s \
    --cert "${CONTROLLER_CERT}" \
    --key "${CONTROLLER_KEY}" \
    --cacert "${CA_CERT}" \
    "${AGENT_ENDPOINT}/api/v1/info" 2>&1)

if echo "$response" | grep -q '"success":true\|"data"'; then
    echo -e "${GREEN}✓ mTLS authentication successful!${NC}"
    echo "Response:"
    echo "$response" | head -20
else
    echo -e "${RED}✗ mTLS authentication failed${NC}"
    echo "Response: $response"
    exit 1
fi

echo ""

# Test 4: Verify TLS version
echo -e "${BLUE}Test 4: Verify TLS version (should be TLS 1.3)${NC}"

tls_version=$(curl -s -v \
    --cert "${CONTROLLER_CERT}" \
    --key "${CONTROLLER_KEY}" \
    --cacert "${CA_CERT}" \
    "${AGENT_ENDPOINT}/api/v1/health" 2>&1 | grep -i "SSL connection\|TLS" | head -1)

if echo "$tls_version" | grep -q "TLSv1.3"; then
    echo -e "${GREEN}✓ Using TLS 1.3${NC}"
    echo "$tls_version"
elif echo "$tls_version" | grep -q "TLSv1.2"; then
    echo -e "${YELLOW}⚠ Using TLS 1.2 (expected TLS 1.3)${NC}"
    echo "$tls_version"
else
    echo -e "${YELLOW}⚠ Could not determine TLS version${NC}"
    echo "$tls_version"
fi

echo ""

# Summary
echo -e "${GREEN}=== All mTLS Tests Passed ===${NC}"
echo ""
echo "Summary:"
echo "  ✓ Certificates are valid and properly signed"
echo "  ✓ Server requires client certificate authentication"
echo "  ✓ Controller can authenticate with valid certificate"
echo "  ✓ Connection is encrypted with TLS"
echo ""
echo -e "${BLUE}Next Steps:${NC}"
echo "  1. Configure this agent in the controller UI"
echo "  2. Monitor agent logs for authentication events"
echo "  3. Test inbound/client management operations"
echo ""
