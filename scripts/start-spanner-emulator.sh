#!/usr/bin/env bash
set -euxo pipefail

/usr/local/bin/spanner_emulator --override_max_databases_per_instance 10000 "$@"
