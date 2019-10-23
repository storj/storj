#!/usr/bin/env bash

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

docker run --rm -p 5433:5432 --name storj_sim_postgres postgres &

cleanup(){
  docker rm -f storj_sim_postgres
}
trap cleanup EXIT

STORJ_SIM_DATABASE=${STORJ_SIM_DATABASE:-"teststorj"}

RETRIES=5

until psql -h localhost -U postgres -d postgres -c "select 1" > /dev/null 2>&1 || [ $RETRIES -eq 0 ]; do
  echo "Waiting for postgres server, $((RETRIES--)) remaining attempts..."
  sleep 1
done

psql -h localhost -p 5433 -U postgres -c "create database $STORJ_SIM_DATABASE;"

export STORJ_SIM_POSTGRES="postgres://postgres@localhost:5433/$STORJ_SIM_DATABASE?sslmode=disable"

$SCRIPTDIR/test-sim.sh