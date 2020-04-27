# Storage Node Downtime Tracking

## Abstract

This document describes a means of tracking storage node downtime and using this information to suspend and disqualify.

## Background

The previous implementation of uptime reputation consisted of a ratio of online audits to offline audits. The problem we encountered was that, due to the frequency of auditing any particular node being directly correlated with the number of pieces it holds, some nodes' reputations would quickly become destroyed over a relatively short period of downtime. To solve this problem we need a system which takes into account not only how many offline audits occur, but _when_ they occur as well.

## Design

The solution proposed here is to use a series of sliding windows to indicate a general timeframe in which offline audits occur. Each window keeps two separate tallies indicating how many online and how many offline audits occurred within its timeframe.

For example, we could have windows with a length of 30 minutes, and if a node is both online and offline for any number of audits, both the online and offline fields of the window will be set to true. By compressing offline audits over the course of the window into a single value we can neutralize the damaging effect of frequent audits. It also gives us a timeframe of when the failures occurred.

Storage node downtime can have a range of causes. For those storage node operators who may have fallen victim to a temporary issue, we want to give them a chance to diagnose and fix it before disqualifying them for good. For this reason, we are introducing suspension as a component of disqualification.

Suspension is implemented by periodically calculating the percentage of audit windows over a tracking period in which only the offline field set. If this percentage is above some tolerated threshold, we suspend node. Once a node is suspended, we give it a grace period to fix any potential issues which are causing it to be offline during audits. After this, the node has a length of time equal to one tracking period to lower its offline audit window percentage within the acceptable range before it is disqualified.

### Database table

We will need a place to store these windows. A table in the satellite database will probably be sufficient.

Table
```sql
CREATE TABLE audit_windows (
    NodeID BYTEA,
    Results BIT(2),
    WindowStart TIMESTAMP,
)
```
The `Results` column is a string of 2 bits where the least significant bit (big endian) corresponds to offline audits, and the most significant bit corresponds to online audits. The `WindowStart` column refers to the start boundary of the window.

API
```
type AuditWindows interface {
    // Write uptime windows from a set of audit responses indicating whether a node was online or offline
    Write(ctx context.Context, nodeIDs map[storj.NodeID]bool, windowLength time.Duration) error
    // GetOffendingNodes returns nodes who have exceeded the allowed offline-only percentage of audit windows
    GetOffendingNodes(ctx context.Context, windowLength time.Duration, maxWindows int) (storj.NodeIDList, error)
    // Cleanup deletes entries which are no longer needed
    Cleanup(ctx context.Context, windowLength time.Duration, maxWindows int) error
}
```

Write
```sql
INSERT INTO audit_windows VALUES (
    $1, $2, 
    CASE WHEN $3::bool IS TRUE THEN B'10'
        ELSE B'01'
    END
)
ON CONFLICT (NodeID, WindowStart)
DO UPDATE
SET results = CASE WHEN $3::bool IS TRUE THEN results | B'10'
        ELSE results | B'01'
    END;
```

GetOffendingNodes
```sql
SELECT node_id FROM (
    SELECT node_id, (count(*)/total) AS offline_percentage
    FROM (
        SELECT node_id, results, count(*) as total
        FROM audit_windows
        WHERE window_start >= $1 AND window_start < $2
        GROUP BY node_id
    ) t
    WHERE results = B'01'
    GROUP BY node_id
)
WHERE offline_percentage > $3
```

Cleanup
```sql
DELETE FROM audit_windows WHERE WindowStart < $1;
```

### A service for writing to and reading from windows

To write and read from these windows we need some configurable values.
- WindowLength: Length of time spanning a single audit window.
- TrackingPeriod: Length of time to track audit windows for suspension and disqualification.
- GracePeriod: The length of time to give suspended SNOs to diagnose and fix issues causing downtime. Afterwards, they will have one tracking period to decrease the offline window fraction before disqualification.
- AllowedOfflinePercentage: The offline-only percentage of audit windows within the tracking period we will tolerate before consequences are levied.

These values need to live somewhere to be passed to the database. Maybe on a service for handling writes and reads on audit windows

```
type Service struct {
    TrackingPeriod           time.Duration
    GracePeriod              time.Duration

    windowLength             time.Duration
    allowedOfflinePercentage float64

    auditWindowsDB           DB
}
```

```
type Service interface {
    // Write audit windows from a set of audit responses indicating whether a node was online or offline
    Write(ctx context.Context, nodeIDs map[storj.NodeID]bool) error
    // GetOffendingNodes returns nodes who have exceeded the allowed offline-only percentage of audit windows
    GetOffendingNodes(ctx context.Context) (storj.NodeIDList, error)
    // Cleanup deletes entries which are no longer needed
    Cleanup(ctx context.Context) error
}
```

#### Write
To write to the database we will give the audit reporter access to this service. The audit reporter receives the audit results of each node for a segment. We can use this information to build a map of nodes and their online/offline status and pass it to the AuditWindows service to write to the database. By truncating the current time to the nearest multiple of the WindowLength on the service, the database can easily determine if any entries corresponding to the current window already exist and insert or update as needed.

