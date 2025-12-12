#!/usr/bin/env bash

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

cleanup(){
  docker rm -f postgres-$BUILD_NUMBER
  docker rm -f redis-$BUILD_NUMBER
}
trap cleanup EXIT

# TODO somehow provide `shasum` binary 

BUILD_NUMBER="dev"

export STORJ_SIM_POSTGRES="postgres://postgres@localhost:5433/teststorj?sslmode=disable"
export STORJ_SIM_REDIS="localhost:6380"

docker run --rm -d -p 5433:5432 -e POSTGRES_HOST_AUTH_METHOD=trust --name postgres-$BUILD_NUMBER postgres:17
docker run --rm -d -p 6380:6379 --name redis-$BUILD_NUMBER redis:latest

until $(docker logs postgres-$BUILD_NUMBER | grep "database system is ready to accept connections" > /dev/null)
    do printf '.'
    sleep 5
done

docker exec postgres-$BUILD_NUMBER createdb -U postgres teststorj
# fetch the remote main branch
git fetch --no-tags --progress -- https://github.com/storj/storj.git +refs/heads/main:refs/remotes/origin/main
$SCRIPTDIR/start-sim.sh
