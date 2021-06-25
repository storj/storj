#/bin/bash

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
    fi
}

trap onExit EXIT;

docker pull postman/newman:alpine;

docker run -v .:/etc/newman -t postman/newman:alpine run GraphQL.postman_collection.json -e GraphQLEndoints.postman_environment.json;
