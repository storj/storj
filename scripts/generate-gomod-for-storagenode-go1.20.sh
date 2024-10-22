#!/usr/bin/env bash

# This script reverts dependencies that prevent
# storagenode being compiled with Go 1.20.
#
# We need to compile storagenode with Go 1.20 to
# support older Window releases for the time being.

set -eu
set -o pipefail

cp "$1" "$2"
cp "${1%.*}.sum" "${2%.*}.sum"

# downgrade dependencies that require Go 1.21+
go get -modfile "$2" \
    cloud.google.com/go@v0.115.0 \
    cloud.google.com/go/auth@v0.7.0 \
    cloud.google.com/go/auth/oauth2adapt@v0.2.2 \
    cloud.google.com/go/bigquery@v1.61.0 \
    cloud.google.com/go/compute/metadata@v0.4.0 \
    cloud.google.com/go/iam@v1.1.10 \
    cloud.google.com/go/longrunning@v0.5.10 \
    cloud.google.com/go/secretmanager@v1.13.3 \
    cloud.google.com/go/spanner@v1.64.0 \
    github.com/envoyproxy/go-control-plane@v0.12.0 \
    github.com/go-logr/logr@v1.4.1 \
    github.com/golang/glog@v1.2.0 \
    github.com/google/s2a-go@v0.1.7 \
    github.com/googleapis/gax-go/v2@v2.12.5 \
    github.com/googleapis/go-sql-spanner@v1.6.1-0.20240816093805-21f8b75ae3fc \
    go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc@v0.49.0 \
    go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp@v0.49.0 \
    go.opentelemetry.io/otel@v1.24.0 \
    go.opentelemetry.io/otel/metric@v1.24.0 \
    go.opentelemetry.io/otel/trace@v1.24.0 \
    google.golang.org/api@v0.188.0 \
    google.golang.org/genproto@v0.0.0-20240711142825-46eb208f015d \
    google.golang.org/genproto/googleapis/api@v0.0.0-20240701130421-f6361c86f094 \
    google.golang.org/genproto/googleapis/rpc@v0.0.0-20240711142825-46eb208f015d \
    google.golang.org/grpc@v1.64.1 \
    github.com/cncf/xds/go@v0.0.0-20240423153145-555b57ec207b \
    github.com/envoyproxy/protoc-gen-validate@v1.0.4 \
    github.com/googleapis/enterprise-certificate-proxy@v0.3.2 \
    github.com/coreos/go-oidc/v3@v3.9.0

# set the go version and remove toolchain line
go mod edit -go 1.20 "$2"

# tidy the sum file
go mod tidy -modfile "$2"

# remove toolchain line, because it confuses go1.20
sed -i -e '/toolchain/d' "$2"
