#!/bin/bash
set -ueo pipefail

echo "$GCLOUD_KEY" | gcloud auth activate-service-account --key-file=-
gcloud config set project "$GCP_PROJECT"
gcloud config set compute/zone "$GCP_ZONE"
gcloud auth configure-docker
gcloud container clusters get-credentials "$CLUSTER_NAME"
