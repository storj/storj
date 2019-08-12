# Auditing V2: Random Node Selection

## Abstract

This design doc outlines how we will implement a Version 2 of the auditing service.
With our current auditing service, we are auditing per segment of data.
We propose to replace this method of auditing with a new selection process that selects by random node instead.

## Background

As our network grows, it will take longer for nodes to get vetted.
This is because every time an upload happens, we send 5% of the uploaded data to unvetted nodes and 95% to vetted nodes.
When auditing occurs, we currently select a random stripe within a random segment.
If we're selecting audits at random per byte, every segment has some percentage that went to vetted nodes.
As more nodes join the network, it will become exponentially less likely that an unvetted node will be audited since most data will be stored on vetted nodes.
With a satellite with one petabyte of data, new nodes will take one month to get vetted. However, with 12PB of data on the network, this vetting process time would take 12 months, which is much too long. We need a scalable approach.

We want a way to select segments to audit based on statistically randomly selected storage nodes.
Currently, there's not a way to select a random storage node and audit on that basis.

## Design

We will create an audit observer that uses the metainfo loop, and this observer will create a reservoir sample of segments for every node. This audit observer will replace the existing method of auditing per byte. The observer will loop through all of metainfo and build a reservoir cache for each node. The audit will then pick a configurable number of random nodes, then a random segment to audit from each of the nodes' reservoirs.

If each segment generates 80 pieces on average, every time we pick a segment, we're not only auditing one specific node, we're also auditing 79 other nodes. The chance of a node appearing in a segment's pointer is proportional to the amount of data that the node actually stores. The more data that the node stores, the more chance it will be audited. We will set a minimum number of audits for unvetted nodes, and expect more audits for nodes that store more data.

After selection, the rest of the auditing process will occur the same way as it does currently: picking a random segment, picking a random stripe, downloading all erasure shares associated with that stripe and using Berlekamp-Welch algorithm (via the Infectious library) to verify that they haven't been altered.
The chances of selecting the same stripe are rare, but it's normal/expected behavior if occasionally a stripe is audited more than once.

We plan to run a simulation for this new method of auditing so we can estimate appropriate settings around reservoir sampling. If we decide that we want to prefer nodes with less audits, then we will implement the power of two choices, in which we randomly select two nodes, then choose the one with less audits.
This would still require tracking number of audits, but it would prevent having to sort and query all nodes by audit count, which could cause undesirable behavior. For example, when new nodes join the network, the audit system could become stuck auditing only new nodes, and ignoring more established nodes.

We are expecting close to 3 audits per day for unvetted nodes. The satellite currently issues one audit every 30 seconds, the current interval, which gives us close to 3,000 audits per day. There are about 1,000 nodes on the network currently. This could mean that the default size of a reservoir should be three, if we assume a full iteration of reservoirs in one day.

### Selection via Reservoir Sampling

Reservoir sampling: a family of algorithms for randomly choosing a sample of items with uniform probability from a stream of data (of unknown size).

As the audit observer uses the metainfo loop to iterate through the pointerdb, we're going through all segments and creating reservoirs per node, and filling the reservoirs with segments to audit.

In order to increase the amount of audits for unvetted nodes, we can build larger reservoir samples for unvetted nodes.
Two configurations for reservoir sampling: number of segments per unvetted nodes, and for vetted.

E.g. If nodes n000, n002, and n003 are vetted, they will have less reservoir slots than unvetted nodes n001 and n004.

n000 + + + +
n001 + + + + + + +
n002 + + + +
n003 + + + +
n004 + + + + + + +

Unvetted nodes should get 25,000 pieces per month. On a good day, there will be 1000 pieces added to an unvetted node, which should quickly fill the reservoir sample.

• We have a reservoir of k items and a stream of n items, where n is an unknown number.
• Fill the reservoir from [0...k-1] with the first k items in the stream
• For every item in the stream from index i=k..n-1, pick a random number j=rand(0..i), and if j<k, replace reservoir[j] with stream[i]

## Rationale: Discussion of Alternate Approaches, Advantages, Disadvantages

An audit observer on the metainfo loop, like every observer on the loop, will always hit every segment on the satellite exactly once, allowing us to get the most accurate possible sample for each node.

While we initially considered integrating the audit system's random node selection process with the existing garbage collection observer, we decided not to do this because the difference in required interval for each observer would mean either too many gc bloom filters being created unnecessarily or audits occurring too slowly.

An initial idea for implementation was to sort the nodes table for nodes with least amount of audits, then select one node randomly within that low amount of audits.
However, we decided it may not be necessary to keep track of how many audits per storage node if we're able to randomly select across nodes. Also, as new nodes join, this method won't adequately select older nodes or nodes with more audits. This method would also require performance optimizations to account for reads from the db, and updates when audits happen.
Random selection comes with an overall easier algorithm to implement with more statistical balance across the nodes.

Another approach that we decided not to pursue was the a reverse method of looking up segments or pieces by node ID e.g. a table where each row is a node ID and an encrypted metainfo path.
Every time a segment is committed or deleted, that table (and every node) gets updated.
This could simplify the garbage collection process, but complexify upload and download.
We decided that this would increase database size too significantly to be viable.

## Implementation

1. [Create a simulation](https://storjlabs.atlassian.net/browse/V3-2359) for random audit selection with reservoirs to figure out how many audits for vetted and unvetted nodes would be issued per day, configurations for reservoir sizes for vetted and unvetted nodes, and other insights (probably work with Jens and/or DS team for this).
2. [Add reservoir sampling struct for node auditing.](https://storjlabs.atlassian.net/browse/V3-2360)
3. [Create an audit observer that connects to metainfo loop.](https://storjlabs.atlassian.net/browse/V3-2361)
4. [Have the audit observer update the reservoir sampling structs.](https://storjlabs.atlassian.net/browse/V3-2362)
5. [Implement random selection of a node's reservoir, then of a random segment to audit.](https://storjlabs.atlassian.net/browse/V3-2363)
6. [Audit the segment selected from part 5 in the same way as verification happens in the existing system.](https://storjlabs.atlassian.net/browse/V3-2364)

## Open issues

1. Location of reservoir sampling. (resolved)
    - Where do we have this happen? In a garbage collection observer or a new observer?

From Moby: "The main issue with integrating it into the gc observer is it means we will always be forcing the gc interval and the node audit reservoir sampling interval to be exactly the same. I don't think the performance gain of combining the two is necessarily worth the limitations created. Plus, this is the entire reason we created the metainfo loop/observer architecture."

    - We want uploads to be performant with minimal db transactions, but we know that audits need to happen very frequently.
    - We'll create a new audit observer.

2. Should we run both audit selection processes within the same loop or in separate loops? (resolved)
- We'll only be using one audit selection process which will happen using the audit observer. It will create reservoir samples for all nodes.