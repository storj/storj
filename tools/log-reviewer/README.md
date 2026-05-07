# Satellite log reviewer

A scheduled job that reads Storj satellite error logs every day, figures
out what is broken, and files **one GitHub issue per bug** in
`storj/qa-storj` so engineers can triage and fix.

This README is written so that someone who has never seen the codebase
can understand both **what** it does and **why** every piece exists.
Pour yourself a coffee — it's long but everything matters.

---

## 1. The problem this solves

Storj satellites emit *a lot* of error logs. On a normal day you'll see
~50 000 WARNING / ERROR / PANIC lines across the satellite pods. A
human cannot read all of them, and a dashboard with raw counts is
useless because most of those lines are noise:

- The same error fires once per request, so 5 000 identical lines mean
  one bug, not 5 000 bugs.
- Stack traces look slightly different per call (different goroutine
  paths, different user IDs in the wrapped error), so a naive
  string-match doesn't dedup them.
- Some "errors" are not actually bugs (`context canceled` happens
  whenever a user disconnects mid-request — that's life, not a problem).

What we want, in plain words:

> Every morning, give me a short list of "this is broken, here's what
> I think is wrong, here are the next steps." One ticket per bug. No
> noise. If the same bug recurs day after day, don't keep spamming —
> just keep the ticket open.

That's what this tool does.

---

## 2. The big picture

```
                                  Cloud Scheduler
                                  (daily 06:00 UTC)
                                         │
                                         ▼
                           ┌────────────────────────────┐
                           │  Cloud Run Job (this code) │
                           └────────────────────────────┘
                                         │
        ┌────────────────────────────────┼─────────────────────────────────┐
        ▼                                ▼                                 ▼
 ┌─────────────┐                 ┌──────────────┐                  ┌──────────────┐
 │ Cloud       │ pull last 26h   │ Vertex AI /  │ ask for          │ GitHub API   │
 │ Logging     │ of WARN+ logs   │ Gemini       │ hypothesis +     │ open issues  │
 │ (read-only) │ ◄────────────── │ (gemini-2.5- │ next steps for   │ + push daily │
 └─────────────┘                 │  flash)      │ NEW bug groups   │ report .md   │
                                 └──────────────┘                  └──────────────┘
                                         │
                                         ▼
                                 ┌──────────────┐
                                 │ GCS bucket   │ remembers what we saw,
                                 │ cluster-     │ how often, and what
                                 │ state.json   │ Gemini said about it
                                 └──────────────┘
```

A single daily run does this top to bottom:

1. **Read** the last 26 hours of satellite logs from Cloud Logging.
2. **Cluster** lines that are essentially the same error (different
   user IDs / timestamps / hash IDs collapse into one bucket).
3. **Filter out** clusters that match a known-benign pattern (we know
   they're noise).
4. **Group** clusters that share a root cause into "bug groups" — one
   bug = one ticket, even if it surfaces through several log call sites.
5. **Ask Gemini** to write a hypothesis and next steps for new bug
   groups. (Cached for previously-seen ones — saves money.)
6. **Decide** which bug groups deserve a GitHub ticket today (burst or
   recurrence — explained below).
7. **Render** a daily markdown report with all findings.
8. **Push** the report to a branch in `storj/qa-storj` and file new
   GitHub issues.
9. **Save state** back to GCS so the next run remembers everything.

---

## 3. Three layers of identity

This is the most important concept in the whole tool. Read it twice.

```
  Log entry              "context canceled" + stack trace + user_id=U12345
  Log entry              "context canceled" + stack trace + user_id=U67890
  Log entry              "context canceled" + stack trace + user_id=U13579
        │
        │  many → one (normalize_for_signature replaces UUIDs, IPs, timestamps,
        │             hashes, base64 with placeholders, then SHA-1)
        ▼
  Cluster   sig=9ca54019…  count=3  message_template="context canceled"
  Cluster   sig=8816874c…  count=20 message_template="failed to update reputation\nstorj.io/storj/satellite/audit/..."
  Cluster   sig=bee08720…  count=15 message_template="error(s) during audit\nstorj.io/storj/satellite/audit/..."
        │
        │  several → one (bug_group_key uses subsystem + first prose line)
        ▼
  Bug group  hash=bce20ed7  subsystem=audit   "failed to update reputation"   2 clusters
  Bug group  hash=6ca19def  subsystem=audit   "error(s) during audit"         1 cluster
  Bug group  hash=…         subsystem=…       …
        │
        │  one bug group = one GitHub ticket
        ▼
  GitHub issue #693 with label log-bug:bce20ed7
```

### Layer 1: Log entry → Cluster

Many raw log entries collapse into one cluster if they describe the
same thing. The signature ignores anything that *varies between calls*
of the same code: user IDs, timestamps, IP addresses, request IDs,
long hex hashes, base64 tokens, line numbers in stack frames.

Code: `entry_signature()` and `normalize_for_signature()` in `main.py`.

### Layer 2: Cluster → Bug group

Sometimes the same root cause produces *multiple* clusters because
it surfaces through different log call sites. For example, a Spanner
outage makes 4 different `metabase/changestream` functions log
errors. Filing 4 tickets for one outage is annoying, so we group
clusters by `(subsystem, first prose line of the message)`. Different
prose lines = different bugs. Same prose lines + same subsystem = same
bug group → one ticket.

Code: `bug_group_key()` and `group_clusters()` in `main.py`.

### Layer 3: Bug group → GitHub issue

One bug group becomes at most one open GitHub issue. The dedup label
is `log-bug:<8-char hex>`. We also tag every constituent cluster with
`log-cluster:<8-char hex>` for cross-search.

If an issue already exists with the matching `log-bug:` label (open or
closed within the last 7 days), we skip — no spam. As a safety net the
dedup also probes `log-cluster:` labels, in case an older version of
the bot filed an issue under the old label-only scheme.

Code: `github_create_issue()` and `github_issue_exists()` in `main.py`.

---

## 4. Step by step: what happens during one daily run

### Step 1 — Build the Cloud Logging filter

We can't read every log line in the project; we need a filter. The
filter says "satellite containers, severity at least WARNING, time
window the last 26 hours, minus a few cheap exclusions from
`filters.yaml`."

Why 26 hours, not 24? Because a 24-hour window run at 06:00 UTC could
miss a log emitted at 05:59 UTC if there's any clock skew or ingestion
delay. 26 hours has a 2-hour overlap with yesterday's window, which the
state file dedups across runs.

Code: `build_filter()` in `main.py`.

### Step 2 — Pull logs

Stream entries from the Cloud Logging API in pages of 1000 with a
short pause between pages so we stay under the quota. There's a hard
cap (`MAX_ENTRIES`, default 50 000) so a chatty window can't make the
job time out.

Code: `fetch_logs()` in `main.py`.

### Step 3 — Cluster the entries

For each entry, compute a normalized signature (`logger | level |
normalize(message)`) and bucket entries with the same signature. Each
bucket is a `Cluster` — it carries `count`, a few sample payloads,
the timestamps of first/last sighting, etc.

Why `level` is in the signature: an ERROR-level "context canceled" is
a different concern than a DEBUG-level one, even if the message is
identical.

Why `error` (the wrapped error string) is **not** in the signature
anymore: it varies per call (user IDs, partition tokens, DB error
detail) but doesn't change the bug. We dropped it after we noticed
that the same code site produced two separate clusters just because
the wrapped error text differed.

Code: `entry_signature()`, `cluster_entries()`.

### Step 4 — Match known-benign patterns

Some errors are noise we already understand. They live in a YAML
block at the bottom of `context.md` (between `<!-- known_benign:` and
`-->`):

```yaml
- pattern: "context canceled"
  reason: "client/RPC cancellation; expected with churn"
- pattern: "Monthly bandwidth limit exceeded"
  reason: "user hit quota; not an incident"
- pattern: "failed to get product for ID"
  reason: "false-positive ERROR from stripe.GetPlacementPriceModel default fallback"
…
```

If a cluster's `message_template` contains one of these substrings, it
gets routed to the **Known-benign / suppressed** section of the daily
report and is **not** considered for ticket creation.

Code: `load_known_benign()` reads the YAML, `match_known_benign()`
does the substring check.

### Step 5 — Group clusters into bug groups

For every actionable cluster (not benign, not in `suppress.yaml`),
compute its `bug_group_key`:

- If the message has a meaningful prose first line (e.g. `"Could not
  get freeze status"`), the key is `subsystem | normalized_prose`.
- If the message is a pure stack trace (e.g.
  `storj.io/storj/satellite/metabase/changestream.ReadPartition:180`),
  the key is `subsystem | <frames> | terminal_pkg` so different
  packages within the same subsystem don't accidentally merge.

Take SHA-1, keep first 8 hex chars → that's the **bug group hash**.
This is what becomes the `log-bug:<hash>` label on the GitHub issue.

Code: `bug_group_key()`, `group_clusters()`.

### Step 6 — Ask Gemini for hypotheses (only on new clusters)

For each cluster that is **new** in this run (signature not in the
state from previous runs), we send Gemini:

- The codebase context (the entirety of `context.md`)
- The cluster's logger, level, message template, sample log entries
- A prompt asking for: a one-line summary, an urgency tag, a 2-4
  sentence hypothesis, and 2-3 concrete next steps

Gemini returns JSON. We cache the answer in state so we don't pay
again for the same cluster on subsequent days.

Why only new clusters? Cost. Sending all 30+ clusters to Gemini every
day would be wasteful when most are unchanged from yesterday.

Why we send `context.md`: it tells Gemini about Storj's subsystems,
which errors are known-benign vs known-serious, common cascade
patterns. The hypotheses go from generic ("check database health") to
specific ("`accountfreeze` chore lookup at line 200 — DB is unreachable
or row contention; spike >50/day = suspect connectivity").

Code: `_Analyzer` class with consecutive-failure circuit breaker. If
Gemini fails 3 times in a row, the run aborts so we don't ship stub
analyses and accidentally close real bugs.

### Step 7 — Decide which bug groups get a ticket today

This is the **burst-or-recurrence** check. A bug group qualifies if
**either**:

- **Burst**: combined count this run ≥ `BURST_THRESHOLD` (default 5).
  The error is happening *a lot right now*, file a ticket immediately
  even if it's the first time we see it.
- **Recurrence**: at least one constituent cluster has been observed
  in `RECURRENCE_RUNS` or more distinct prior runs (default 2). It's
  rare but it keeps coming back, so it's a real bug, not a one-off
  blip.

Why both? Different shapes of bug:

- A bug that fires 100 times in one window is loud → ticket now.
- A bug that fires once a day for a week is quiet but recurring → also
  a ticket, just on day 2 instead of day 1.
- A flaky network blip that fires once and never again → no ticket.
  It saves human attention.

`run_count` is tracked in the state file (per signature) and bumped
once per run when the signature appears.

Already-existing tickets: `github_issue_exists` looks up
`log-bug:<hash>` in the last 7 days (open or recently closed) — found?
skip. Otherwise, file.

A `MAX_ISSUES_PER_RUN` cap (default 20) keeps a runaway-bug day from
producing a torrent of tickets.

Code: `open_issues()`, `github_issue_exists()`.

### Step 8 — Render the daily report

A single markdown file `reports/satellite-logs/YYYY-MM-DD.md`. Sections:

- **TL;DR** — counts per category, top 3 subsystems by error volume,
  number of bug groups vs clusters
- **Top issues** — full Gemini analysis for each high-count cluster
  (count ≥ 5)
- **By bug group** — table showing how multi-cluster groups merged
- **By subsystem** — every cluster in this run, grouped by subsystem
  (audit, metainfo, payments, …)
- **Known-benign / suppressed** — collapsed list of clusters we
  filtered out, with the reason

Code: `render_report()`.

### Step 9 — Publish

- **Report** is committed to the configured branch (default
  `satellite-log-reports`) in the configured repo (default
  `storj/qa-storj`).
- **GitHub issues** are filed for qualifying bug groups.
- **State** (the JSON file in GCS) is updated and saved last, so a
  failure in the publish step doesn't lose what we learned this run.

---

## 5. The state file — why it's the heart of the tool

```
gs://storj-prod-satellite-log-reviewer-state/cluster-state.json
```

It looks like:

```json
{
  "last_run": "2026-05-07T08:20:38Z",
  "clusters": {
    "8816874c3cdcbe2b": {
      "signature": "8816874c3cdcbe2b",
      "logger": "<unknown-logger>",
      "level": "ERROR",
      "message_template": "failed to update reputation information…",
      "first_seen_ever": "2026-04-26T15:57:17Z",
      "last_seen_ever":  "2026-05-07T03:29:31Z",
      "total_count": 266,
      "run_count": 12,
      "analysis": {
        "summary": "Failed to update node reputation after audit results.",
        "urgency": "medium",
        "hypothesis": "...",
        "next_steps": ["...", "..."],
        "analyzed_at": "2026-04-29T07:30:42Z"
      }
    },
    …
  }
}
```

Why each field matters:

- `total_count` — lifetime occurrence counter across all runs.
- `run_count` — how many distinct daily runs have observed this
  signature. Drives the recurrence threshold.
- `first_seen_ever` / `last_seen_ever` — for the report's "first seen"
  line so SREs can tell new bugs from veterans.
- `analysis` — cached Gemini hypothesis. Reused on subsequent runs
  unless the cluster signature changes.

A 90-day eviction policy drops anything we haven't seen in that long.

---

## 6. Configuration (environment variables)

| Variable | Default | What it does |
|---|---|---|
| `GCP_PROJECT` | (required) | GCP project to read logs from |
| `GCP_REGION` | `us-central1` | Vertex AI region |
| `STATE_BUCKET` | (required) | GCS bucket for the state file |
| `GEMINI_MODEL` | `gemini-2.5-flash` | Vertex AI model id |
| `WINDOW_HOURS` | `26` | How far back to read logs |
| `MAX_ENTRIES` | `50000` | Hard cap on entries read per run |
| `MAX_ENTRIES_PER_CLUSTER` | `20` | Sample cap per cluster |
| `MAX_NEW_CLUSTERS_TO_ANALYZE` | `15` | Gemini cost cap per run |
| `BURST_THRESHOLD` | `5` | Combined count needed to file immediately |
| `RECURRENCE_RUNS` | `2` | Distinct runs needed to file recurring bugs |
| `ISSUE_THRESHOLD` | `1` | Hard floor on combined count |
| `MAX_ISSUES_PER_RUN` | `20` | Rate-limit on tickets per run |
| `READ_SLEEP_S` | `1.5` | Pause between Cloud Logging pages |
| `STATE_RESET` | `false` | If `true`, treat all clusters as "new" for classification (keeps run_count history) |
| `DRY_RUN` | `false` | If `true`, write the report to `gs://…/dry-run/` instead of GitHub and don't open issues |
| `GITHUB_REPO` | `storj/qa-storj` | Where reports + issues land |
| `GITHUB_BRANCH` | `satellite-log-reports` | Branch for daily reports |
| `REPORT_DIR` | `reports/satellite-logs` | Folder inside the repo |
| `GITHUB_APP_ID` | (required) | GitHub App credentials |
| `GITHUB_INSTALLATION_ID` | (required) | |
| `GITHUB_PRIVATE_KEY` | (required) | |

---

## 7. The known-benign list — how to add a new pattern

When you find a cluster that's noise but the bot keeps filing tickets
for it, edit `context.md`. Add an entry to the YAML block at the
bottom:

```yaml
- pattern: "your substring here"
  reason: "short explanation; why benign; spike threshold if any"
```

The `pattern` is a substring match (regex-escaped). It runs against
the cluster's `message_template`. Any cluster whose template contains
the pattern gets routed to the suppressed section and never fires a
ticket.

Tips:

- Keep patterns specific enough to avoid false positives. `error` is
  too broad; `failed to get product for ID` is fine.
- Add a one-line `reason` that's actually informative — Gemini reads
  this when analyzing related clusters.
- For errors that should only be benign at low rates, use the
  "Severity-thresholded errors" section in `context.md` instead. That
  section keeps them in Top issues but tells the on-call when to act.

After editing, redeploy: `bash deploy/deploy.sh`. The change ships
with the next image.

---

## 8. File layout

```
tools/log-reviewer/
├── main.py            # the agent, ~1500 lines
├── test_main.py       # 58 unit tests for pure functions
├── context.md         # subsystem descriptions, known-benign YAML, runbook
├── filters.yaml       # query-time exclusions (cheap, applied in Cloud Logging)
├── suppress.yaml      # signature-list to suppress (rarely-used escape hatch)
├── requirements.txt   # python deps
├── Dockerfile         # builds the Cloud Run image
├── README.md          # this file
└── deploy/
    ├── setup-iam.sh   # one-shot bootstrap of SA, GCS, Secret Manager
    └── deploy.sh      # build + push image, deploy Cloud Run job, wire scheduler
```

---

## 9. Setup (one-time)

### 9.1 GCP IAM, bucket, secrets

```bash
cd tools/log-reviewer/deploy
PROJECT_ID=storj-prod REGION=us-central1 bash setup-iam.sh
```

Creates:

- Service account `satellite-log-reviewer@storj-prod.iam.gserviceaccount.com`
  with `roles/logging.viewer`, `roles/aiplatform.user`, and bucket
  access on its own state bucket.
- GCS bucket `storj-prod-satellite-log-reviewer-state`.
- Three empty Secret Manager secrets for the GitHub App credentials.

### 9.2 GitHub App

Create a GitHub App on the org. Grant it:

- **Contents**: Read & write (to commit the report file)
- **Issues**: Read & write (to file tickets)
- **Metadata**: Read (required base permission)

Install the app on the **storj/qa-storj** repository. Note the
`installation_id` from the install URL.

Push credentials into Secret Manager:

```bash
echo -n "<APP_ID>"           | gcloud secrets versions add github-app-id           --data-file=-
echo -n "<INSTALLATION_ID>"  | gcloud secrets versions add github-installation-id  --data-file=-
gcloud secrets versions add github-private-key --data-file=./app-private-key.pem
shred -u app-private-key.pem
```

### 9.3 Deploy

Start with dry-run so the first runs publish to GCS instead of GitHub:

```bash
PROJECT_ID=storj-prod REGION=us-central1 \
    STATE_BUCKET=storj-prod-satellite-log-reviewer-state \
    SA_EMAIL=satellite-log-reviewer@storj-prod.iam.gserviceaccount.com \
    DRY_RUN=true \
    bash deploy/deploy.sh
```

Trigger manually and inspect:

```bash
gcloud run jobs execute satellite-log-reviewer \
    --project storj-prod --region us-central1 --wait

gsutil cat gs://storj-prod-satellite-log-reviewer-state/dry-run/$(date -u +%F).md | less
```

Once 2–3 dry-run reports look right, redeploy with `DRY_RUN=false`.

---

## 10. Daily operations

### Manual run
```bash
gcloud run jobs execute satellite-log-reviewer \
    --project storj-prod --region us-central1 --wait
```

### Read the bot's own logs
```bash
gcloud logging read \
  'resource.type=cloud_run_job AND resource.labels.job_name=satellite-log-reviewer' \
  --project storj-prod --limit 50 --format "value(textPayload)" --order asc
```

### Inspect the state file
```bash
gsutil cat gs://storj-prod-satellite-log-reviewer-state/cluster-state.json | jq '.clusters | length'
```

### Pause / resume the daily cron
```bash
gcloud scheduler jobs pause  satellite-log-reviewer-daily --location us-central1 --project storj-prod
gcloud scheduler jobs resume satellite-log-reviewer-daily --location us-central1 --project storj-prod
```

### Roll back to a prior image
```bash
# list tags
gcloud artifacts docker images list \
  us-central1-docker.pkg.dev/storj-prod/satellite-log-reviewer/agent --include-tags

# pin
gcloud run jobs update satellite-log-reviewer \
  --project storj-prod --region us-central1 \
  --image us-central1-docker.pkg.dev/.../agent:<OLDER_TAG>
```

---

## 11. Deploy-time runbook (important)

Whenever you change anything that affects cluster signatures, bug-group
keys, or the `log-bug:` / `log-cluster:` label format, follow this
order so the daily cron doesn't file a duplicate batch in the gap:

1. **Pause the scheduler** before touching code.
2. **Bulk-close existing auto-issues** with a "superseded" comment.
3. **Reset the GCS state file** so the next run is a clean
   classification (`gsutil rm gs://…/cluster-state.json`).
4. **Deploy the new image**, trigger one manual run, sanity-check the
   output and the GitHub issues.
5. **Resume the scheduler.**

Skipping any step risks orphan issues with stale labels that the new
dedup logic can't see.

---

## 12. Cost (rough)

For a satellite project with ~50 000 WARN+ log lines per day:

- Cloud Logging reads: ~$0–2/month (most projects under the free tier)
- Vertex AI / Gemini 2.5 Flash: ~$1–5/month (we cap at 15 fresh calls
  per run, ~$0.005 each)
- Cloud Run execution time: cents
- GCS state file: cents

Total: < $10/month. Cheap for what it does.

---

## 13. What this tool does NOT do

- Does **not** modify log data; reads only.
- Does **not** touch the satellite database or any production service.
- Does **not** auto-tune `filters.yaml`, `suppress.yaml`, or
  `context.md`. All mutations happen by human-authored commits, so you
  always have a git history of why a noise pattern was added.
- Does **not** auto-close GitHub issues. A human triages them.
- Does **not** ping people or send chat alerts. Issues are the only
  output channel.

---

## 14. FAQ

**Q: Why bug groups *and* clusters? Why not just one?**

A cluster is "exact same log line." A bug group is "same code site
that's broken." A bug can surface through several cluster signatures
(e.g. one outage triggers four different log statements at four call
layers). Filing four tickets for one outage is annoying; filing one
ticket that lists the four clusters is exactly what an SRE wants.

**Q: Why does the bot wait two runs before opening a ticket for a
recurring error?**

Because a single error happening once doesn't deserve a ticket — it
might be a flaky network blip. We want signal, not noise. If the
error reappears in a second run, it's clearly recurring → ticket.
If it happens 5+ times in one run, that's already loud enough → ticket
right away.

**Q: How does the bot know about Storj subsystems?**

`context.md` lists every relevant subsystem with its Go package
prefix. `subsystem_for_logger()` matches the logger name (or the first
storj.io frame in the message) against that table. Gemini also reads
the whole `context.md`, so its hypotheses cite the right subsystem
and reference the right code paths.

**Q: What if the same bug shows up but with a slightly different
signature tomorrow (e.g. someone refactored the code)?**

The cluster signature changes, so the `log-cluster:<sig8>` label
changes too. The dual-label dedup probe (`github_issue_exists`) also
checks the **bug group hash** (`log-bug:<hash>`), which is much more
stable because it ignores stack-frame line numbers and the wrapped
error text. A small refactor won't fool it.

**Q: Why do I see `Subsystem: unknown` on some issues?**

Most satellite call sites don't call `logger.Named("foo")`, so the
zap logger field is empty. We try to derive the subsystem from the
first `storj.io/storj/...` frame in the stack trace. If there's no
storj frame at all (e.g. a third-party library logging on its own),
we fall back to `unknown`.

**Q: Where does the daily report end up?**

`https://github.com/storj/qa-storj/blob/satellite-log-reports/reports/satellite-logs/YYYY-MM-DD.md`

**Q: Where do the issues go?**

`https://github.com/storj/qa-storj/issues?q=is%3Aissue+label%3Asatellite-log`

**Q: How do I know the bot didn't open a duplicate?**

Search by the `log-bug:<hash>` label. Each open + recently-closed
issue with that label is the same bug. There should be at most one
open per hash.
