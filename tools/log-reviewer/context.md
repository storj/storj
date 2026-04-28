# Storj Satellite — AI Triage Context

## Architecture overview

The satellite is a modular monolith. Logs come from several Kubernetes containers:
- `satellite` — main process (metainfo gRPC API, overlay, orders, audit, repair, GC)
- `satellite-api` — HTTP console API and web frontend backend
- `satellite-admin` — internal admin API
- `satellite-gc` — garbage collection sender
- `satellite-repair` — repair worker

## Key subsystems and their Go package paths

| Logger prefix | Subsystem | Notes |
|---|---|---|
| `storj.io/storj/satellite/metainfo` | Object metadata API | gRPC upload/download/delete |
| `storj.io/storj/satellite/overlay` | Node reputation | Node selection, churn |
| `storj.io/storj/satellite/repair` | Repair checker/worker | Under-replicated segments |
| `storj.io/storj/satellite/audit` | Audit verifier | Verifies nodes store data |
| `storj.io/storj/satellite/accounting` | Usage accounting | Tally, rollup, bandwidth |
| `storj.io/storj/satellite/payments` | Stripe billing | Invoices, charges, subscriptions |
| `storj.io/storj/satellite/console` | Web console | User/project/API key management |
| `storj.io/storj/satellite/gc` | Garbage collection | Bloom filter, piece deletion |
| `storj.io/storj/satellite/nodeevents` | Node lifecycle | Node online/offline events |
| `storj.io/storj/satellite/gracefulexit` | Graceful exit | Node departure protocol |

## Known benign / expected errors

These are expected at low rates and do not require investigation:
- `context canceled` — client disconnected mid-request; normal
- `connection reset by peer` — normal storage node network churn
- `rpc error: code = Canceled` — same as context canceled
- `Monthly bandwidth limit exceeded` / `Storage limit exceeded` / `Segment limit exceeded` — user hitting quota; filtered at source
- `Ignoring invoice; account has non-Paid kind` — free-tier accounts skipped during billing; expected
- `invalid provider favicon.ico` — browser favicon request hitting SSO endpoint; cosmetic
- `service takes long to shutdown` — during rolling deploys; transient

## Known serious errors (investigate immediately)

- Redis `connection pool: failed to dial` — Redis/Dragonfly unreachable; if count >100 in a day, check Redis cluster health
- `ranged loop failure` — ranged loop observer crashed; affects repair, GC, accounting accuracy
- `error archiving SN and bucket bandwidth rollups` — accounting data loss risk; check DB connectivity
- `Could not get freeze status` — account freeze chore can't reach its backend; users may be overbilled
- `too many open files` — file descriptor leak on satellite pod
- `audit failed` spike — nodes going offline or serving bad data
- `failed to record deletion remainder charge` — Stripe charge not recorded; revenue impact

## Common error patterns and root causes

### `ExceedsUploadLimits` / `error while getting storage/segments usage`
Package: `satellite/metainfo`. Happens when usage cache (Redis) is stale or unreachable. Usually cascades from a Redis outage. Check Redis first.

### `Could not track new project's storage and segment usage`
Package: `satellite/metainfo`. `addToUploadLimits` calls Redis to update usage counters. Failure means usage tracking is broken for that upload. Correlates with Redis errors.

### `ranged loop failure`
Package: `satellite/metabase/rangedloop`. The ranged loop drives repair checker, GC, and tally. A crash here means segments go unchecked. Check for DB connection errors in surrounding context.

### `error archiving ... bandwidth rollups`
Package: `satellite/accounting`. Rollup archival writes bandwidth stats for node payouts. Failure here leads to incorrect payout calculations. Check CockroachDB/Spanner health.

### `failed to record deletion remainder charge`
Package: `satellite/payments`. When an object is deleted mid-billing-period, a prorated charge is attempted. Failure means that charge is lost. Check Stripe API status and error details.

### `Sending hubspot event` errors
Package: `satellite/analytics`. HubSpot API call failed. Low business impact; retry logic handles most cases.

### `unable to delete zombie objects and segments`
Package: `satellite/metabase`. Zombie objects are uploads that never completed. GC cleanup failed. Usually a DB issue.

## Infrastructure notes

- Redis/Dragonfly: used for usage counters and rate limiting; outages cascade into many `metainfo` errors
- CockroachDB/Spanner: primary metadata store; connection errors affect all subsystems
- Stripe: external billing API; errors here are revenue-impacting
- Vertex AI / Gemini: used for this analysis; not part of satellite
