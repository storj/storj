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
- repair, finding missing pieces from nodes that are offline or unreliable and also finding reliable nodes
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

We want to create a read-only cache that contains data from the nodes table that will be used to select storage nodes to upload files to.

The cache should contain the following data:
- already selected nodes and already selected **new** nodes (this may need to be 2 caches so we can select a smaller percentage of new nodes)

The nodes table is the source of truth for the node data and should remain that way. The cache needs to be updated when it becomes stale. The cache should be updated when the following occur:
- a node in the cache is no longer in good condition to be storing data, this node needs to be removed from the cache
- a node that was previously able to store data changes and doesn't want to get data anymore (i.e. out of space or graceful exit), this node needs to be removed from the cache
- a new node becomes vetted and should be in the cache that contains the vetted nodes, this node needs to be moved from the unvetted group to the vetted group
- a node that was not previously able to store data changes and becomes able to, this node needs to be added to the cache
- a node changes its external address or port, this node needs to be updated in the cache

Using Redis:
Using a remote cache like redis, would be preferred versus in memory since this cache will be used in each satellite api replica (which can currently scale up to 12).  The more api replicas we have, the more we caches will need to be updated. I’m worried of 2 things, that this will cause more load on the database and also that every time we update the cache we need to lock it and i want to reduce that. We need to know the size this cache will be since our current redis instance handles up to 1 Gb so we might need to increase the size. Currently about 80% of the nodes table qualifies to be selected, so the size of the cache for the largest cache should be about 8MB right now.

## Rationale

Here are some other design that were considered:

1) cache the entire nodes table

This approach would allow more services to use this cache and benefit from the performance gains. However, it would also add a lot of complexity. Since the nodes table is the source, we need to update the cache anytime it becomes out of sync. Some queries might be able to be stale if it doesn't impact anything else, however it might get complex keeping track of what can and can't be stale. If the entire table is cached it will become outdated very often since so many services write to the table ( see background section above for a list). A fix to this could be to have the cache be the source of truth and frequently sync the cache to the nodes table. However this would be a big undertaking since it would require changing all the services that interact with the nodes table.

## Implementation

Steps for implementation:
- create a selected nodes cache in the satellite/overlay pkg
- add redis support for this selected nodes cache
- populate the selected nodes cache with data from the nodes table
- update the selected nodes cache when node table data changes
- have the upload operation to use the selected nodes cache

## Open issues

- Should this cache only be used for selecting groups of storage nodes to upload files to? Or do we want to use it for other use cases as well right now.
