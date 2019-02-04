#!/bin/bash
set -ueo pipefail

CHANGES=$(git diff --name-only $TRAVIS_COMMIT_RANGE -- go.mod go.sum)

if [ -z "$CHANGES" ]
then
    echo "go modules not changed"
else
    echo "go module changes detected"
    gospace istidy
fi
