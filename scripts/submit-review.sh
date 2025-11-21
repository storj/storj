#!/usr/bin/env bash
set -euo pipefail

if [ -z "${2:-}" ]; then
  COMMIT=$(git rev-parse HEAD)
else
  COMMIT=$(git rev-parse $2)
fi

if [ -z "$COMMIT" ]; then
  echo "Could not determine the current commit hash."
  exit 1
fi

if [ -z "${1:-}" ]; then
  echo "Usage: $0 <review-content-json> [current-commit-hash]"
  exit 1
fi

if [ ! -f "$1" ]; then
  echo "The file '$1' does not exist."
  exit 0
fi

if [ ! $GERRIT_TOKEN ]; then
  echo "GERRIT_TOKEN is not set. Please set it to your Gerrit API key."
  exit 1
fi

if [ ! $GERRIT_USER ]; then
  echo "GERRIT_USER is not set. Please set it to your Gerrit user."
  exit 1
fi

CHANGE_ID=$(curl -u $GERRIT_USER:$GERRIT_TOKEN  "https://review.dev.storj.tools/a/changes/?o=ALL_REVISIONS&q=commit:$COMMIT" | tail -n -1 | jq -r '.[0].triplet_id')
curl -u $GERRIT_USER:$GERRIT_TOKEN --basic -X POST "https://review.dev.storj.tools/a/changes/$CHANGE_ID/revisions/$COMMIT/review" \
  -H "Content-Type: application/json" \
  --data @$1
