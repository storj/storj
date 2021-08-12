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
newman run "$REPOROOT"/web/satellite/tests/graphql/GraphQL.postman_collection.json -e "$REPOROOT"/web/satellite/tests/graphql/GraphQLEndPoints_Dev.postman_environment.json --suppress-exit-code 1