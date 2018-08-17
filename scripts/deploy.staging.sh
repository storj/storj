#!/bin/bash

PROJECT_NAME=$1
CONTAINER=$2

kubectl config set-cluster storj-nonprod
kubectl --namespace storj-staging patch deployment $PROJECT_NAME -p"{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"$PROJECT_NAME\",\"image\":\"$CONTAINER\"}]}}}}"
