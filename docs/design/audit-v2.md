# Auditing V2: Random Node Selection

## Abstract

This design document describes auditing based on reservoir sampling segments per node.

## Background

As our network grows, it will take longer for nodes to get vetted.
This is because every time an upload happens, we send 5% of the uploaded data to unvetted nodes and 95% to vetted nodes.
Currently, we select a random stripe from a random segment for audits.
This correlates with auditing per byte. This means we are less likely to audit an unvetted node because only 5% gets uploaded to unvetted nodes.
It will become exponentially less likely that an unvetted node will be audited.

With a satellite with one petabyte of data, new nodes will take one month to get vetted.
However, with 12PB of data on the network, this vetting process time would take 12 months, which is much too long.
We need a scalable approach.

We want a way to select segments to audit based such that every node has an equal likelihood to be audited.

## Design

1. An audit observer iterates over the segments and uses reservoir sampling to pick paths for each node.
2. Once we have iterated over metainfo, we will put the segments from reservoirs in random order into audit queue.
3. Audit workers pick a segment from the queue.
4. Audit worker then picks a random stripe from the segment to audit.

Using reservoir sampling means we have an equal chance to pick a segment for every node.
Since every segment also audits 79 other nodes, we also audit other nodes.
The chance of a node appearing in a segment pointer is proportional to the amount of data that the node actually stores.
The more data that the node stores, the more chance it will be audited.

For unvetted and vetted nodes, we can have different reservoir sizes to ensure that unvetted nodes get audited faster.

By using a separate queue, we ensure that workers can run in separate processes and simplifies selection logic.
When we finish a new reservoir set, we override the previous queue, rather than adding to it.
Since the new data is more up to date and there's no downside in clearing the queue.
To have less predictability, we add the whole reservoir set in random node order, one segment at a time, to the queue.

Audit workers audit as previously:

1. Pick a segment from the queue.
2. Pick a random stripe.
3. Download all erasure shares.
4. Use Berlekamp-Welch algorithm to verify correctness.

This is a simplified version that doesn't describe [containment mode](audit-containment.md). Chances of selecting the same stripe are rare, but it wouldn't cause any significant harm.

To estimate appropriate settings for reservoir sampling, we need to run a simulation.

### Selection via Reservoir Sampling

Reservoir sampling: a family of algorithms for randomly choosing a sample of items with uniform probability from a stream of data (of unknown size).

Audit observer uses metainfo loop to iterate through the metainfo. It creates a reservoir per node. Reservoirs are filled with segments.

To increase audits for unvetted nodes, we can create a larger reservoir for unvetted nodes.
Two configurations for reservoir sampling: number of segments per unvetted nodes, and for vetted.

E.g. If nodes `n000`, `n002`, and `n003` are vetted, they will have fewer reservoir slots than unvetted nodes `n001` and `n004`.

```
n000 + + + +
n001 + + + + + + +
n002 + + + +
n003 + + + +
n004 + + + + + + +
```

Unvetted nodes should get 25,000 pieces per month. On a good day, there will be 1000 pieces added to an unvetted node, which should quickly fill the reservoir sample.

Algorithm:

+ We have a reservoir of `k` items and a `stream` of `n` items, where `n` is an unknown number.
+ Fill the reservoir from `[0...k-1]` with the first `k` items in the `stream`.
+ For every item in the `stream` from index `i=k..n-1`, pick a random number `j=rand(0..i)`, and if `j<k`, replace `reservoir[j]` with `stream[i]`.

## Rationale

An audit observer using metainfo loop will hit every segment exactly once, allowing us to get the most accurate possible sample for each node.

While we initially considered integrating the audit system's random node selection process with the existing garbage collection observer,
we decided not to do this because the difference in required interval for each observer would mean either too many bloom filters being created unnecessarily or audits occurring too slowly.
It also simplifies the implementation.

An initial idea for implementation was to sort the nodes table for nodes with least amount of audits, then select one node randomly within that low amount of audits.
However, we decided it may not be necessary to keep track of how many audits per storage node if we're able to randomly select across nodes.
Also, as new nodes join, this method won't adequately select older nodes or nodes with more audits.
This method would also require performance optimizations to account for reads from the DB, and updates when audits happen.
Random selection comes with an overall easier algorithm to implement with more statistical balance across the nodes.

Another approach that we decided not to pursue was the a reverse method of looking up segments or pieces by node ID e.g. a table where each row is a node ID and an encrypted metainfo path.
Every time a segment is committed or deleted, that table (and every node) gets updated.
This could simplify the garbage collection process, but complexify upload and download.
We decided that this would increase database size too significantly to be viable.

If we need fewer audits, then we could use power of two choices, in which we randomly select two nodes, then choose the one with fewer audits.
This would require tracking number of audits, but it would prevent having to sort and query all nodes by audit count, which could cause undesirable behavior.
For example, when new nodes join the network, the audit system could become stuck auditing only new nodes, and ignoring more established nodes.

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

    - We want uploads to be performant with minimal DB transactions, but we know that audits need to happen very frequently.
    - We'll create a new audit observer.

2. Should we run both audit selection processes within the same loop or in separate loops? (resolved)
- We'll only be using one audit selection process which will happen using the audit observer. It will create reservoir samples for all nodes.