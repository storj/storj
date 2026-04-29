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
- `too many open files` — file descriptor leak on satellite pod
- `failed to record deletion remainder charge` — Stripe charge not recorded; revenue impact

## Severity-thresholded errors (benign at low rate, investigate on spike)

These ERROR-level logs come from code paths that retry or fall back gracefully. At low daily counts they are normal background noise from the decentralized network. Investigate only when the count exceeds the listed threshold or when correlated with infrastructure alerts.

| Pattern | Source | Spike threshold | Why low rate is OK |
|---|---|---|---|
| `failed to update reputation information with audit results` | `satellite/audit/reporter.go` | >100/day | Logged after retries are exhausted on a per-node reputation write. Audit data itself is preserved; only the reputation update for some nodes failed. Common with node deletions or transient row contention. |
| `error(s) during audit` | `satellite/audit/worker.go` | >100/day | Per-segment verification noise (RPC timeout, bad piece, network drop to a specific node). `RecordAudits` is called with the partial report regardless, so successful audits still persist. |
| `Could not get freeze status` | `satellite/accountfreeze/billingfreezechore.go` | >50/day | Per-user `freezeService.GetAll` lookup failed; chore continues to next invoice. At >50/day suspect DB connectivity. |
| `process` (with audit/worker.go in stack) | `satellite/audit/worker.go:74` | >50/day | Outer wrapper that fires when `Worker.process()` returns any error. Same root cause family as `error(s) during audit` and `failed to update reputation` — counts double-count audit-iteration failures. Returns nil so the loop continues. |
| `error retrieving payments` | `satellite/payments/storjscan/chore.go:70` | >20/day | RPC to the storjscan ETH indexer failed. Chore returns nil and tries again next cycle; user payments are not lost — they're picked up on the next successful tick. Spike means storjscan service is down. |

## Common error patterns and root causes

### `failed to get product for ID` (false-positive ERROR)
Package: `satellite/payments/stripe`. Logged at [service.go:689](satellite/payments/stripe/service.go) every time `GetPlacementPriceModel` falls back to its default product (productID=0, no `ProductName`) — which is the **expected** path for any project whose placement is not in `PlacementProductMap`. The code immediately falls back to `"Product 0"` and the invoice generates correctly. This is a logging-severity bug, not a billing bug; suppress until the satellite team downgrades the call to Debug or populates a default `ProductName`.

### `internal error` from `metainfo/endpoint.go:590` (catch-all)
Package: `satellite/metainfo`. The `default:` branch of `ConvertKnownErrWithMessage` — fires when the metabase returned an error class that isn't in the explicit switch (ObjectNotFound, Canceled, ObjectLock, etc.). Returns `rpcstatus.Internal` to the uplink. **Not benign**: each occurrence represents an unmapped error class worth investigating because we're returning an opaque 500 to the user. The cluster signature may include the underlying error text — read each sample individually rather than treating the cluster as one issue.

### `Failed to get bucket notification config, failing safe` (intentional fallback)
Package: `satellite/metainfo/eventing.go:26`. WARNING-level. When both the bucket-eventing cache and the database lookup fail, the code logs and returns `TransmitEvent=true` — letting the eventing service decide downstream. Comment in source says "Fail-safe mode". Always benign.

### `Error enqueueing message` (3rd-party analytics)
Package: `satellite/analytics`. `service.segment.Enqueue(message)` failed — usually because the Segment.io client queue is full or the upstream API returned an error. Affects telemetry only; user actions still complete. Same family as `Sending hubspot event`.

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

<!-- known_benign:
- pattern: "context canceled"
  reason: "client/RPC cancellation; expected with churn"
- pattern: "rpc error: code = Canceled"
  reason: "same as context canceled"
- pattern: "connection reset by peer"
  reason: "storage node network churn; expected"
- pattern: "Monthly bandwidth limit exceeded"
  reason: "user hit quota; not an incident"
- pattern: "Storage limit exceeded"
  reason: "user hit quota; not an incident"
- pattern: "Segment limit exceeded"
  reason: "user hit quota; not an incident"
- pattern: "Ignoring invoice; account has non-Paid kind"
  reason: "free-tier accounts skipped during billing; expected"
- pattern: "invalid provider favicon.ico"
  reason: "browser favicon hitting SSO; cosmetic"
- pattern: "service takes long to shutdown"
  reason: "rolling deploys; transient"
- pattern: "error while getting storage/segments usage"
  reason: "Redis cache stale/unreachable cascade; investigate Redis only if sustained"
- pattern: "ExceedsUploadLimits"
  reason: "Redis usage cache stale; cascades from infra, not a code bug"
- pattern: "Could not track new project's storage and segment usage"
  reason: "Redis usage write failed; cascades from infra"
- pattern: "superfluous response.WriteHeader call"
  reason: "Go HTTP framework noise; not a bug worth paging on"
- pattern: "returning error to client"
  reason: "user-visible 4xx/5xx; investigate only with corroborating signal"
- pattern: "Sending hubspot event"
  reason: "third-party analytics; retry covers it"
- pattern: "send request failed"
  reason: "external API call failed (Plausible/HubSpot); transient network or service issue"
- pattern: "failed to send pageview event to plausible"
  reason: "Plausible analytics service unreachable or outage; low business impact"
- pattern: "failed to get product for ID"
  reason: "false-positive ERROR: stripe.GetPlacementPriceModel default fallback at service.go:689 logs ERROR but code continues with Product N name; bill generates correctly"
- pattern: "Failed to get bucket notification config, failing safe"
  reason: "intentional fallback: cache+DB both failed, code returns TransmitEvent=true downstream; benign per source comment"
- pattern: "Error enqueueing message"
  reason: "Segment.io analytics queue failure; telemetry-only, user actions unaffected"
-->

