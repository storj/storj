# Planned Downtime 

## Abstract

This document describes a design that allows storage node operators to take their nodes offline at planned times without penalty. The satellite uses knowledge of planned downtime to move pieces between nodes to manage durability.

## Background

With the completion of [storage node downtime tracking with audits](./storage-node-downtime-tracking-with-audits.md), we will soon be penalizing nodes for downtime through suspension and disqualification. This is good news for durability because up until this point, the only incentive for an SNO to keep their node from going offline is a lower payout - there were no long-term consequences until the new downtime tracking system.

However, there are a variety of situations where an SNO may know in advance that they want to take their node(s) offline for a period of time. For example, if a node operator is moving and runs their node out of their home, they will need to take their node offline in order to move it. If an operator is running nodes out of a datacenter, that datacenter may have planned maintenance scheduled.

"Unplanned downtime" describes any storage node downtime that does not fall under "planned downtime" as described in this document. Unplanned downtime might be caused by a variety of events including, but not limited to, power outages, deliberate or accidental shutoff of a node, or fatal storagenode configuration problems.

At any time, a node might be storing pieces for segments that are very close to the repair threshold (or even below the repair threshold). The node going offline would damage the health of these segments even more, so if we know in advance that a node will be going offline, we can preemptively transfer pieces from particularly damaged segments off of it. 

## Design

### Tracking Planned Downtime

Basically the goal from a technical perspective is to just not create audit windows for hours where a node has planned downtime. Preferably in a way where we can tell if a node is currently in planned downtime from the `nodes` table with no other tables. If we do not create audit windows for planned downtime hours, node online scores will not be impacted by any downtime during that time.

We will allow each node to schedule a single upcoming planned downtime period. All the relevant information for this period will be added to the `nodes` table.

We also want to track the total amount of planned downtime for each node. So at least 4 fields need to be added to the `nodes` table: `next_planned_downtime_start` timestamp, `next_planned_downtime_duration` int (hours), `total_planned_downtime_start` timestamp, and `total_planned_downtime_period` int (hours). `total_planned_downtime_start` and `total_planned_downtime_period` are used to keep track of how much planned downtime the node has taken in a globablly configured period of time (e.g. a month or year). They will both be reset after the period passes.

When we audit a node, if that node is offline _and_ is in a period of planned downtime, we want to cut short any reputation calculations so that we do not modify their audit score when they are in planned downtime.

### Transferring Pieces

Conveniently, we already have protobufs and example code for transferring pieces from one node to another as a result of [Graceful Exit](./storagenode-graceful-exit/), so implementing this type of transfer should be fairly straightforward.

The main thing that needs to be added on the storagenode side is an endpoint exclusively for satellite use that allows the satellite to initiate a piece transfer request unrelated to graceful exit. It is possible that we could use the same code path that graceful exit will use (we are currently refactoring graceful exit), but this might not be the best idea. In any case, it is worth considering, especially if we want to stream many piece transfer requests back-to-back.

On the satellite side, we will need to add a chore that does the following:
1. Get a list of nodes with upcoming planned downtime within a configurable interval.
2. Create a metainfo loop observer which generates a list of unhealthy and/or close-to-unhealthy segments which these planned downtime nodes are also a part of (can this somehow be added to the checker?). The output of the observer should be a map of node IDs to piece IDs which need to be transferred (as well as any other important info in metainfo needed for a piece transfer).
3. Create a limiter for opening concurrent connections to nodes, and using this limiter, for each node, attempt to transfer all of the relevant pieces.

An attempt to transfer a stream of pieces for a given node will look like:
1. Open connection to stream piece transfer requests to nodes (like graceful exit, but initiated by satellite)
2. Select target node from overlay cache. Exclude nodes with upcoming planned downtime (TODO come up with easy to say name that describes "nodes with upcoming planned downtime")
3. Construct and sign piece transfer request with origin node, target node, piece ID, any other relevant info
4. Send piece transfer request to node and wait for response
5. On failure, retry
6. On success, go to next piece and repeat from step 2
7. On list completion, close connection

