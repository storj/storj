#!/usr/bin/env bash
set -euxo pipefail
curl -X POST -F "jenkinsfile=<$1" https://build.dev.storj.io/pipeline-model-converter/validate
