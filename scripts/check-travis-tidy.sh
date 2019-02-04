#!/bin/bash

CHANGES=$(git diff --name-only $TRAVIS_COMMIT_RANGE -- go.mod go.sum) || CHANGES="fail"

if [ -z "$CHANGES" ]
then
    echo "go modules not changed"
else
    echo "go module changes detected: $CHANGES"
    gospace istidy
fi
