#!/usr/bin/env bash
#
# One-shot IAM + storage setup for the satellite log reviewer.
# Run this once per environment. Idempotent.
#
# Required env:
#   PROJECT_ID    - GCP project (e.g. storj-prod)
#   REGION        - GCP region for Cloud Run (e.g. us-central1)
# Optional:
#   SA_NAME       - service account short name (default: satellite-log-reviewer)
#   STATE_BUCKET  - GCS bucket name (default: ${PROJECT_ID}-satellite-log-reviewer-state)

set -euo pipefail

: "${PROJECT_ID:?set PROJECT_ID}"
: "${REGION:?set REGION}"
SA_NAME="${SA_NAME:-satellite-log-reviewer}"
STATE_BUCKET="${STATE_BUCKET:-${PROJECT_ID}-satellite-log-reviewer-state}"
SA_EMAIL="${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"

echo ">> enabling required APIs"
gcloud services enable \
    run.googleapis.com \
    cloudscheduler.googleapis.com \
    secretmanager.googleapis.com \
    aiplatform.googleapis.com \
    logging.googleapis.com \
    storage.googleapis.com \
    artifactregistry.googleapis.com \
    cloudbuild.googleapis.com \
    --project "${PROJECT_ID}"

echo ">> creating service account (if missing): ${SA_EMAIL}"
if ! gcloud iam service-accounts describe "${SA_EMAIL}" --project "${PROJECT_ID}" >/dev/null 2>&1; then
    gcloud iam service-accounts create "${SA_NAME}" \
        --project "${PROJECT_ID}" \
        --display-name="Satellite log reviewer"
fi

echo ">> granting minimal roles on ${PROJECT_ID}"
for role in \
    roles/logging.viewer \
    roles/aiplatform.user \
    roles/secretmanager.secretAccessor; do
    gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
        --member="serviceAccount:${SA_EMAIL}" \
        --role="${role}" \
        --condition=None \
        --quiet >/dev/null
done

echo ">> creating state bucket: gs://${STATE_BUCKET}"
if ! gcloud storage buckets describe "gs://${STATE_BUCKET}" --project "${PROJECT_ID}" >/dev/null 2>&1; then
    gcloud storage buckets create "gs://${STATE_BUCKET}" \
        --project "${PROJECT_ID}" \
        --location="${REGION}" \
        --uniform-bucket-level-access \
        --public-access-prevention
fi

echo ">> granting bucket access to SA"
gcloud storage buckets add-iam-policy-binding "gs://${STATE_BUCKET}" \
    --member="serviceAccount:${SA_EMAIL}" \
    --role="roles/storage.objectAdmin" >/dev/null

echo ">> creating Secret Manager secrets (empty placeholders if missing)"
for secret in github-app-id github-installation-id github-private-key; do
    if ! gcloud secrets describe "${secret}" --project "${PROJECT_ID}" >/dev/null 2>&1; then
        gcloud secrets create "${secret}" --project "${PROJECT_ID}" --replication-policy=automatic
        echo "   -> created ${secret} (EMPTY — add a version before deploying)"
    fi
    gcloud secrets add-iam-policy-binding "${secret}" \
        --project "${PROJECT_ID}" \
        --member="serviceAccount:${SA_EMAIL}" \
        --role="roles/secretmanager.secretAccessor" >/dev/null
done

cat <<EOF

Done. Next steps:

  1. Create a GitHub App (see README.md), install it on storj/storj with
     repository contents:write, then feed its credentials into the secrets:

         echo -n "<APP_ID>" | gcloud secrets versions add github-app-id \\
             --project ${PROJECT_ID} --data-file=-
         echo -n "<INSTALLATION_ID>" | gcloud secrets versions add github-installation-id \\
             --project ${PROJECT_ID} --data-file=-
         gcloud secrets versions add github-private-key \\
             --project ${PROJECT_ID} --data-file=./app-private-key.pem

  2. Deploy the Cloud Run Job and Scheduler:

         PROJECT_ID=${PROJECT_ID} REGION=${REGION} \\
             STATE_BUCKET=${STATE_BUCKET} SA_EMAIL=${SA_EMAIL} \\
             bash deploy.sh

EOF
