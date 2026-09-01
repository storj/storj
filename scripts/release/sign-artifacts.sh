#!/bin/sh

# Copyright (C) 2025 Storj Labs, Inc.
# See LICENSE for copying information.

# Signs Windows artifacts with Azure Trusted Signing from inside a docker build.
#
# Credentials come from build secrets (see the finalized-binaries bake target).
# Without them signing fails, unless ALLOW_UNSIGNED is set: then the artifacts
# are left alone and an .unsigned marker is written next to them, so that
# builds without access to the signing account still work. Whoever assembles
# the release renames the artifacts from there.
#
# Usage: sign-artifacts.sh <file>...

set -eu
# Never trace: the access token would end up in the build log.
set +x

secret() {
    if [ -s "/run/secrets/$1" ]; then cat "/run/secrets/$1"; fi
}

AZURE_TENANT_ID="$(secret azure_tenant_id)"
AZURE_CLIENT_ID="$(secret azure_client_id)"
AZURE_CLIENT_SECRET="$(secret azure_client_secret)"
SIGN_KEYSTORE="$(secret sign_keystore)"
SIGN_ALIAS="$(secret sign_alias)"

if [ $# -eq 0 ]; then
    echo "Usage: $0 <file>..." >&2
    exit 1
fi

if [ -z "$AZURE_TENANT_ID" ] || [ -z "$AZURE_CLIENT_ID" ] || [ -z "$AZURE_CLIENT_SECRET" ] ||
    [ -z "$SIGN_KEYSTORE" ] || [ -z "$SIGN_ALIAS" ]; then
    if [ -z "${ALLOW_UNSIGNED:-}" ]; then
        echo "Error: no signing credentials available" >&2
        echo "       pass ALLOW_UNSIGNED=1 to build unsigned artifacts instead" >&2
        exit 1
    fi

    echo "No signing credentials available, marking artifacts as unsigned"
    for file in "$@"; do
        touch "$(dirname "$file")/.unsigned"
    done
    exit 0
fi

# Fetching the token directly avoids installing the azure cli; the curl config
# keeps the client secret off the command line.
response="$(curl -sS --config - <<-CURLRC
	url = "https://login.microsoftonline.com/${AZURE_TENANT_ID}/oauth2/v2.0/token"
	data = "grant_type=client_credentials"
	data-urlencode = "client_id=${AZURE_CLIENT_ID}"
	data-urlencode = "client_secret=${AZURE_CLIENT_SECRET}"
	data-urlencode = "scope=https://codesigning.azure.net/.default"
	CURLRC
)"

STOREPASS="$(printf '%s' "$response" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4 || true)"
if [ -z "$STOREPASS" ]; then
    # The error fields describe the failure without echoing any credential.
    echo "Error: failed to acquire an Azure access token" >&2
    printf '%s' "$response" | grep -o '"error[^"]*":"[^"]*"' >&2 || true
    exit 1
fi
export STOREPASS

exec java -jar /jsign.jar \
    --storetype TRUSTEDSIGNING \
    --keystore "$SIGN_KEYSTORE" \
    --storepass env:STOREPASS \
    --alias "$SIGN_ALIAS" \
    "$@"
