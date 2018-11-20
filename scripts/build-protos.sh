#!/usr/bin/env bash

# see https://github.com/gogo/protobuf#most-speed-and-most-customization
go get github.com/gogo/protobuf/proto
go get github.com/gogo/protobuf/jsonpb
go get github.com/gogo/protobuf/protoc-gen-gogo
go get github.com/gogo/protobuf/gogoproto

go generate ./pkg/pb ./pkg/statdb/proto
