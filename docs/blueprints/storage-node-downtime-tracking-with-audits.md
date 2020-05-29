# Storage Node Downtime Tracking With Audits

## Abstract

This document describes a means of tracking storage node downtime with audits and using this information to suspend and disqualify.

## Background

The previous implementation of uptime reputation consisted of a ratio of online audits to offline audits. We encountered a problem where some nodes' reputations would quickly become destroyed over a relatively short period of downtime due to the frequency of auditing any particular node being directly correlated with the number of pieces it holds. To solve this problem we need a system that takes into account not only how many offline audits occur, but _when_ they occur as well.

## Design

The solution proposed here is to use a series of sliding windows to indicate a general timeframe in which offline audits occur. Each window keeps two separate tallies indicating how many offline audits and total audits a particular node received within its timeframe. Once a window is complete, it is scored by calculating the percentage of total audits for which it was offline. We can average these scores over a trailing period of time, called the _tracking period_, to determine an overall "offline score" to be used for suspension and disqualification. By granting each individual window the same weight in the calculation of the overall average, the effect of any particularly unlucky period can be minimized while still allowing us to take the failures into account over a longer period.

Storage node downtime can have a range of causes. For those storage node operators who may have fallen victim to a temporary issue, we want to give them a chance to diagnose and fix it before disqualifying them for good. For this reason, we are introducing suspension as a component of disqualification.

Once a node's offline score has risen above an _offline threshold_, it is _suspended_ and enters a period of review. A suspended node will not receive any new pieces, but can continue to receive download and audit requests for the pieces it currently holds. However, its pieces are considered to be unhealthy. We repair a segment if it contains too many unhealthy pieces, at which point we may transfer the repaired pieces from a suspended node to a more reliable node. If at any point during the review period we find that a node's score has fallen below the offline threshold, it is unsuspended, or _reinstated_, but it remains _under review_. This prevents nodes from alternating between suspension and reinstatement without consequence.

The review period consists of one _grace period_ and one _tracking period_. The _grace period_ is given to fix whatever issue is causing the downtime. After the grace period has expired, any offline audits will fall within the scope of the tracking period, and thus will be used in the node's final evaluation. If at the end of the review period, the node is still suspended, it is disqualified. Otherwise, the node is no longer _under review_.

## Rationale

This approach works because it allows us to consider the number of offline audits, but ensure that they are spread out over a period of time. For instance, if a node happens to be offline for 1 hour and unluckily receives an absurdly high amount of audits at that time, it should still be able to recover. It has only affected the score of one window. If we take an example of a tracking period of 30 days and a window size of 24 hours, we can see that a single ruined window should not spell disaster. However, if a node is having bad luck with audits over multiple windows over the tracking period, this seems to indicate that the node is not quite as reliable as we would like. Even then, the addition of suspension mode gives the node a chance to fix its connection issues.

### Alternate approaches

### Audit windows with binary online and offline fields
#### Description
Each window tracks whether a node was offline and/or online for _any_ audits within its timeframe. Use offline-only windows to determine punishment.

#### Discussion
With this route, we could look for consecutive offline-only windows, or we could opt for something similar to the main design of this document where we look for a certain percentage of offline-only windows over a longer period of time.

This design contains the merit that only using 2 bits to indicate online/offline is quite cheap and could save us some DB space.

The main concern I have with this idea is that, because we only look for windows where the online bit is `0` and the offline bit is `1`, as long as a node passes a single audit within any particular window, it would be able to skip the rest of them and go unnoticed. For frequently audited nodes, this could result in a large number of audits which can be avoided.

The main design of this document does not fall victim to the same exploit. Since we keep tallies of offline audits vs total audits per window, there is no ability for offline audits to go unnoticed. 

However, if we can tune the incentives to stay online such that there is very little reason to dodge audits, we might be able to use this idea to save DB space as the network grows.

### Audit windows with separate tallies indicating offline and total audits. The node receives strikes for bad windows
#### Description
This idea is almost identical to the main design of the document. Each window keeps two separate totals of how many offline and total audits occurred within its timeframe. The difference is that rather than finding the average offline percentage per window, we give the node a strike if any window falls below the accepted threshold. If the node receives a set number of strikes over a period of time then there are consequences.

The main concern I had with this idea is that a node going over the acceptable threshold by a small amount receives the same strike as a node which was 100% offline. It seems to me like the node should at least get some credit for the amount it _was_ online.

By keeping track of the offline percentage per window and then averaging the scores together across a longer period, the main design addresses this concern.

## Implementation

### 1) Determine the business requirement for the minimum frequency of audits per node. Implement the necessary changes to make this happen. 

This will determine what window sizes we can work with. Ideally even the least audited nodes should be audited multiple times over the course of a window.

### 2) Add new nullable timestamp columns `offline_suspended` and `under_review` to `nodes` table

### 3) Implement a DB table to store audit history

```sql
CREATE TABLE audit_history (
    node_id BYTEA,
    data BYTEA,
)
```
`data` refers to a serialzed data structure containing the node's audit history.

```
type AuditResults struct {
    Offline int
    Total   int
}

type AuditHistory map[time.Time]AuditResults

```
The map key refers to the start boundary of the window. This can be determined by truncating the current time down to the nearest multiple of the window size.

