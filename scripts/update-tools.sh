#!/bin/sh
set -ue
dominikh="$(curl https://api.github.com/repos/dominikh/go-tools/releases/latest -s | jq -r .tag_name)"
sed -i'' "s/staticcheck@.*$/staticcheck@$dominikh/" Dockerfile.jenkins

golangci="$(curl https://api.github.com/repos/golangci/golangci-lint/releases -s | jq -r '.[].tag_name' | sort -rV | head -n 1)"
sed -i'' "/golangci-lint.sh/s/v[0-9.]*$/$golangci/" Dockerfile.jenkins
sed -i'' "/golangci-lint.sh/s/v[0-9.]*$/$golangci/" Makefile
git diff
