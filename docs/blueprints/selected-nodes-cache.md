# Selected Nodes Cache

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

We want to create a read-only in-memory (see "rationale" section on why not use redis) cache that contains data from the nodes table that will be used to select storage nodes to upload files to.

The cache should contain the following data:
- already selected nodes and already selected **new** nodes (this may need to be 2 caches so we can select a smaller percentage of new nodes)
- The only data that is sent to the uplink about the storage nodes is its address, typically being the IP and port. So the data that the cache should store is the storage node id, ip, port.

The nodes table is the source of truth for the node data and should remain that way. The cache needs to be updated when it becomes stale. The cache should be updated when the following occur:

A node should be removed from the cache when the following happen:
- a node is disqualified, suspended, exited, out of space, hasn't been contacted recently

A node should be updated in the cache when the following happen:
- a node's address, port, or ip changes

A node should be added to the cache when:
- it is a new node on the network and is added to the nodes table for the first time (add to the unvetted selected nodes cache)

A node should move from the unvetted to vetted cache when:
- when it has completed enough audits and uptime checks to be vetted

## Rationale

Here are some other design that were considered:

1) cache the entire nodes table

This approach would allow more services to use this cache and benefit from the performance gains. However, it would also add a lot of complexity. Since the nodes table is the source, we need to update the cache anytime it becomes out of sync. Some queries might be able to be stale if it doesn't impact anything else, however it might get complex keeping track of what can and can't be stale. If the entire table is cached it will become outdated very often since so many services write to the table ( see background section above for a list). A fix to this could be to have the cache be the source of truth and frequently sync the cache to the nodes table. However this would be a big undertaking since it would require changing all the services that interact with the nodes table.

2) Using Redis instead of in memory cache

Using a remote cache like redis, might be preferred versus in memory since this cache will be used in each satellite api replica (which can currently scale up to 12).  The more api replicas we have, the more caches if we store in memory.

Concerns about using a remote cache are:
- the network latency retrieving data from the cache. Using the cache, selecting nodes should be very fast (estimating 0.1-0.5ms), so adding a network call out to redis will make this fast cache dramtically slower.

Concerns about **not** using a remote cache are:
- that this will cause more load on the database to keep all replica caches in sync (bandwidth and load),
- every time we update the cache we may need to lock it and potentially block other work,
- every replica will use up this much more space in memory which will increase over time and add up with each new replica.

We need to know the size this cache will be since our current redis instance handles up to 1 GB so we might need to increase the size. Currently about 80% of the nodes table qualifies to be selected for uploads, so the size of the cache for the largest cache should be about 8MB if we store the entire row in the cache, but if we only store the address and storagenode id the cache should be less than a MB.

For now, lets try out using an in-memory cache for the performanace gains. If we encounter any of the concerns outlined above then we can change in the future.

## Implementation

Steps for implementation:
- create a selected nodes cache in the satellite/overlay pkg
- add redis support for this selected nodes cache
- populate the selected nodes cache with all valida nodes from the nodes table when it is initialized
- update the selected nodes cache when node table data changes (see list from design section above)
- have the upload operation use the selected nodes cache

## Open issues

- Should this cache only be used for selecting groups of storage nodes to upload files to? Or do we want to use it for other use cases as well right now.

- Is it sufficient to only store the minimum data needed to send back to the uplink? Versus all the data needing to confirm the node is valid? Meaning should we only store the address and storagenode id *or* store also store all the fields needed to determine if the node is valid to upload data to?

- Should we intermittenly re-initialize the cache since it might get out of sync over time?
