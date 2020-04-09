# Storage Node Downtime Disqualification

## Abstract

This document describes storage node downtime suspension and disqualification.

## Background

The previous implementation of downtime disqualification led to the unfair disqualification of storage nodes. This prompted a halt to downtime disqualification and a new design for [Downtime Tracking](storage-node-downtime-tracking.md).

While the previously mentioned blueprint presents a means of tracking downtime, it leaves out the design of how to use this information for disqualification. That is the purpose of this blueprint.

## Design

### Suspension

Storage node downtime can have a range of causes. For those storage node operators who may have fallen victim to a temporary issue, we want to give them a chance to diagnose and fix it before disqualifying them for good. For this reason, we are introducing downtime suspension as a component of disqualification.

For downtime suspension and disqualification we need a few new configurable values:

- Tracking Period: The length of time into the past for which we measure downtime.
- Allowed Downtime: The amount of downtime within the tracking period we will allow before suspension.
- Grace Period: The length of time to give suspended SNOs to diagnose and fix issues causing downtime. Afterwards, they will have one tracking period to demonstrate an acceptable amount of downtime before disqualification.

When the [downtime tracking system](storage-node-downtime-tracking.md) adds an entry to the nodes_offline_time table, if the node is not already suspended, we check the total amount of downtime for the tracking period, e.g. if the tracking period is 30 days, we sum downtime for the last 30 days. If the total is greater than the allowed downtime, the node is suspended and put into an "under review" state for a length of time equal to the grace period + one tracking period.

Suspended nodes are not selected for uploads, and all pieces they hold are considered unhealthy. This means that we consider these pieces to be unreliable. If the number of healthy pieces for a segment drops below the repair threshold, the segment will be repaired and some, or all, of the unhealthy pieces may be moved onto other healthy nodes. Nodes which previously held these pieces which were "repaired" should eventually be told to put them into the trash by the garbage collection service. Until the segment falls into repair and the piece is given to another node, it is still possible to receive download requests for this piece while in suspension mode.

### Evaluating suspended nodes

To reiterate, when a node's downtime rises above the allowed amount, it is both suspended and put into an "under review" state. We will implement a chore to periodically check nodes that are under review to see if their downtime has fallen within the acceptable range. If so, they are reinstated, but they still remain under review. Once the duration of review ends, the node is evaluated. If it still has a greater downtime than allowed, it is disqualified. Otherwise it is cleared.

## Rationale

### Early reinstatement for good nodes?
An earlier iteration of this document suggested a fixed suspension period, i.e. once suspended, even if the node fixed its downtime issues, it could not be reinstated until the end of the suspension. This was motivated by the idea that continuous evalution of a suspended node's downtime could result in it fluctuating between suspension and reinstatement, thus never remaining in suspension long enough to become disqualified. However, we decided that is is better for the nwtwork as a whole if we allow nodes which have fixed their issues to come back ASAP. However, in order to remedy the potential for a node to avoid disqualification by fluctuating between suspension and reinstatement, we still review the node's downtime at the end of some period.

### Tracking period
One might think to measure downtime within a static window, such as within month boundaries, after which downtime is reset. This introduces a problem. If one day of downtime within the tracking period is allowed, why _not_ shut down on the final day? This is compounded by the fact that any number of nodes could have the same idea, resulting in a potentially significant portion the network going offline at the same time. The trailing tracking period does not fall victim to the same problem. There is no specific timeframe which has a higher incentive for downtime, and any observed downtime will follow the node for the length of the tracking period.

### Revert to using audits for uptime reputation?
This blueprint for disqualification was written to work with the currently implemented downtime tracking system. However, questions have recently been raised as to whether the change from using audits to check downtime to the current 'downtime tracking' system was necessary. The impetus for the transition was that a node could become disqualified by being offline for only a short period of time due to the chance that it could be audited many times in quick succession. That said, auditing does give us some indication of node downtime. Is implementing a new system for tracking downtime worth the additional resources if the audit system is already randomly reaching out to nodes? Is it possible for us to salvage the previous system and account for its flaws? Perhaps. Here is a quick idea for discussion, the details of which would require further investigation:

Scale the _weight_ of the uptime check based on time elapsed since last contact.

The previous uptime reputation system worked much like the current audit reputation. See [Node Selection](node-selection.md) for more information.
> α(n) = λ·α(n-1) + _w_(1+_v_)/2
>
> β(n) = λ·β(n-1) + _w_(1-_v_)/2
>
> R(n) = α(n) / (α(n) + β(n))

