#!/usr/bin/env bash
# Create a local self-signed CODE SIGNING certificate and import it into the
# login keychain. Signing the app with a STABLE identity (instead of the default
# ad-hoc signature, whose hash changes every build) lets macOS keep the Screen
# Recording (TCC) grant across rebuilds.
#
# Run once per machine. `security find-identity -v` will still show 0 "valid"
# identities because the cert is self-signed/untrusted — that's expected and
# fine: codesign signs with the private key regardless, and TCC keys on the
# signing identity, not on Gatekeeper trust.
set -euo pipefail

IDENTITY_NAME="${SIGN_IDENTITY:-Audit Extractor Dev}"
KEYCHAIN="$HOME/Library/Keychains/login.keychain-db"

if security find-identity -p codesigning | grep -q "$IDENTITY_NAME"; then
  echo "Identity '$IDENTITY_NAME' already present."
  exit 0
fi

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

cat > "$WORK/cert.conf" <<EOF
[ req ]
distinguished_name = dn
x509_extensions = v3
prompt = no
[ dn ]
CN = $IDENTITY_NAME
[ v3 ]
keyUsage = critical, digitalSignature
extendedKeyUsage = critical, codeSigning
basicConstraints = critical, CA:false
EOF

openssl req -x509 -newkey rsa:2048 -sha256 -days 3650 -nodes \
  -keyout "$WORK/key.pem" -out "$WORK/cert.pem" -config "$WORK/cert.conf"

# -legacy is required: OpenSSL 3 PKCS12 defaults are unreadable by Apple's `security`.
openssl pkcs12 -export -legacy -macalg sha1 \
  -inkey "$WORK/key.pem" -in "$WORK/cert.pem" \
  -name "$IDENTITY_NAME" -out "$WORK/id.p12" -passout pass:audit

security import "$WORK/id.p12" -k "$KEYCHAIN" -P audit \
  -T /usr/bin/codesign -T /usr/bin/security

echo
echo "Imported '$IDENTITY_NAME'. Now run: make dist"
echo "(The first codesign may prompt for keychain access — click 'Always Allow'.)"
