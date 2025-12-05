#!/usr/bin/env bash
set -ex
storj-up init nomad --name=core --ip=$IP minimal,gc
storj-up image satellite-api,storagenode,gc $IMAGE:$TAG
storj-up persist storagenode,satellite-api,gc
storj-up env set satellite-api STORJ_DATABASE_OPTIONS_MIGRATION_UNSAFE=full,testdata
nomad run storj.hcl