#### GetOffendingNodes
When reading, what we want to know is whether any nodes have gone above the allowed offline percentage so we can suspend or disqualify them. To do this, we can call the audit_windows database method, `GetOffendingNodes`, and pass in windowLength and trackingPeriod from the service.

#### Cleanup
In order to avoid the table becoming infinitely large, we need to delete entries after a certain point. We have two options here:
1) Since we only look at windows within the tracking period, any entries beyond this amount are unnecessary and can be deleted.
2) Perhaps we want to keep old entries around longer than necessary in the event that a node wants to dispute their suspension or disqualification. In this case we simply need another configurable value to define how long we want to keep entries.

As mentioned above, we can give the audit reporter access to the service for writes, but when do we read and delete?

### AuditWindows Chore

```
type Chore struct {
    Loop  sync2.Cycle

    auditWindows auditWindows.Service
    cache        overlay.Service
}
```

The job of the Chore is to handle suspending, reinstating, and disqualifying nodes, and cleaning up the audit_windows table on a regular basis. 
The workflow of the chore could look something like this:

1) Get uptime-suspended nodes from overlay cache
2) Get offending nodes from audit_windows
3) Compare these lists to determine outcomes
- Node is currently suspended and is still offending: Check if a length of time equal to the grace period + tracking period has elapsed since the time of suspension. If so, disqualify the node. Otherwise, continue. 
- Node is suspended, but currently not offending: reinstate the node.
- Node is currently offending, but not suspended: suspend the node.
4) Clean table of excess entries

## Rationale

### Alternate approaches

### Route 1: Audit windows with binary online and offline fields
#### Description
Use a series of sliding windows to indicate a general timeframe in which offline audits occur. Each window tracks whether a node was offline and/or online for any audits within its timeframe. Use offline-only windows to determine punishment.
#### Option 1: Punish for consecutive offline-only windows
##### Problems
- If the window is large, frequently audited nodes can skip most of their audits, since they only need to be online for a single audit within the window.
- If the window is small, determining how many consecutive offline windows are allowed is difficult. With a 10 minute window and a consecutive offline window limit 12, a frequently audited node would be punished after 2 hours while a node audited once per day would be punished after 12 days.
- Regardless of the window size, due to the consecutive requirement, a node only needs to be online for 1 audit to break the chain of offline-only windows. It is very easy to dodge many audits this way.

#### Option 2: Main design of this document
##### Problems
- If the window is large, frequently audited nodes can skip most of their audits, since they only need to be online for a single audit within the window.
- The chore, which handles doing the calculations for suspension, reinstatement, DQ, is doing a lot of work. It feels very clunky to me.

---

### Route 2: Audit windows with separate online and offline tallies
#### Description
Each window keeps two separate totals of how many online and offline audits occurred within its timeframe. Use online percentage per window to determine punishment.
#### Option 1: If the online percentage for a window drops below a threshold, the node gets a strike. If it gets too many strikes over a period of time, it is punished.
##### Problems
- For nodes with lower audit frequencies, a single mistake can be irredeemable depending on what the required online percentage is.
- A node falling _just_ short of the required percentage receives the same strike as a node which got 0%. Seems like it should at least get some credit for the amount it _was_ online.

#### Option 2: Use total online audit percentage over a rolling period to determine punishment (dropping them into audit window buckets just saves DB space)
##### Problems
- Variations in audit rate can cause problems

EX:<br>
_(24hr Window | Online/Offline)_<br>
MON 100/0<br>
TUE 50/50<br>
/* a ton of data is deleted from the node */<br>
WED 5/0<br>
THU 5/0<br>
FRI 5/0<br>

The current online percentage is ~76% (165/215)<br>
MON falls out of scope and SAT comes in.

TUES 50/50<br>
/* a ton of data is deleted from the node */<br>
WED 5/0<br>
THU 5/0<br>
FRI 5/0<br>
SAT 5/0<br>

Now the online percentage is ~58% (70/120)<br>
Even though the node has been online for all audits since its last calculation, its score has fallen due to the decreased rate in audits

#### Option 3: Determine the online audit percentage per window. Get the average window score over a rolling period to determine punishment
Following the above example:

_(24hr Window | Online/Offline | Online %)_<br>
MON 100/0 - 100%<br>
TUE 50/50 - 50%<br>
/* a ton of data is deleted from the node */<br>
WED 5/0 - 100%<br>
THU 5/0 - 100%<br>
FRI 5/0 - 100%<br>

The current average window score is ~90% (100 + 50 + 100 + 100 + 100 / 5) _see problems section_<br>
MON falls out of scope and SAT comes in.

TUES 50/50 - 50%<br>
/* a ton of data is deleted from the node */<br>
WED 5/0 - 100%<br>
THU 5/0 - 100%<br>
FRI 5/0 - 100%<br>
SAT 5/0 - 100%<br>

The average window score remains the same even though the sample size has decreased.

##### Problems
- The mathematical reasoning here might be nonsensical. If we're talking about "an average of percentages", the sample size of each percentage must be taken into account. Maybe there's a mathematically correct way of accomplishing this general idea.

## Implementation

WIP
1) Determine the business requirement for minimum frequency of audits per node. Implement the necessary changes to make this happen.

## Wrapup

WIP

## Open issues
