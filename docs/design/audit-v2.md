# Auditing V2: Random Node Selection

## Abstract

This design doc outlines how we will implement a Version 2 of the auditing service.
With our current auditing service, we are currently auditing per segment of data.
We need to keep this existing method of auditing, but also add an additional audit selection process that selects random nodes to audit.

## Background

As our network grows, it will take longer for nodes to get vetted.
This is because every time an upload happens, we send 5% of the uploaded data to unvetted nodes and 95% to vetted nodes.
When auditing occurs, we currently select a random stripe within a random segment.
If we're selecting audits at random per byte, every segment has some percentage that went to vetted nodes.
As more nodes join the network, it will become exponentially less likely that an unvetted node will be audited since most data will be stored on vetted nodes.
With a satellite with one petabyte of data currently, new nodes will take one month to get vetted, which is way too long.

We want a way to select segments to audit based on statistically randomly selected storage nodes.
However, the way we select nodes should also be biased toward unvetted nodes.
Currently, there's not a way to select a stripe based on a storage node.

## Design

We will create an audit observer that uses the metainfo loop, and this observer will use both methods of selection. It will create reservoir samples for segments following the current selection method (randomly across bytes), and also create a small reservoir for each unvetted node.

After selection, the rest of the auditing process will occur the same way (picking a segment, picking a stripe, downloading all erasure shares associated with that stripe).
The chances of selecting the same stripe are rare, but it's normal/expected behavior if occasionally a stripe is audited more than once.

With both loops, we should have auditing that occurs statistically uniform across both nodes and bytes.

### **Selection via Reservoir Sampling**

Reservoir sampling: a family of algorithms for randomly choosing a sample of items with uniform probability from a stream of data (of unknown size).

As the audit observer uses the metainfo loop to iterate through the pointerdb, we're going through all segments and creating reservoirs per unvetted node, and filling the reservoirs with segments to audit.
The reason for not having reservoirs for all nodes it to minimize memory consumption. We're adding this new method of audit selection is to increase audit frequency for unvetted nodes.
Once vetted, they should be selected for auditing based on the per stripe (existing selection method), but there is no need to increase the frequency of audits for already-vetted nodes.

We could generate 5 random segments per node, then randomly select some segments per node.
By choosing nodes out of the reservoirs, we would receive a random sample for auditing.

• We have a reservoir of k items and a stream of n items, where n is an unknown number.
• Fill the reservoir from [0...k-1] with the first k items in the stream
• For every item in the stream from index i=k..n-1, pick a random number j=rand(0..i), and if j<k, replace reservoir[j] with stream[i]

## Rationale

An audit observer on the metainfo loop, like every observer on the loop, will always hit every segment on the satellite exactly once, allowing us to get the most accurate possible sample for each node.

While we initially considered integrating the audit system's random node selection process with the existing garbage collection observer, we decided not to do this because the difference in required interval for each observer would mean either too many bloom filters being created unnecessarily or audits occurring too slowly.

An initial idea for implementation was to sort the nodes table for nodes with least amount of audits, then select one node randomly within that low amount of audits.
However, we decided it may not be necessary to keep track of how many audits per storage node if we're able to randomly select across nodes.

Another approach that we decided not to pursue was the a reverse method of looking up segments or pieces by node ID e.g. a table where each row is a node ID and an encrypted metainfo path.
Every time a segment is committed or deleted, that table (and every node) gets updated.
This could simplify the garbage collection process, but complexify upload and download.
We decided that this would increase database size too significantly to be viable.

## Implementation

1. Add reservoir sampling struct for node auditing.
2. Create an audit observer that connects to metainfo loop but otherwise uses existing audit code for selection.
3. Have the audit observer update the reservoir sampling structs.
3. Make selection biased in favor of unvetted nodes, and select a random segment to audit based on the populated reservoir sampling struct for that node.
4. Audit the segment selected from part 3, as we are doing already.

## Open issues

1. Location of reservoir sampling. (resolved)
    - Where do we have this happen? In a garbage collection observer or a new observer?

From Moby: "The main issue with integrating it into the gc observer is it means we will always be forcing the gc interval and the node audit reservoir sampling interval to be exactly the same. I don't think the performance gain of combining the two is necessarily worth the limitations created. Plus, this is the entire reason we created the metainfo loop/observer architecture."

    - We want uploads to be performant with minimal db transactions, but we know that audits need to happen very frequently.
    - We'll create a new audit observer.

2. Should we run both audit selection processes within the same loop or in separate loops? (resolved)
- We'll use one audit observer that joins with the metainfo loop. This observer will be in charge of both methods of selection. It will create reservoir samples for segments following the current selection method (randomly across bytes), and also create a small reservoir for each unvetted node.