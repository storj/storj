# Satellite log reviewer

A scheduled Cloud Run Job that reviews the last ~26h of Storj satellite
logs in GCP, clusters recurring problems, and pushes a markdown report
back to this repository under `reports/satellite-logs/`.

## Design in one picture

```
Cloud Scheduler (daily, 06:00 UTC)
        │
        ▼
Cloud Run Job  ──► Cloud Logging API (satellite containers, WARN+)
   │                        │
   │                        ▼
   │               cluster by message signature
   │                        │
   │               ┌────────┴────────┐
   ▼               ▼                 ▼
 GCS state     Vertex AI          GitHub API
(dedup across  (Gemini 3.1 Pro,   (PUT reports/satellite-logs/
 runs, 90d     hypothesis for      YYYY-MM-DD.md on
 retention)    NEW clusters only)  the feature branch)
```

Design choices and why:

- **Cloud Logging severity is unreliable for zap logs** — we take the union of
  `severity>=WARNING` and `jsonPayload.L in {ERROR,WARN,DPANIC,PANIC,FATAL}`
  so an entry that zap emits as ERROR but GCP demotes to WARNING still lands
  in the report.
- **Message-signature clustering**, not raw grouping, so node IDs / segment
  IDs / timestamps do not split what is really one problem into thousands.
- **State in GCS** so the daily run can tell `new` from `ongoing` from
  `went silent`. Signatures unseen for 90 days are evicted.
- **Gemini is only called on NEW clusters** (capped at 15 per run). Ongoing
  clusters are listed as a table without spending tokens.
- **Minimum blast radius**: the worker SA has `roles/logging.viewer`,
  `roles/aiplatform.user`, and GCS object access on its own state bucket.
  No write access to Cloud Logging, no access to the satellite DB, no
  ability to touch GitHub issues or PRs.

## Files

| path | purpose |
|---|---|
| `main.py` | agent entry point |
| `filters.yaml` | query-time noise exclusions (start empty) |
| `suppress.yaml` | post-cluster suppress list by signature |
| `Dockerfile` | container image |
| `deploy/setup-iam.sh` | one-shot IAM / GCS / Secret Manager bootstrap |
| `deploy/deploy.sh` | build image, deploy Cloud Run Job, wire up Scheduler |

Reports land in `reports/satellite-logs/` on the branch configured by
`GITHUB_BRANCH` (default `claude/review-satellite-logs-EGmLz`).

## One-time setup

### 1. GCP IAM, bucket, secrets

```bash
cd tools/log-reviewer/deploy
PROJECT_ID=storj-prod REGION=us-central1 bash setup-iam.sh
```

The script creates:
- Service account `satellite-log-reviewer@storj-prod.iam.gserviceaccount.com`
  with `roles/logging.viewer`, `roles/aiplatform.user`, and bucket access.
- GCS bucket `storj-prod-satellite-log-reviewer-state`.
- Three empty Secret Manager secrets for the GitHub App credentials.

### 2. GitHub App

Create a GitHub App on the `storj` organization.

- **Permissions** (repository): `Contents: Read & write`. Nothing else.
- **Where can this app be installed**: only this organization.
- Generate a private key (`.pem`), save it locally.
- Install the app on the **storj/storj** repository only. Note the
  installation id from the install URL (`.../installations/<ID>`).

Feed the credentials into Secret Manager:

```bash
echo -n "<APP_ID>"           | gcloud secrets versions add github-app-id           --project storj-prod --data-file=-
echo -n "<INSTALLATION_ID>"  | gcloud secrets versions add github-installation-id  --project storj-prod --data-file=-
gcloud secrets versions add github-private-key --project storj-prod --data-file=./app-private-key.pem

# shred the local copy of the private key once it is in Secret Manager
shred -u app-private-key.pem
```

### 3. Deploy

First deploy in dry-run so the first few runs only log what they would
publish without actually writing to GitHub:

```bash
PROJECT_ID=storj-prod REGION=us-central1 \
    STATE_BUCKET=storj-prod-satellite-log-reviewer-state \
    SA_EMAIL=satellite-log-reviewer@storj-prod.iam.gserviceaccount.com \
    DRY_RUN=true \
    bash deploy.sh
```

Trigger it manually to check the output in Cloud Run logs:

```bash
gcloud run jobs execute satellite-log-reviewer \
    --project storj-prod --region us-central1 --wait
```

Once you have reviewed 2–3 dry-run executions and they look sensible,
redeploy with `DRY_RUN=false`.

## Tuning noise

Two knobs, in order of preference:

1. **`filters.yaml`** — Cloud Logging filter exclusions. Cheapest, applied
   before anything is read. Use for high-volume noise whose exclusion rule
   is easy to express (a particular logger, a particular error string).
2. **`suppress.yaml`** — suppress by cluster signature. Use for rarer but
   recurring clusters you have already triaged and decided are not worth
   daily attention. The signature to put here is the 16-char hex id next
   to each cluster in the report.

Both files are baked into the image, so changes require a redeploy
(`bash deploy/deploy.sh`). That is intentional — every tweak is a git
commit you can revert.

## Operational notes

- **Manual run**: `gcloud run jobs execute satellite-log-reviewer ...`.
- **Logs from the reviewer itself**: Cloud Logging, resource
  `cloud_run_job` with name `satellite-log-reviewer`.
- **State inspection**: `gcloud storage cat gs://.../cluster-state.json`.
- **Rollback to a prior image**: `gcloud run jobs update ... --image=...`
  with the prior `:IMAGE_TAG` (Artifact Registry keeps them).

## What this does NOT do

- It does not create GitHub issues or PRs. Only commits report files to
  the configured branch.
- It does not touch the satellite database or any production service.
- It does not modify log data; it only reads.
- It does not auto-tune `filters.yaml` or `suppress.yaml`. All mutations
  happen by human-authored commits.
