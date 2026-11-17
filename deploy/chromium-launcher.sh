#!/bin/sh
set -eu

browser=$(node -p 'require("/usr/lib/node_modules/playwright").chromium.executablePath()')

# Chromium does not read NODE_EXTRA_CA_CERTS. Trust only the recording CA's
# public key when capture supplies it, preserving verification for other keys.
# go-mitmproxy uses this same key for its generated interception certificates.
if [ -n "${NODE_EXTRA_CA_CERTS:-}" ]; then
    spki=$(node -e '
        const { X509Certificate, createHash } = require("node:crypto");
        const cert = new X509Certificate(require("node:fs").readFileSync(process.argv[1]));
        const key = cert.publicKey.export({ type: "spki", format: "der" });
        process.stdout.write(createHash("sha256").update(key).digest("base64"));
    ' "$NODE_EXTRA_CA_CERTS")
    exec "$browser" "--ignore-certificate-errors-spki-list=$spki" "$@"
fi

exec "$browser" "$@"
