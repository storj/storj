#!/usr/bin/env bash

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

CONTAINER_NAME=storj_sim_postgres
docker run -d --rm -p 5433:5432 --name $CONTAINER_NAME postgres

cleanup(){
  docker rm -f $CONTAINER_NAME
}
trap cleanup EXIT

STORJ_SIM_DATABASE=${STORJ_SIM_DATABASE:-"teststorj"}

RETRIES=5

until docker exec $CONTAINER_NAME psql -h localhost -U postgres -d postgres -c "select 1" > /dev/null 2>&1 || [ $RETRIES -eq 0 ]; do
  echo "Waiting for postgres server, $((RETRIES--)) remaining attempts..."
  sleep 1
done

docker exec $CONTAINER_NAME psql -h localhost -U postgres -c "create database $STORJ_SIM_DATABASE;"

export STORJ_SIM_POSTGRES="postgres://postgres@localhost:5433/$STORJ_SIM_DATABASE?sslmode=disable"

$SCRIPTDIR/test-sim.sh