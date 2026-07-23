#!/usr/bin/env bash
#
# Generate self-signed dev TLS certificates for the mTLS gRPC and Thrift servers.
#
# Produces a local CA plus a server and client certificate (all signed by that CA)
# in the certs/ directory. For local development only — never used in production,
# where TLS is handled at the AWS infrastructure layer.
#
# IMPORTANT — X.509 extensions are required, not optional:
# An earlier version of these commands produced minimal certs with no keyUsage /
# extendedKeyUsage extensions (and a CA with no keyUsage at all). Go's TLS verifier
# and `openssl s_client` accept such certs, so the gRPC server worked — but strict
# RFC 5280 verifiers reject them. Python 3.14 + OpenSSL 3.6 (the Thrift Python
# client) failed with "CA cert does not include key usage extension", which
# surfaced confusingly as a "bad record MAC" error on the Go server side. The fix
# is to issue standards-compliant certs:
#   - CA:    basicConstraints=CA:TRUE, keyUsage=keyCertSign,cRLSign
#   - server: keyUsage=digitalSignature,keyEncipherment, extendedKeyUsage=serverAuth, SAN
#   - client: keyUsage=digitalSignature, extendedKeyUsage=clientAuth
#
# Usage (from the repo root):
#   ./certs/gen-certs.sh
#
# After regenerating, restart any running server so it reloads the new certs.

set -euo pipefail

# Resolve the certs/ directory relative to this script so it works from any cwd.
CERTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "Generating dev TLS certificates in ${CERTS_DIR} ..."

# --- CA ---
# keyUsage=keyCertSign is required — strict verifiers (e.g. Python's ssl) reject a
# CA cert with no keyUsage extension.
openssl genrsa -out "${CERTS_DIR}/ca.key" 4096
openssl req -new -x509 -days 3650 -key "${CERTS_DIR}/ca.key" -out "${CERTS_DIR}/ca.crt" \
  -subj "/CN=dev-ca" \
  -addext "basicConstraints=critical,CA:TRUE" \
  -addext "keyUsage=critical,keyCertSign,cRLSign"

# --- Server ---
# SAN required (Go 1.15+ rejects CN-only certs); keyUsage + extendedKeyUsage=serverAuth
# required by strict verifiers.
openssl genrsa -out "${CERTS_DIR}/server.key" 4096
openssl req -new -key "${CERTS_DIR}/server.key" -out "${CERTS_DIR}/server.csr" \
  -subj "/CN=localhost"
openssl x509 -req -days 825 -in "${CERTS_DIR}/server.csr" \
  -CA "${CERTS_DIR}/ca.crt" -CAkey "${CERTS_DIR}/ca.key" -CAcreateserial \
  -extfile <(printf "subjectAltName=DNS:localhost,IP:127.0.0.1\nbasicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature,keyEncipherment\nextendedKeyUsage=serverAuth") \
  -out "${CERTS_DIR}/server.crt"

# --- Client ---
# keyUsage + extendedKeyUsage=clientAuth required by strict verifiers.
openssl genrsa -out "${CERTS_DIR}/client.key" 4096
openssl req -new -key "${CERTS_DIR}/client.key" -out "${CERTS_DIR}/client.csr" \
  -subj "/CN=dev-client"
openssl x509 -req -days 825 -in "${CERTS_DIR}/client.csr" \
  -CA "${CERTS_DIR}/ca.crt" -CAkey "${CERTS_DIR}/ca.key" -CAcreateserial \
  -extfile <(printf "basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature\nextendedKeyUsage=clientAuth") \
  -out "${CERTS_DIR}/client.crt"

# Clean up intermediate CSRs.
rm -f "${CERTS_DIR}/server.csr" "${CERTS_DIR}/client.csr"

echo "Done. Verifying chain:"
openssl verify -CAfile "${CERTS_DIR}/ca.crt" "${CERTS_DIR}/server.crt" "${CERTS_DIR}/client.crt"
