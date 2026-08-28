#!/usr/bin/env bash

# Copyright (C) 2026 Storj Labs, Inc.
# See LICENSE for copying information.

# Builds the storagenode MSI with wixl (msitools). Requires installer/windows/bin/Storj.CA.dll
# (see installer/windows/ca) and the windows_amd64 storagenode binaries.

set -euo pipefail

if [ $# -ne 4 ]; then
    echo "Usage: $0 <build version> <storagenode.exe> <storagenode-updater.exe> <output.msi>"
    exit 1
fi

# MSI ProductVersion must be numeric major.minor.patch; drop the "v" prefix, prerelease and metadata.
VERSION=$(printf '%s' "$1" | cut -d'v' -f2- | cut -d'-' -f1 | cut -d'+' -f1)
if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "Error: cannot derive MSI version from '$1' (got '$VERSION')" >&2
    exit 1
fi

STORAGENODE_EXE="$(realpath "$2")"
UPDATER_EXE="$(realpath "$3")"
OUTPUT="$(realpath -m "$4")"
mkdir -p "$(dirname "$OUTPUT")"

cd "$(dirname "$0")"

# --extdir points wixl's UI extension at our vendored ui/ directory (Common.wxs uses our banner and dialog images).
wixl -a x64 --ext ui --extdir . \
    -D "Version=${VERSION}" \
    -D "StoragenodeExe=${STORAGENODE_EXE}" \
    -D "StoragenodeUpdaterExe=${UPDATER_EXE}" \
    -D "StorjCADll=bin/Storj.CA.dll" \
    -o "$OUTPUT" ./*.wxs

echo "Built $OUTPUT"
