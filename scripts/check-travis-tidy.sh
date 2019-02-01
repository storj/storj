#!/bin/bash

[ $(git diff --name-only $TRAVIS_COMMIT_RANGE -- go.mod go.sum) ] && gospace tidy || true