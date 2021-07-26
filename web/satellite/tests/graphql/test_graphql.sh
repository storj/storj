#!/usr/bin/env bash

# Stop on first error
set -e;
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

trap onExit EXIT;

go run web/satellite/tests/graphql/main.go
docker pull postman/newman:alpine;
docker run --network="host" -v ${PWD}/web/satellite/tests/graphql/:/etc/newman -t postman/newman:alpine run GraphQL.postman_collection.json -e GraphQLEndoints.postman_environment.json;
