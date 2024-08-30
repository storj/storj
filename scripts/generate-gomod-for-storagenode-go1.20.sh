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

# revert dependencies that bump go.mod to 1.21+
go mod edit \
    -require cloud.google.com/go@v0.115.0 \
    -require cloud.google.com/go/auth@v0.7.0 \
    -require cloud.google.com/go/auth/oauth2adapt@v0.2.2 \
    -require cloud.google.com/go/bigquery@v1.61.0 \
    -require cloud.google.com/go/compute/metadata@v0.4.0 \
    -require cloud.google.com/go/iam@v1.1.10 \
    -require cloud.google.com/go/longrunning@v0.5.10 \
    -require cloud.google.com/go/secretmanager@v1.13.3 \
    -require cloud.google.com/go/spanner@v1.64.0 \
    -require github.com/envoyproxy/go-control-plane@v0.12.0 \
    -require github.com/go-logr/logr@v1.4.1 \
    -require github.com/golang/glog@v1.2.0 \
    -require github.com/google/s2a-go@v0.1.7 \
    -require github.com/googleapis/gax-go/v2@v2.12.5 \
    -require github.com/googleapis/go-sql-spanner@v1.6.1-0.20240816093805-21f8b75ae3fc \
    -require go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc@v0.49.0 \
    -require go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp@v0.49.0 \
    -require go.opentelemetry.io/otel@v1.24.0 \
    -require go.opentelemetry.io/otel/metric@v1.24.0 \
    -require go.opentelemetry.io/otel/trace@v1.24.0 \
    -require google.golang.org/api@v0.188.0 \
    -require google.golang.org/genproto@v0.0.0-20240711142825-46eb208f015d \
    -require google.golang.org/genproto/googleapis/api@v0.0.0-20240701130421-f6361c86f094 \
    -require google.golang.org/genproto/googleapis/rpc@v0.0.0-20240711142825-46eb208f015d \
    -require google.golang.org/grpc@v1.64.1 \
    -go 1.20 \
    $2

# tidy the sum file
go mod tidy -modfile "$2"

# remove toolchain line, because it confuses go1.20
sed -i '/toolchain/d' "$2"