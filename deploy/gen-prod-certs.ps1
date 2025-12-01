# Production mTLS Certificate Generation for 3x-ui Multi-Server
# Generates CA, Controller client cert, and Agent server certs

$ErrorActionPreference = "Stop"
Set-Location "$PSScriptRoot\certs"

Write-Host "=== Generating Controller Client Certificate ===" -ForegroundColor Green
openssl genrsa -out controller/controller.key 2048
openssl req -new -key controller/controller.key -out controller/controller.csr -subj "/CN=3x-ui-controller"

@"
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid,issuer
"@ | Out-File -Encoding ascii controller/controller.ext

openssl x509 -req -in controller/controller.csr -CA ca/ca.crt -CAkey ca/ca.key -CAcreateserial -out controller/controller.crt -days 365 -extfile controller/controller.ext
Remove-Item controller/controller.csr, controller/controller.ext
Write-Host "OK - Controller cert generated" -ForegroundColor Green

Write-Host ""
Write-Host "=== Generating Agent1 Server Certificate (vpn-test) ===" -ForegroundColor Green
openssl genrsa -out agent-vpn-test/agent.key 2048
openssl req -new -key agent-vpn-test/agent.key -out agent-vpn-test/agent.csr -subj "/CN=agent-vpn-test"

@"
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid,issuer
subjectAltName = @alt_names

[alt_names]
IP.1 = 31.57.93.249
DNS.1 = vpn-test
"@ | Out-File -Encoding ascii agent-vpn-test/agent.ext

openssl x509 -req -in agent-vpn-test/agent.csr -CA ca/ca.crt -CAkey ca/ca.key -CAcreateserial -out agent-vpn-test/agent.crt -days 365 -extfile agent-vpn-test/agent.ext
Remove-Item agent-vpn-test/agent.csr, agent-vpn-test/agent.ext
Write-Host "OK - Agent1 cert generated" -ForegroundColor Green

Write-Host ""
Write-Host "=== Generating Agent2 Server Certificate (vpn-telepuziks) ===" -ForegroundColor Green
openssl genrsa -out agent-vpn-telepuziks/agent.key 2048
openssl req -new -key agent-vpn-telepuziks/agent.key -out agent-vpn-telepuziks/agent.csr -subj "/CN=agent-vpn-telepuziks"

@"
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid,issuer
subjectAltName = @alt_names

[alt_names]
IP.1 = 91.217.76.87
DNS.1 = vpn-telepuziks
"@ | Out-File -Encoding ascii agent-vpn-telepuziks/agent.ext

openssl x509 -req -in agent-vpn-telepuziks/agent.csr -CA ca/ca.crt -CAkey ca/ca.key -CAcreateserial -out agent-vpn-telepuziks/agent.crt -days 365 -extfile agent-vpn-telepuziks/agent.ext
Remove-Item agent-vpn-telepuziks/agent.csr, agent-vpn-telepuziks/agent.ext
Write-Host "OK - Agent2 cert generated" -ForegroundColor Green

Write-Host ""
Write-Host "=== Verifying Certificates ===" -ForegroundColor Yellow
openssl verify -CAfile ca/ca.crt controller/controller.crt
openssl verify -CAfile ca/ca.crt agent-vpn-test/agent.crt
openssl verify -CAfile ca/ca.crt agent-vpn-telepuziks/agent.crt

Write-Host ""
Write-Host "=== Certificate Generation Complete! ===" -ForegroundColor Green
Get-ChildItem -Recurse -Include *.crt,*.key | Format-Table Directory, Name, Length