### 4) Add `AuditHistoryConfig` to overlay config. 

```
type AuditHistoryConfig struct {
    WindowSize       time.Duration `help:"the length of time spanning a single audit window."`
    TrackingPeriod   time.Duration `help:"the length of time to track audit windows for node suspension and disqualification."`
    GracePeriod      time.Duration `help:"The length of time to give suspended SNOs to diagnose and fix issues causing downtime. Afterwards, they will have one tracking period to reach the minimum online score before disqualification."`
    OfflineThreshold float64       `help:"The point above which a node is punished for offline audits. Determined by calculating the percentage of offline audits within each window and finding the average across windows within the tracking period."`
}
```

### 5) Write to audit history and evaluate node standing
The overlay DB method `BatchUpdateStats` is where a node's audit reputation is updated by audit results. It iterates through each node, retrieving its dossier from the `nodes` table and updates its reputation. We also need to update and read information from the node dossier to set and evaluate a node's standing regarding audit windows, so this method is a good place to do this. 

Add a new argument to `BatchUpdateStats` to pass in `AuditWindowConfig`. This gives us the window size parameter required to write to the audit windows table and the tracking period, grace period, and offline threshold parameters required to evaluate whether a node needs to be suspended, reinstated, or disqualified.

For each node, take the `offline_suspended` and `under_review` values from the node dossier. 
Select and unmarshal the node's entry in the audit history table. 
If a value does not exist in the windows map for the current window, this means that the previous window is now complete, and we need to evaluate the node and delete any windows which have fallen out of scope.
We evaluate the node's current standing by finding what percentage of audits were offline per window, then finding the average score across all complete windows within the tracking period.

With this information, there are a number of conditions we need to evaluate:

1) The current offline score is above the OfflineThreshold
    
    1a. The node is under review

    - Reinstate the node if it is suspended.

    - Check if the review period has expired. We can do this by taking the start boundary of the current window and subtracting the tracking period and grace period lengths from it. If this value is greater than the `under_review` value, this means that the node's review period has elapsed and the `under_review` field can be cleared.

    1b. The node is not under review

    - The node is in good standing and does not need to be updated.

2) The current offline score is below the OfflineThreshold

    2a. The node is under review

    - Check if the review period has expired. If so, the `disqualified` column should be set (either to the current time, or the start boundary of the current window since that was the point at which all of the data had been collected)

    - Suspend the node if it is not already suspended. 
        
    2b. The node is not under review

    - Suspend the node and set its `under_review` field in the audit history struct to the current time.

After the evaluation is complete, insert a new window into the Windows map and write the serialized audit history back to the database.

NOTE: We should not implement disqualification right away. It might also be good to implement the `offline_suspended` column, but not use it for anything to begin with. This way we can have a bit of time to observe how well the system works (how many nodes enter suspension, testing the notifications)

### 6) Implement email and node dashboard notifications of offline suspension and under review status
The `NodeStats` protobuf will need to be updated to send and receive these new fields

## Wrapup

Once the design outlined in the document is implemented, other documents detailing the old uptime reputation will need to be edited.

## Open issues

1) Nodes stuck in suspension

    Since a suspended node cannot receive new pieces, and it can only be evaluated for reinstatement after an audit, if it happened to have all of its pieces deleted, it would be stuck in a limbo state where it would never leave suspension.

    This most likely means it is a new node with very few pieces to begin with. We can reduce the likelihood of this happening by requiring a minimum number of windows in order to be evaluated. In other words, rather than suspending a new node for messing up its first day of audits with its one and only piece, we wait until it has a full tracking period of audit windows before evaluating. This way, it hopefully has accumulated a few more pieces, which will reduce the likelihood that they are all deleted and the node is condemned to limbo.

    On the other hand, this could increase repair costs.

2) Nodes alternating in and out of suspension

    A node which is under review can be suspended and reinstated any number of times until the end of the review period. We could end up with a situation where a node is suspended, comes back, gets more pieces, and is suspended again. If the completion of the review period results in disqualification, all of those pieces will become unhealthy.

    An alternative approach might be to simply not give new pieces to nodes under review, whether suspended or not. However, some nodes might think that waiting for the review period to end in order to get more pieces is not worth it. If they shut their node down and start over, all the pieces would need to repaired as well.

3) Network issues

    Failing to connect to a node does not necessarily mean that it is offline.

    Satellite side network issues could results in many nodes being counted as offline.
    One solution for satellite side issues could be that we cache and batch audit history writes. Upon syncing to the DB, we determine the total percentage of offline audits the batch contains. If it is above some threshold, we decide that there must have been some network issues and we either throw out the results or give everyone a perfect score for that period. 

    It will be more difficult to differentiate network problems from real downtime for a single node. 

    We've received some suggestions about retrying a connection before determining that a node is offline. One the one hand, this gives us more confidence that the node is in fact offline. On the other hand, this increases code complexity and decreases audit throughput.

    If we decide not to attempt retries, we should adjust the offline threshold accordingly to account for offline false positives and ensure that even the smallest nodes are still audited enough that any false positives should not pose a real threat.
