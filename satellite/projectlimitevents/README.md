# projectlimitevents

This package queues and processes project limit threshold notification emails for storage and egress usage.

## Threshold detection

Storage and egress (bandwidth) thresholds are detected differently due to the nature of how each resource is accounted for.

### Storage

Storage threshold detection happens post-commit, after the object has been successfully written and its size is known. It is triggered in:

- `metainfo.CommitObject` — for regular multi-segment uploads
- `metainfo.CommitInlineObject` — for small inline uploads

At that point the committed size has already been added to the live accounting (Redis) cache by the preceding `CommitSegment` calls. Detection reads the current cache total as "after" and subtracts the committed size to derive "before", then checks whether any threshold (80%, 100%) was crossed in that range.

This approach was chosen because the object size is not known at `BeginObject` time — only the final `CommitObject` has the authoritative `TotalEncryptedSize`. Checking at `BeginObject` with a placeholder headroom of 1 byte would never detect a realistic threshold crossing.

### Egress (bandwidth)

Egress threshold detection happens at download request time, inside `checkDownloadLimits`, before the download is served.

Unlike storage, bandwidth is not committed in a single atomic step. Orders are created during a download and settled asynchronously in bulk by the orders service. There is no equivalent of `CommitObject` to hook into.

Detection therefore uses a point-in-time check against the accumulated monthly total already in the live accounting cache: if `current >= 80%` (or `100%`) of the limit and the corresponding flag is not yet set, an event is enqueued.

The practical consequence is that detection can lag by at most one download request: if a download pushes usage from 79% to 81%, the check at the start of that request sees 79% and does not fire. The event fires on the next download request when the settled total is visible in the cache. This is acceptable because bandwidth notifications are informational and bandwidth settlement latency is already on the order of minutes.
