#!/bin/sh

set -euo pipefail

if [[ ! -f "config/config.yaml" ]]; then
	./storagenode setup --config-dir config --identity-dir /app/identity
fi