#!/usr/bin/env bash
# Copyright (C) 2022 Storj Labs, Inc.
# See LICENSE for copying information.

cd "$(dirname "${BASH_SOURCE[0]}")"
set -xuo pipefail

CHECK=web/multinode

CHANGED=false

#last commit
git diff HEAD HEAD~1 --name-only | grep $CHECK

if [ $? -eq 0 ]; then
    ./build.sh
fi

#working directory
git diff --name-only | grep $CHECK

if [ $? -eq 0 ]; then
    ./build.sh
fi

