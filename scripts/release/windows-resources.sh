#!/usr/bin/env bash

# Copyright (C) 2025 Storj Labs, Inc.
# See LICENSE for copying information.

# This script generates Windows resources for each component.
#
# Windows requires a special syso file that contains version information.
# This information is used by Windows to display the version of the application.
# It also contains an icon and a description.

set -euo pipefail

# Validate VERSION is set
if [ -z "${VERSION:-}" ]; then
    echo "Error: VERSION environment variable is not set" >&2
    exit 1
fi

# Version without the v prefix and without prerelease and metadata information
CORE=$(printf '%s' "${VERSION}" | cut -d'v' -f2- | cut -d'-' -f1 | cut -d'+' -f1)
# Separate the version fields for Windows resource files.
MAJOR="$(printf '%s' "${CORE}" | cut -d'.' -f1)"
MINOR="$(printf '%s' "${CORE}" | cut -d'.' -f2)"
PATCH="$(printf '%s' "${CORE}" | cut -d'.' -f3)"

# Validate version components are numeric
if ! [[ "$MAJOR" =~ ^[0-9]+$ ]]; then
    echo "Error: Invalid version format - MAJOR must be numeric (got: '$MAJOR' from VERSION='$VERSION')" >&2
    exit 1
fi
if ! [[ "$MINOR" =~ ^[0-9]+$ ]]; then
    echo "Error: Invalid version format - MINOR must be numeric (got: '$MINOR' from VERSION='$VERSION')" >&2
    exit 1
fi
if ! [[ "$PATCH" =~ ^[0-9]+$ ]]; then
    echo "Error: Invalid version format - PATCH must be numeric (got: '$PATCH' from VERSION='$VERSION')" >&2
    exit 1
fi

PRERELEASE="$(printf '%s' "${VERSION}" | cut -d'-' -s -f2- | cut -d'+' -f1-)"
METADATA="$(printf '%s' "${VERSION}" | cut -d'+' -s -f2-)"

for component in "$@"; do
	echo "Generating Windows resources for ${component}"
	name="$(basename "$component")"
	goversioninfo -64 -o "$component/resource_windows_amd64.syso" \
		-original-name "$name.exe" \
		-description "$name program for Storj" \
		-product-ver-major "${MAJOR}" -ver-major "${MAJOR}" \
		-product-ver-minor "${MINOR}" -ver-minor "${MINOR}" \
		-product-ver-patch "${PATCH}" -ver-patch "${PATCH}" \
		-product-version "${MAJOR}.${MINOR}.${PATCH}-${PRERELEASE}" \
		-special-build "${METADATA}" \
		-icon resources/icon.ico \
		resources/versioninfo.json
done
