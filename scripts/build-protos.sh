#!/usr/bin/env bash

protoc -Iprotos/overlay -Iprotos/google protos/overlay/overlay.proto --go_out=plugins=grpc:protos/overlay/