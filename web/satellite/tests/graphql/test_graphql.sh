#!/usr/bin/env bash
set -ueo pipefail

function onExit {
    if [ "$?" != "0" ]; then
        echo "Tests failed";
        # build failed, don't deploy
        exit 1;
    else
        echo "Tests passed";
        # deploy build
		exit 0;
    fi
}

trap onExit EXIT
go run "$REPOROOT"/web/satellite/tests/graphql/main.go

# docker run --network="host" -v "$REPOROOT"/web/satellite/tests/graphql/:/etc/newman -t postman/newman:alpine run GraphQL.postman_collection.json -e GraphQLEndoints.postman_environment.json;
