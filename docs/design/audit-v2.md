# Auditing V2: Random Node Selection

## Abstract

This design doc outlines how we will implement a Version 2 of the auditing service.
With our current auditing service, we are currently auditing per segment of data.
We need to keep this existing method of auditing, but also add an additional audit selection process that selects random nodes to audit.

## Background

As our network grows, it will take longer for nodes to get vetted.
This is because every time an upload happens, we send 5% of the uploaded data to unvetted nodes and 95% to vetted nodes.
When auditing occurs, we currently select a random stripe within a segment.
If we're selecting audits at random per byte, every segment has some percentage that went to vetted nodes.
As more nodes join the network, it will become exponentially less likely that an unvetted node will be audited since most data will be stored on vetted nodes.
With a satellite with one petabyte of data currently, new nodes will take one month to get vetted, which is way too long.

We want a way to select segments to audit based on statistically randomly selected storage nodes.
However, the way we select nodes should also be biased toward unvetted nodes.
Currently, there's not a way to select a stripe based on a storage node.

## Design

Two different loops will select audits:
- One loop will select based on stripes.
- The second loop will select based on nodes.

After selection, the rest of the auditing process will occur the same way (picking a segment, picking a stripe, downloading all erasure shares associated with that stripe).
The chances of selecting the same stripe are rare, but should be accounted for.

With both loops, we should have auditing that occurs statistically uniform acorss both nodes and bytes.

### **Selection via Reservoir Sampling**

Reservoir sampling: a family of algorithms for randomly choosing a sample of items with uniform probability from a stream of data (of unknown size).

During uploads or during garbage collection bloom filter generation, we're going through all segments and creating filters per node.
This is a good opportunity to insert reservoir sampling for auditing purposes.

We will have little reservoirs of segments for all of the nodes.
We could generate 5 random segments per node, then randomly select some segments per node.
By choosing nodes out of the reservoirs, we would receive a random sample.
Unvetted nodes would get the same number of audits as vetted nodes.

However, one problem with piggybacking off of existing garbage collection bloom filter generation is that bloom filter generation occurs every seven days.

The gc observer subscribes to the metainfo loop every week, but we could make an observer that subscribes more often and gets a stream as often as we want, so it doesn't have to be part of gc.
However, this could potentially create a lot more db transactions, which we want to minimize.
[See Open Issue 1.]

## Rationale

An initial idea for implementation was to sort the nodes table for nodes with least amount of audits, then select one node randomly within that low amount of audits.
However, we decided it may not be necessary to keep track of how many audits per storage node if we're able to randomly select across nodes.

We were also considering implementing a reverse way of looking up segments or pieces by node IDs.
e.g. a table where each row is a node ID and an encrypted metainfo path.
Every time a segment is committed or deleted, that table (and every node) gets updated.
The advantage is that it can super simplify the garbage collection process, but complexify upload and download.

## Implementation

1. Nail down open issue 1 and identify where reservoir sampling should occur.
2. Create new part of audit service that loops over nodes.
3. Add new methods to this loop that allow for selection from reservoir.
4. Coordinate both selection loops to use same "audit path" for verification.

## Open issues

1. Location of reservoir sampling.
    - Where do we have this happen?
    - We want uploads to be performant with minimal db transactions, but we know that audits need to happen very frequently.

2. How to coordinate both loops running simultaneously/using same "audit path" once data is selected for audit?