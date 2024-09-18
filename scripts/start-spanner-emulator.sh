#!/usr/bin/env bash
set -euxo pipefail

/usr/local/bin/spanner_emulator --abort_current_transaction_probability 0 "$@"