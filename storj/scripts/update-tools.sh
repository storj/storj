#!/bin/sh
set -ue
[ -t 1 ] && echo "Checking for updated staticcheck at https://github.com/repos/dominikh/go-tools"
dominikh="$(curl https://api.github.com/repos/dominikh/go-tools/releases/latest -s | jq -r .tag_name)"
[ -t 1 ] && echo "Latest version of staticcheck is $dominikh"
sed -i'' "s/staticcheck@.*$/staticcheck@$dominikh/" Dockerfile.jenkins

[ -t 1 ] && echo "Checking for updated golangci-lint at https://github.com/repos/golangci/golangci-lint"
golangci="$(curl https://api.github.com/repos/golangci/golangci-lint/releases -s | jq -r '.[].tag_name' | sort -rV | head -n 1)"
[ -t 1 ] && echo "Latest version of golangci-lint is $golangci"
sed -i'' "/golangci-lint.sh/s/v[0-9.]*$/$golangci/" Dockerfile.jenkins
sed -i'' "/golangci-lint.sh/s/v[0-9.]*$/$golangci/" Makefile
git diff
