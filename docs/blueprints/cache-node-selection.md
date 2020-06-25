# Nodes Selection Cache

## Abstract

When an uplink uploads a file, the satellite selects which storage nodes to store that data on. This involves querying the nodes table in the satellitedb and it is a time intensive query resulting in slow upload times. This design doc explores caching node table data to improve upload performance.

## Background

The size of nodes table for the largest satellite currently has about 10,830 rows and is approximately 10MB in size.

Many satellite services **write** to the nodes table. This includes:
- the contact service, every 1-2 hr every storage node checks-in and updates its row in the nodes table
- garbage collection, updates node piece count
- audit, batch updating node stats and updating node uptime info
- gracefulexit, disqualifies nodes and updates node exit status

Many satellite services **read** from the nodes table. Including the following:
- metainfo service, to select nodes for uploads and deletes
- satellite inspector, finding known offline nodes
- repair, finding missing pieces from nodes that are offline or unreliable and also finding reliable nodes (repair uses the ReliabilityCache of this data)
- garbage collection, reads node pieces counts
- gracefulexit, getting exited nodes

What data can be stale and for how long:
- adding a new node to the network: this can be stale
- changing a node from unvetted to vetted: this can be stale
- removing a node from being vetted: preferably not stale, but could handle a small amount of this

When selecting nodes to store files to, the following criteria must be met:
- the node is not disqualified
- the node is not suspended
- the node has not exited
- the node has sufficent free disk space
- the node has been contacted recently
- the node has participated in a sufficient number of audit
- the nodes has sufficient uptime counts
- it is a storage node type

## Design

We want to create a read-only in-memory (see "rationale" section on why not other options) cache that contains data from the satellitedb nodes table that will be used to select storage nodes to upload files to. The cache will be updated every few minutes but syncing to the database and querying for all valid nodesto store data on.

### In-memory

This selected-nodes-cache will be stored in-memory of the Satellite API process. This means that each replica of the API will contain a copy of the cache. Currently we run three API replicas. While an in-memory cache is the fastest option (vs using redis or postgres), it comes with some tradeoffs and added complexity. This includes:
- cache invalidation (see #cache-invalidation): what is the best way to update every replica of the cache when a change is made to only one of them
- as the API replicas are scaled up to handle more load, synchronizing every cache with the database could become a lot of load on the database
- as the number of storage nodes on the network grows and as the number of API replicas grow, this cache could end up accounting for a lot of memory usage

### Cache invalidation

Options to update every replica of the cache when a change is made to only one of them:
- if the cache can handle being stale to some degree, then each cache could simply contact the database and refresh all of its contents on some interval (this is what we chose for now, other options can be explored when needed)
- create a message queue that contains the updates to the cache that each cache reads from
- create RPC endpoint for caches to communicate that either something needs to be updated, or to re-sync with the db

### Data
The cache should contain the following data:
- already selected nodes and already selected **new** nodes (this may need to be 2 caches so we can select a smaller percentage of new nodes)
- The only data that is sent to the uplink about the storage nodes is its address, typically being the IP and port. So the data that the cache should store is the storage node id, ip, port.

### Update
The nodes table is the source of truth for the node data and should remain that way. The cache needs to be updated when it becomes stale. The cache can handle some amount of being stale, but ultimatly the cache should be updated when the following occur:

A node should be removed from the cache when the following happen:
- a node is disqualified, suspended, exited, out of space, hasn't been contacted recently

A node should be updated in the cache when the following happen:
- a node's address, port, or ip changes

A node should be added to the cache when:
- it is a new node on the network and is added to the nodes table for the first time (add to the unvetted selected nodes cache)
- a node now has more space

A node should move from the unvetted to vetted cache when:
- when it has completed enough audits and uptime checks to be vetted

## Rationale

Here are some other design that were considered:

1) cache the entire nodes table

This approach would allow more services to use this cache and benefit from the performance gains. However, it would also add a lot of complexity. Since the nodes table is the source, we need to update the cache anytime it becomes out of sync. Some queries might be able to be stale if it doesn't impact anything else, however it might get complex keeping track of what can and can't be stale. If the entire table is cached it will become outdated very often since so many services write to the table ( see background section above for a list). A fix to this could be to have the cache be the source of truth and frequently sync the cache to the nodes table. However this would be a big undertaking since it would require changing all the services that interact with the nodes table.

2) Using Redis instead of in memory cache

Using a remote cache like redis, might be preferred versus in memory since this cache will be used in each satellite api replica (which can currently scale up to 3-12).  The more api replicas we have, the more cache replicas stored in memory.

Concerns about using a remote cache are:
- the network latency retrieving data from the cache. Using the in-memory cache, selecting nodes should be very fast (estimating 0.1-0.5ms), so adding a network call out to redis will make this fast cache slower. However we could mitigate network latency by running redis closer to the satellite (i.e. in same kubernetes cluster or at least same physical location).

Concerns about **not** using a remote cache are:
- cache coherence
- that this will cause more load on the database to keep all replica caches in sync (bandwidth and load),
- every time we update the cache we may need to lock it and potentially block other work,
- every replica will use up this much more space in memory which will increase over time and add up with each new replica.

We need to know the size this cache will be since our current redis instance handles up to 1 GB so we might need to increase the size. Currently about 80% of the nodes table qualifies to be selected for uploads, so the size of the cache for the largest cache should be about 8MB if we store the entire row in the cache, but if we only store the address and storagenode id the cache should be less than a MB.

For now, lets try out using an in-memory cache for the performanace gains. If we encounter any of the concerns outlined above then we can change in the future.

3) Using a postgres materialized view for cached node data

Using a postgres materialized view to store all the vetted and unvetted nodes would allow us to implement this cache in the database layer instead of the application layer. This would require the application code to handle the refresh of the materialized view which could occur when one of the events from the #Update section happened. 

Pro:
- allows the db to handle the logic instead of adding a cache at the application layer
- can add an index to the expression to make even faster
- simplifies the select storage node query

Con:
- when refreshing the table, the entire query is executed again and the table is rebuilt. Depending on how often we rerfreshed and how large the materialized, this could be a lot of work for the db.
- we would still need to do a round trip to database to get the selected storage nodes
- the materialized view is represented as a table stored on disk which may be slower than a proper cache in memory

## Implementation

Implementation Options:
1. create in-memory cache as described in #Design
2. create a cache using redis (see #rationale 2.)
3. use a queue to store selections computed ahead of time (see prototype [PR](https://github.com/storj/storj/pull/3835)), this can be combined with 1 to reduce load on db as well

Detailed steps for implementation creating an in-memory cache for each Satellite API (1.):
- create a selected nodes cache in the satellite/overlay pkg
- populate the selected nodes cache with all valid nodes from the nodes table when it is initialized
- update the selected nodes cache when node table data changes (see list from design section above)
- add solution to cache invalidation
- have the upload operation use the selected nodes cache

## Open issues