In the case of a successful uptime check, _v_ is set to 1. Otherwise, it is set to -1. Previously, _w_, or _weight_, was configured at runtime and thereafter remained static. Thus, all uptime checks carried the same weight. However, should this really be the case?
For example, a node fails two uptime checks within a timespan of 1 minute. The second failure does not give us very much additional information does it? We already knew the node was offline less than one minute ago. If the difference were one hour that information might be more useful in determining uptime.

We might be able to account for this by adjusting _w_ based on how much time has elapsed since last contact.
To do this we'll need a new configurable `Uptime Scale`: a duration against which to scale the weight of an individual uptime check. To demonstrate, let's take an Uptime Scale of one hour. If a node is found to be offline when audited and its last contact is greater than or equal to one hour ago, the weight is scaled to 100%. This check will result in the full reputation punishment. A second failed check occurs 30 minutes later. This is one half the scale: the weight is reduced by 50%. Another failed check occurs one minute later, and the weight is scaled back to 1/60 the punishment.

This implementation still exhibits the problem that, for nodes whose last contact was over one hour ago, we could concievably give them a full punishment even if they were only offline for one minute.

## Implementation

1. Remove old uptime reputation values from codebase:

    uptime_success_count<br>
    total_uptime_count<br>
    uptime_reputation_alpha<br> 
    uptime_reputation_beta<br>

2. Add `downtime_suspended` and `under_review` timestamp columns to the nodes table and rename the `suspended` column to `audit_suspended`

3. Implement new logic in estimation chore to suspend nodes
    - Refactor downtimeTrackingDB method `GetOfflineTime` to check for entries which contain downtime from outside the measurement period

        There is a particular characteristic of downtime tracking to take note of here:<br>
        
        A downtime entry tracked at March 2nd 00:00 contains downtime for some length of time preceding that point. See [downtime tracking](storage-node-downtime-tracking.md)<br>
        
        If we want to measure total downtime starting at March 1st 00:00, and there is a March 1st 01:00 entry indicating 2 hours of downtime, we need to truncate the measured downtime in our calculation in order to avoid unfairly including downtime which occurred outside the tracking period.<br>
        
        We should be able to solve this problem by taking the first entry within the tracking period and checking if the offline time it contains extends beyond the start of the tracking period. If so, truncate it to the start of the tracking period, then sum the rest of the entries as usual.

    - Refactor relevant overlay methods to handle downtime_suspended nodes: `KnownReliable`, `FindNodes`, etc.
    - Add notification to SN dashboard indicating suspension for downtime.

4. Implement email notifications if node becomes suspended

5. Implement evaluation chore to reinstate and disqualify nodes
    - Check total downtime for each suspended node where last_contact_success > last_contact_failure. If the total downtime has fallen below the allowed downtime, the node's suspension is lifted, though it still remains under review.
    - If node.under_review <= now - (grace period + tracking period) it is eligible for evaluation
    - If the node is eligible for evaluation, we must also ensure that the entire tracking period is accounted for. If last contact was a failure and occurred before the end of the tracking period, we might have a window of downtime which has not yet been recorded. Thus, to measure all downtime for the tracking period, we must also wait until last_contact_success > failure OR last_contact_failure >= end of tracking period.

        When these conditions are met, measurements should take care to include all downtime within the period. In addition to the lower bound issue explained above under step 3, we also have an upper bound issue in this case. We may have an entry which exists beyond the upper bound of the tracking period, yet holds some amount of downtime which occured within it.<br>
        <br>
        For example, if we want to measure the total downtime for node A from March 1st 00:00 to March 31st 00:00, and we have an entry from April 1st 01:00 indicating 2 hours of downtime, we must include the one hour of downtime which occurred within the tracking period in our calculation. 

## Wrapup

- The person that implements step 5 above should archive this document.

- The [Disqualification blueprint](disqualification.md), and possibly the whitepaper, will need to be updated to reflect new up/downtime disqualification mechanic.

- Edit [Audit Suspension blueprint](audit-suspend.md) to reflect change from `suspended` to `audit_suspended`

- Link to this document in [Downtime Tracking](storage-node-downtime-tracking.md)

## Open issues

- It is possible for a node to continuously cycle through suspension and reinstatement. How frequently this could happen depends upon the length of the tracking and grace periods. Should there be a maximum number suspensions before disqualification?

