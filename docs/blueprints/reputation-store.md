# Reputation Store

## Abstract

Right now, we update the database for every single audit. Whenever we audit a stripe, we get a collection of audit outcomes for nodes holding that stripe (e.g. successes, failures, contained, suspended, etc...). For each outcome, we make a database transaction and update all the nodes for that audit outcome at once. 

This is not very performant. The `nodes` table is also heavily contested, and is one of the most crucial pieces for performant network operations - it is fundamental for uploads and downloads, and any delay will be directly seen by customers.

This document describes a solution for an interface to a central reputation store, which occasionally syncs with the `nodes` table. This allows us to transfer the workload of audits off of the `nodes` table to another store. 

## Background

The reason the `nodes` table has such poor performance is because we are doing a lot of row updates (e.g. audits, checkins) at the same time as we are trying to scan over the entire table (e.g. node selection, redash). Independently, row updates or scans work very well on Cockroach, but when combined, they can result in very undesirable performance.

Having a separate store for reputation allows the audit service to make lots of updates for individual nodes, without needing to worry about bottlenecks caused by queries which need to do scans of the `nodes` table.

Some fields are necessary in both the `nodes` table and the new reputation store. We will call these "contested" columns. Some examples are `disqualified` and `suspended`. Node selection needs to read them frequently, and audit needs to update them depending on audit outcomes.

Through [denormalization](https://en.wikipedia.org/wiki/Denormalization), we can duplicate these fields in two separate stores so that the audit service can update the `nodes` table only when needed. The uncontested columns can be frequently updated by the audit service via the reputation store and the contested columns can be frequently read by node selection via the nodes table. The contested columns in the nodes table will only need to be updated by the reputation service if one of them changes.

The reputation store will be considered the source of truth for the contested values, as it will contain the most recently-updated values for reputation.

Note that we may replace the contested values in the overlay with a single `healthy` column (see "Other Details" below).

### Reputation Package

The reputation package will be responsible for maintaining the reputation store and updating the `nodes` table when necessary.

It will contain a service with the following functionality. We can change this in the future if needed - this is just a starting point.

```
type DB interface {
    Update(nodeID storj.NodeID, overlay.AuditType) (_, *StatusChange, changed bool, err error)
    Get(nodeID storj.NodeID) (*Info, error)
}

type Service struct {
    overlay *overlay.Service
    db      DB
    config  Config
}

func (service *Service) ApplyAudit(nodeID storj.NodeID, overlay.AuditType) error
func (service *Service) Get(nodeID storj.NodeID) (*Info, error)
func (service *Service) TestingDisqualify(nodeID storj.NodeID) error
func (service *Service) TestingSetState(state Info) error
```

`DB` contains the low-level interface for the backing store. It is designed in a way so that we can implement a solution using any key-value store. For our initial solution, we plan to implement it using Cockroach. However, in the future we should be able to easily switch it out thanks to the simple interface.

Here are some type definitions for the above. We should also move types such as `AuditHistory` and `AuditType` out of the `overlay` package and into the `reputation` package. 

```
// these are the values that need to be updated in the nodes table if they change
type statusChange struct {
    Contained bool
    Disqualified *time.Time
    Suspended *time.Time
    UnknownAuditSuspended *time.Time
    OfflineSuspended *time.Time
    VettedAt *time.Time
}

// these are all reputation values, including those in the statusChange struct
type Info struct {
    AuditSuccessCount int64
    TotalAuditCount   int64
    VettedAt          *time.Time

    Contained             bool
    Disqualified          *time.Time
    Suspended             *time.Time
    UnknownAuditSuspended *time.Time
    OfflineSuspended      *time.Time
    UnderReview           *time.Time
    OnlineScore           float64
    AuditHistory          AuditHistory

    AuditReputationAlpha        float64
    AuditReputationBeta         float64
    UnknownAuditReputationAlpha float64
    UnknownAuditReputationBeta  float64
}
```

The reputation service has a function called `ApplyAudit` to update a single node with a single audit outcome. The implementation that backs the service can handle this however we decide (e.g. db, in-memory cache).

`ApplyAudit` will first update the reputation store backing the service. If any of the fields in `statusChanged` have been modified (e.g. `disqualified`, `suspended`), `ApplyAudit` will also update the overlay cache with these new values. However, because events like nodes becoming disqualified are very infrequent, the overall load on that table from audits should be significantly reduced.

### Store

The store that backs the reputation service needs to have all the same fields as the `Info` type defined above, plus a primary key,  `id`, which should be the node ID. 

### Other Details

If we decide to use Cockroach as the implementation for the store, we can repurpose the `audit_histories` table, as long as we make sure to remove references to it from queries in the overlay.

In a later stage of this implementation, we should replace the "contested" fields in the `nodes` table with a single `healthy` boolean, which indicates whether a node should be included in node selection. This would help with `nodes` table performance since from a node selection standpoint, there is no need to treat fields like `disqualified` and `suspended` differently. We would only need to query one field to determine health for node selection, rather than ~5. If we replace the contested fields with `healthy`, we will need to update services like payments, which depend on knowing if a node is `disqualified`. These services will need to query the reputation store, rather than the `nodes` table.

## Rationale

One solution to this problem that doesn't involve a DB would be to back the reputation service with an in-memory cache. The cache would need to pull reputation information from the `nodes` table on startup, keep it up to date as audits occur, and flush on a time interval or whenever a contested field for a node changes due to audits (e.g. disqualified).

We could still implement an in-memory cache with the design outlined above with a few key changes:
* Add a `Initialize(nodeID storj.NodeID, reputation Info)` function to the service, allowing us to set the reputation for a node in the cache based on the `nodes` table on satellite startup.
* Update the design to copy _all_ reputation values, not only the contested values, to the `nodes` table when a contested field changes or on shutdown. While a reputation store can keep track of the frequently-updated reputation values between restarts, a cache will need to update these values in the `nodes` table to preserve them.
* Include a `last_flushed` field in the cache for _each_ node. Whenever we update a contested reputation value in the cache, the service transfers all reputation values for that node to the `nodes` table. Otherwise, when enough time (e.g. 1 hour) has passed since `last_flushed` for this particular node, the cache will transfer the values.

If we want to take the approach of having an in-memory cache, we should make sure that all operations it does on the `nodes` table are commutative. That way multiple caches could update the `nodes` table without needing to worry about overwriting any information. The `audit` and `repair` packages both update reputation, and they run on different machines in production. So it is important to make sure that they can work together cohesively.

## Wrapup

Moby and Cam are responsible for archiving this document once the implementation is completed.

## Open issues

