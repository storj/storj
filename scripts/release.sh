#!/usr/bin/env bash
set -eu
set -o pipefail

echo Running "go $@"
exec go "$1" "${@:2}"