The chore will run on a configured interval - if there are nodes with upcoming planned downtime, it will go ahead and try to transfer pieces off of them. Otherwise, it will wait before checking again.

### Restrictions for Entering Planned Downtime
The following is a list of cases where we would not want to allow a node to be accepted for planned downtime:
* The node has already taken too much planned downtime in the past year (or whatever interval we decide)
* The node wants to take too much downtime at one time
* The node wants to take planned downtime too soon (i.e. we do not have enough time to transfer pieces from it)
* Too many nodes are taking planned downtime in the same period (unlikely scenario, but we should account for it just in case everyone wants to take planned downtime on a holiday or something)

### Miscellaneous Details
The satellite chore that transfers pieces off nodes going into planned downtime should run separately from the satellite core. It is not essential for the basic operation of the network, and running it as part of the core would negatively impact performance of essential satellite operations from a customer perspective.

## Implementation

### Config Values
The following are configuration values that will be needed for the planned downtime service (values are WIP and need to be discussed):
* Maximium duration of single planned downtime session for a node: 24 hours
* Minimum notice required for a single planned downtime session for a node: 1 week
* Planned downtime total period - the period for which a node's total planned downtime is tracked and limited: 1 year 
* Planned downtime total limit - the maximum downtime that can be taken inside a planned downtime total period: 1 week
* Interval for selecting "soon-downtime" nodes to transfer pieces from: 1 week 
* Planned downtime segment health threshold - how unhealthy a segment needs to be to justify transferring a piece off of a "soon-downtime" node: 54
* Retry count for single planned downtime piece transfer: 3
* Memory limit for piece transfer list in planned downtime chore: 500MB

## Rationale

The obvious alternative to implementing a planned downtime feature is to not implement one at all. This is obviously easier from a technical perspective, but it comes with downsides: There will always be situations where node operators will want/need to take down their nodes temporarily, and know this in advance. We should expect SNOs to take their nodes offline in these situations regardless of whether we implement a planned downtime feature. If the feature is not implemented, a storage node in this situation would be penalized for going down. Additionally, the durability of any segments the storage node holds pieces for would degrade, potentially bringing many segments into the repair queue. If the satellite knows about the downtime in advance, we can prepare by transferring pieces of weak segments off the storage node and preserve that storage node's reputation during the downtime, in a win-win scenario for the storage node and satellite.

There are some technical details of planned downtime implementation that could be designed differently, but we decided against them as their benefits did not outweight their downsides:
* Instead of storing planned downtime information directly in the `nodes` table, have a separate `planned_downtime` table which has rows for the start time and duration of each node's planned downtime. So that the table does not have to be queried each time a node is audited, have synchronized chores that run at specific times each hour to check the table for upcoming and recently ended planned downtime periods, and update an `in_planned_downtime` flag in the `nodes` table.
    * pros: storage nodes can have multiple planned downtime periods at a time
    * cons: synchronized chores are not something that we have an easy way of doing in a non-hacky way right now; this solution is too complex compared to what is necessary for the minimally satisfactory solution
* Instead of transferring pieces directly from one storage node to another, the satellite place an intermediate role, first downloading the piece from the planned downtime node, then uploading it to the target node 
    * pros: It _might_ be easier to write the code for this (it also might not be)
    * cons: We already have code for successful storagenode-storagenode piece transfers used for graceful exit, and relying on the satellite for the bandwidth of piece transfers would reduce performance

## Wrapup

## Open Issues

* How do we limit planned downtime? I see two main options (but there are probably more):
    1. Have a specific number of hours per year that a node can use for planned downtime. Once they go over, they simply stop being able to schedule planned downtime.
    2. No strict limit on planned downtime. Rather, financially disincentivize nodes from taking planned downtime. No disincentive for first x hours, but after that, planned downtime can reduce payout
* What should the interface for planned downtime look like?
* How far in advance should planned downtime be scheduled? >24 hours? >1 month? No limitation?
* What should the consequences be if a storage node repeatedly fails to transfer pieces before planned downtime?
