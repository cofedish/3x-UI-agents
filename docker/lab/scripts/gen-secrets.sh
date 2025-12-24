#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
LAB_SHARED="$(cd "$SCRIPT_DIR/.." && pwd)/shared"
CERT_DIR="$LAB_SHARED/certs"
JWT_DIR="$LAB_SHARED/jwt"

mkdir -p "$CERT_DIR" "$JWT_DIR"

if [ ! -f "$CERT_DIR/ca.key" ] || [ ! -f "$CERT_DIR/ca.crt" ]; then
  echo "[lab-secrets] Generating CA"
  openssl req -x509 -nodes -newkey rsa:4096 -days 365 \
    -subj "/C=US/ST=Lab/L=Lab/O=3x-ui-lab/OU=Lab/CN=lab-ca" \
    -keyout "$CERT_DIR/ca.key" -out "$CERT_DIR/ca.crt"
fi

make_server_cert() {
  local name="$1"
  local dns_name="$2"
  if [ -f "$CERT_DIR/${name}.crt" ] && [ -f "$CERT_DIR/${name}.key" ]; then
    return
  fi
  echo "[lab-secrets] Generating server cert for $dns_name"
  openssl req -new -nodes -newkey rsa:2048 \
    -subj "/C=US/ST=Lab/L=Lab/O=3x-ui-lab/OU=Lab/CN=${dns_name}" \
    -keyout "$CERT_DIR/${name}.key" -out "$CERT_DIR/${name}.csr"

  openssl x509 -req -in "$CERT_DIR/${name}.csr" -days 365 \
    -CA "$CERT_DIR/ca.crt" -CAkey "$CERT_DIR/ca.key" -CAcreateserial \
    -out "$CERT_DIR/${name}.crt" \
    -extfile <(printf "subjectAltName=DNS:%s\nextendedKeyUsage=serverAuth" "$dns_name")

  rm -f "$CERT_DIR/${name}.csr"
}

make_client_cert() {
  local name="$1"
  if [ -f "$CERT_DIR/${name}.crt" ] && [ -f "$CERT_DIR/${name}.key" ]; then
    return
  fi
  echo "[lab-secrets] Generating client cert for $name"
  openssl req -new -nodes -newkey rsa:2048 \
    -subj "/C=US/ST=Lab/L=Lab/O=3x-ui-lab/OU=Lab/CN=${name}" \
    -keyout "$CERT_DIR/${name}.key" -out "$CERT_DIR/${name}.csr"

  openssl x509 -req -in "$CERT_DIR/${name}.csr" -days 365 \
    -CA "$CERT_DIR/ca.crt" -CAkey "$CERT_DIR/ca.key" -CAcreateserial \
    -out "$CERT_DIR/${name}.crt" \
    -extfile <(printf "extendedKeyUsage=clientAuth")

  rm -f "$CERT_DIR/${name}.csr"
}

make_server_cert "agent-mtls" "agent-mtls"
make_server_cert "agent-jwt" "agent-jwt"
make_client_cert "panel-client"

if [ ! -f "$JWT_DIR/token.txt" ]; then
  echo "[lab-secrets] Generating JWT token"
  openssl rand -hex 32 > "$JWT_DIR/token.txt"
  chmod 600 "$JWT_DIR/token.txt"
fi

chmod 600 "$CERT_DIR"/*.key
chmod 644 "$CERT_DIR"/*.crt

cat > "$LAB_SHARED/lab.env" <<EOF
LAB_SHARED=/opt/lab/shared
PANEL_PORT=2053
PANEL_BASE_PATH=lab
PANEL_USERNAME=admin
PANEL_PASSWORD=admin123
PANEL_LISTEN_IP=0.0.0.0
EOF

echo "[lab-secrets] Secrets ready in $LAB_SHARED"
