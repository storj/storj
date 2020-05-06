# Storage Node Downtime Tracking

## Abstract

This document describes a means of tracking storage node downtime and using this information to suspend and disqualify.

## Background

The previous implementation of uptime reputation consisted of a ratio of online audits to offline audits. The problem we encountered was that, due to the frequency of auditing any particular node being directly correlated with the number of pieces it holds, some nodes' reputations would quickly become destroyed over a relatively short period of downtime. To solve this problem we need a system which takes into account not only how many offline audits occur, but _when_ they occur as well.

## Design

The solution proposed here is to use a series of sliding windows to indicate a general timeframe in which offline audits occur. Each window keeps two separate tallies indicating how many offline audits and total audits a particular node received within its timeframe. Once a window is complete, it will be scored by calculating the percentage of total audits for which it was offline. We can average these scores over a trailing period of time, called the _tracking period_, to determine an overall "offline score" to be used for suspension and disqualification. By granting each individual window the same weight in the calculation of the overall average, the effect of any particularly unlucky period can be contained while still allowing us to take the failures into account.

Storage node downtime can have a range of causes. For those storage node operators who may have fallen victim to a temporary issue, we want to give them a chance to diagnose and fix it before disqualifying them for good. For this reason, we are introducing suspension as a component of disqualification.

Once a node's offline score has risen above a set threshold, it is suspended and given a _grace period_ to fix whatever issue is causing its downtime. After the grace period has expired, the node has one full tracking period to reduce its offline score to be within the acceptable range. If so, the node is reinstated. If, after the tracking period has elapsed, the node's downtime is still above the tolerated threshold, it is disqualified.

When a node is suspended it will not receive any new pieces, but can continue to receive download and audit requests for the pieces it currently holds. However, a suspended node's pieces are considered to be unhealthy. If the number of unhealthy pieces in a segment becomes too high, it is repaired, which can result in a suspended node's piece being moved onto a more reliable node.

## Rationale

This approach works because it allows us to consider the number of offline audits, but ensure that they are spread out over a period of time. For instance, if a node happened to be offline for 1 hour and unluckily receive an absurdly high amount of audits in that time, it should be able to recover. It has only affected the score of one window. If we take an example of a tracking period of 30 days and a window size of 24 hours, we can see that a single ruined window should not spell disaster. However, if a node is having bad luck with audits over multiple windows over the tracking period, this seems to indicate that the node is not quite as reliable as we would like. Even then, the addition of suspension mode gives the node a chance to fix its connection issues.

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
This idea is almost identical to the main design of the document. Each window keeps two separate totals of how many offline and total audits occurred within its timeframe. The difference is that rather than finding the average offline percentage per window, we give the node a strike if any window falls below the accepted threshold. If the node receives a set number of strikes over a period time then there are consequences.

The main concern I had with this idea is that a node going over the acceptable threshold by a small amount receives the same strike as a node which was 100% offline. It seems to me like the node should at least get some credit for the amount it _was_ online.

By keeping track of the offline percentage per window and then averaging the scores together across a longer period, the main design addresses this concern.

## Implementation

### 1) Determine the business requirement for minimum frequency of audits per node. Implement the necessary changes to make this happen. 

This will determine what window sizes we can work with.

### 2) Add new timestamp columns, `offline_scored_at` and `offline_suspended`, to `nodes` table
We do not need to evaluate a node's standing every time it is audited because we are only interested in complete windows. Therefore, we only need to evaluate a node's standing once per window. If `offline_scored_at` does not fall within the current window then we need to set `offline_scored_at` to the current time and evaluate whether the node needs to be suspended, reinstated, or disqualified.

### 3) Implement a DB table to store audit windows

```sql
CREATE TABLE audit_windows (
    node_id BYTEA,
    offline INT,
    total INT,
    window_start TIMESTAMP,
)
```
The `window_start` column refers to the start boundary of the window. This can be determined by truncating the current time down to the nearest multiple of the window size.

### 4) Add `AuditWindowConfig` to overlay config. 

```
type AuditWindowConfig struct {
    WindowSize       time.Duration `help:"the length of time spanning a single audit window."`
    TrackingPeriod   time.Duration `help:"the length of time to track audit windows for node suspension and disqualification."`
    GracePeriod      time.Duration `help:"The length of time to give suspended SNOs to diagnose and fix issues causing downtime. Afterwards, they will have one tracking period to reach the minimum online score before disqualification."`
    OfflineThreshold float64       `help:"The point above which a node is punished for offline audits. Determined by calculating the percentage of offline audits within each window and finding the average across windows within the tracking period."`
}
```

### 5) Write to audit windows and evaluate node standing. 
The overlay DB method `BatchUpdateStats` is where a node's audit reputation is updated by audit results. It iterates through each node, retrieving its dossier from the `nodes` table and updates its reputation. We also need to update and read information from the node dossier to set and evaluate a node's standing regarding audit windows, so this method is a good place to do this. 

Add a new argument to `BatchUpdateStats` to pass in `AuditWindowConfig`. This gives us the window size parameter required to write to the audit windows table and the tracking period, grace period, and offline threshold parameters required to evaluate whether a node needs to be suspended, reinstated, or disqualified.

For each node, take the `offline_suspended` and `offline_scored_at` values from the node dossier. 
Determine whether we need to evaluate the node's standing (see implementation step 2 above).
If so, evaluate the node's current standing by scoring each window by finding what percentage of audits were offline per window, then finding the average score across all complete windows within the tracking period.

```sql
SELECT SUM(offline_score)/COUNT(*)
FROM (
    SELECT (offline/total) AS offline_score
    FROM audit_windows
    WHERE node_id = $1 AND window_start >= $2 AND window_start < $3
) t
```

With this information, there are a number of conditions we need to evaluate:

1) The node is _not_ currently suspended

    1A. The average offline score is below the `OfflineThreshold`

        The node is in good standing and does not need to be updated.

    1B. The average offline score is above the `OfflineThreshold`
    
        Suspend the node by setting `offline_suspended` to the start of the current window.

2) The node _is_ currently suspended 

   2A. The average offline score is below the `OfflineThreshold`

        We need to check if the node needs to be disqualified. We can do this by taking the start boundary of the current window and subtracting the tracking period and grace period lengths from it. If this value is greater than the `offline_suspended` value, this means that the node's time limit to fix its offline score has expired and the `disqualified` column should be set (either to the current time, or the start boundary of the current window since that was the point at which all of the data had been collected)

    2B. The average offline score is above the `OfflineThreshold`
    
        Reinstate the node by setting `offline_suspended` to `null` 

After the evaluation is complete, insert or update the appropriate window in the audit_windows table.

### 6) Implement email and node dashboard notifications of offline suspension

### 7) Create a chore to delete audit windows after a certain time 

## Wrapup

Once the design outlined in the document is implemented, other documents detailing the old uptime reputation will need to be edited.

## Open issues

Since a suspended node cannot receive new pieces, and it can only be evaluated for reinstatement after an audit, if it happened to have all of its pieces deleted, it would be stuck in a limbo state where it would never leave suspension.

This most likely means it was a new node with very few pieces to begin with. We can reduce the likelihood of this happening by requiring a minimum number of windows in order to be evaluated. In other words, rather than suspending a new node for messing up its first day of audits with its one and only piece, we wait until it has a full tracking period of audit windows before evaluating. This way, it hopefully has accumulated a few more pieces, which will reduce the likelihood that they are all deleted and the node is condemned limbo.

On the other hand, this could increase repair costs.