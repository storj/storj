#!/bin/bash
set -ueo pipefail

# decrypt and create any secrets needed my the deployment
sops --decrypt deploy/development/sa.env.secret.sops.yaml | kubectl apply -f -
sops --decrypt deploy/development/sa.identity.secret.sops.yaml | | kubectl apply -f -

# replace the tag on the docker image with the current commit sha used
# when building/pushing image
sed -i -e 's#gcr.io/storj-jessica/sa-k8s-hard-way:latest#gcr.io/storj-jessica/sa-k8s-hard-way:${CI_SHA}#g' deploy/kustomize/

# apply the kustomize kubernetes configs
if [ "$ENV" = "dev" ]; then
    kubectl apply -f deploy/kustomize/overlays/dev
fi

if [ "$ENV" = "prod" ]; then
    kubectl apply -f deploy/kustomize/overlays/canary
fi
