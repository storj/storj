#!/usr/bin/env bash
#
# Build + deploy the Cloud Run Job and create/update the Cloud Scheduler
# trigger that runs it daily.
#
# Required env:
#   PROJECT_ID, REGION, STATE_BUCKET, SA_EMAIL
# Optional:
#   JOB_NAME      (default: satellite-log-reviewer)
#   SCHEDULE_CRON (default: "0 6 * * *"   — 06:00 UTC daily)
#   SCHEDULE_TZ   (default: "UTC")
#   DRY_RUN       (default: "false"      — set "true" for first runs)
#   GEMINI_MODEL  (default: "gemini-3.1-pro")
#   IMAGE_TAG     (default: timestamp)

set -euo pipefail

: "${PROJECT_ID:?set PROJECT_ID}"
: "${REGION:?set REGION}"
: "${STATE_BUCKET:?set STATE_BUCKET}"
: "${SA_EMAIL:?set SA_EMAIL}"

JOB_NAME="${JOB_NAME:-satellite-log-reviewer}"
SCHEDULE_CRON="${SCHEDULE_CRON:-0 6 * * *}"
SCHEDULE_TZ="${SCHEDULE_TZ:-UTC}"
DRY_RUN="${DRY_RUN:-false}"
GEMINI_MODEL="${GEMINI_MODEL:-gemini-3.1-pro}"
IMAGE_TAG="${IMAGE_TAG:-$(date -u +%Y%m%d-%H%M%S)}"

REPO="satellite-log-reviewer"
IMAGE="${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/agent:${IMAGE_TAG}"

cd "$(dirname "$0")/.."

echo ">> ensuring Artifact Registry repo exists"
if ! gcloud artifacts repositories describe "${REPO}" \
        --location "${REGION}" --project "${PROJECT_ID}" >/dev/null 2>&1; then
    gcloud artifacts repositories create "${REPO}" \
        --repository-format=docker \
        --location="${REGION}" \
        --project="${PROJECT_ID}"
fi

echo ">> building image via Cloud Build: ${IMAGE}"
gcloud builds submit \
    --project="${PROJECT_ID}" \
    --tag="${IMAGE}" \
    .

ENV_VARS=$(cat <<EOF
GCP_PROJECT=${PROJECT_ID},GCP_REGION=${REGION},STATE_BUCKET=${STATE_BUCKET},GEMINI_MODEL=${GEMINI_MODEL},DRY_RUN=${DRY_RUN}
EOF
)

SECRETS=$(cat <<EOF
GITHUB_APP_ID=github-app-id:latest,GITHUB_INSTALLATION_ID=github-installation-id:latest,GITHUB_PRIVATE_KEY=github-private-key:latest
EOF
)

echo ">> deploying Cloud Run Job: ${JOB_NAME}"
if gcloud run jobs describe "${JOB_NAME}" \
        --region "${REGION}" --project "${PROJECT_ID}" >/dev/null 2>&1; then
    gcloud run jobs update "${JOB_NAME}" \
        --project="${PROJECT_ID}" \
        --region="${REGION}" \
        --image="${IMAGE}" \
        --service-account="${SA_EMAIL}" \
        --max-retries=1 \
        --task-timeout=1800 \
        --cpu=1 --memory=1Gi \
        --set-env-vars="${ENV_VARS}" \
        --set-secrets="${SECRETS}"
else
    gcloud run jobs create "${JOB_NAME}" \
        --project="${PROJECT_ID}" \
        --region="${REGION}" \
        --image="${IMAGE}" \
        --service-account="${SA_EMAIL}" \
        --max-retries=1 \
        --task-timeout=1800 \
        --cpu=1 --memory=1Gi \
        --set-env-vars="${ENV_VARS}" \
        --set-secrets="${SECRETS}"
fi

SCHEDULER_NAME="${JOB_NAME}-daily"
SCHEDULER_URI="https://${REGION}-run.googleapis.com/apis/run.googleapis.com/v1/namespaces/${PROJECT_ID}/jobs/${JOB_NAME}:run"

# Dedicated invoker SA for the scheduler so we do not reuse the worker SA.
INVOKER_SA="${JOB_NAME}-invoker@${PROJECT_ID}.iam.gserviceaccount.com"
if ! gcloud iam service-accounts describe "${INVOKER_SA}" --project "${PROJECT_ID}" >/dev/null 2>&1; then
    gcloud iam service-accounts create "${JOB_NAME}-invoker" \
        --project="${PROJECT_ID}" \
        --display-name="Invoker for ${JOB_NAME}"
fi
gcloud run jobs add-iam-policy-binding "${JOB_NAME}" \
    --project="${PROJECT_ID}" \
    --region="${REGION}" \
    --member="serviceAccount:${INVOKER_SA}" \
    --role="roles/run.invoker" >/dev/null

echo ">> creating/updating Cloud Scheduler: ${SCHEDULER_NAME}"
if gcloud scheduler jobs describe "${SCHEDULER_NAME}" \
        --location "${REGION}" --project "${PROJECT_ID}" >/dev/null 2>&1; then
    gcloud scheduler jobs update http "${SCHEDULER_NAME}" \
        --project="${PROJECT_ID}" \
        --location="${REGION}" \
        --schedule="${SCHEDULE_CRON}" \
        --time-zone="${SCHEDULE_TZ}" \
        --uri="${SCHEDULER_URI}" \
        --http-method=POST \
        --oauth-service-account-email="${INVOKER_SA}"
else
    gcloud scheduler jobs create http "${SCHEDULER_NAME}" \
        --project="${PROJECT_ID}" \
        --location="${REGION}" \
        --schedule="${SCHEDULE_CRON}" \
        --time-zone="${SCHEDULE_TZ}" \
        --uri="${SCHEDULER_URI}" \
        --http-method=POST \
        --oauth-service-account-email="${INVOKER_SA}"
fi

cat <<EOF

Deployed.

  Image:        ${IMAGE}
  Job:          ${JOB_NAME} (region ${REGION})
  Schedule:     ${SCHEDULE_CRON} (${SCHEDULE_TZ})
  DRY_RUN:      ${DRY_RUN}

To trigger a run manually:
  gcloud run jobs execute ${JOB_NAME} --project ${PROJECT_ID} --region ${REGION}

To flip out of dry-run later:
  DRY_RUN=false PROJECT_ID=${PROJECT_ID} REGION=${REGION} \\
      STATE_BUCKET=${STATE_BUCKET} SA_EMAIL=${SA_EMAIL} bash deploy.sh

EOF
