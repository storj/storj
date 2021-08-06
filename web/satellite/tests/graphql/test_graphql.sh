#!/usr/bin/env bash
set -ueo pipefail

go run "$REPOROOT"/web/satellite/tests/graphql/main.go
# docker pull postman/newman:alpine
# docker run --network="host" -v ${PWD}/web/satellite/tests/graphql/:/etc/newman -t postman/newman:alpine run GraphQL.postman_collection.json -e GraphQLEndoints.postman_environment.json;
