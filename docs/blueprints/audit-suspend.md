# Storagenode "Suspension" State Blueprint

## Introduction

Currently, when a storagenode is audited for an erasure share, there are five possible outcomes:

1. Success: The node responds with the correct data
2. Failure: The node responds with incorrect data
3. Offline: The node cannot be contacted
4. Contained: The node can be contacted, but the connection times out before all the data can be received by the satellite
5. Unknown: The node responds with any other error

Only cases 1 and 2 directly affect a node's audit reputation, which can cause disqualification.

When the [downtime tracking service](./storage-node-downtime-tracking.md) is fully implemented, case 3 can indirectly cause a disqualification.

Case 4 can also indirectly cause disqualification, since a node placed in containment mode will be re-audited at some point with the same 5 potential outcomes.

Case 5 is the only situation where there is currently no potential penalty for responding to an audit with some type of error. Fortunately, having this case has allowed us to find, diagnose, and fix several problems with storagenodes, increasing network durability. Unfortunately, it allows us to perceive nodes that consistently respond to audits with unknown errors as "healthy", giving us an inflated view of durability.

Adding the storagenode preflight checks have allowed us to eliminate the root causes of a significant portion of "unknown" audit errors, but there will always remain some cases of such errors, and most of these cases - such as the cases fixed with the preflight checks - will be related to user-error regarding storagenode configuration.
Because we want to ensure maximum durability for our files, we want to avoid disqualifying storagenodes for simple configuration mistakes that cause them to return errors during audits, but we also want to avoid making these types of errors consequence-free.

Therefore, our goal is to implement some sort of penalty for unknown audit errors, but one that does not lead directly to disqualification, and gives the storagenode operator an opportunity to fix whatever issue is causing the errors.  

## Design Overview
The "suspension" feature will be very similar to the already existing audit disqualification functionality. We will add a concept of "unknown audit reputation", with alpha and beta values that are very similar to those used for [audit reputation](./node-selection.md):

> α(n) = λ·α(n-1) + _w_(1+_v_)/2
>
> β(n) = λ·β(n-1) + _w_(1-_v_)/2
>
> R(n) = α(n) / (α(n) + β(n))

Initial values for alpha (α(0)) and beta (β(0)) will be the same as those for normal audit reputation.

Values for the forgetting factor (λ) and weight (_w_) will be the same as those for normal audit reputation. The threshold used to trigger suspension will be the same as the threshold for audit disqualification. When R(n) falls below this threshold, the node goes into suspension.

For every audit success, the equations above will be updated with _v_=1. For unknown audits, the equations above will be updated with _v_=-1. For any other situation, the equations are not updated.

The only difference between these alpha/beta values and the currently existing audit alpha/beta values is that the new values keep track of the relationship between audit successes and _unknown_ audit errors, while the old values keep track of the relationship between audit successes and _failed_ audit errors.

When the "unknown audit reputation" falls below the threshold, the node will be flagged as suspended.

### Satellite Treatment of a Suspended Node
When a node is suspended, it can still be used to for downloading data, but new data will not be uploaded to it until it passes enough audits so that its "unknown audit reputation" returns to above the threshold. This is very similar to the behavior of a [gracefully exiting node](./storagenode-graceful-exit/overview.md):

Permitted requests: `GET`, `GET_AUDIT`, `DELETE`

Unpermitted requests: `PUT`, `PUT_REPAIR`, `PUT_GRACEFUL_EXIT`, `GET_REPAIR`

#### Audit Service
The satellite will not treat suspended nodes any differently during audits. Audits will continue being conducted on them, which allows for them to exit the suspended status if the node stops returning unknown errors.

#### Repair Service
The checker and repairer will count suspended nodes as "unhealthy", along with disqualified and offline nodes, meaning these nodes will be removed from the segment on a successful repair.

#### Overlay Service
When nodes are selected for new data (uplink upload, repair upload, or graceful exit transfer), the overlay service will not include nodes that are suspended in its query.

When the overlay cache receives a request to update a node's stats, if it was for an unknown or successful audit, it will update the unknown audit alpha/beta values, calculate reputation, and set or unset the suspended timestamp for that node (if applicable). It should not update normal audit alpha/beta for an unknown audit, and it should not update unknown audit alpha/beta for a failed audit.

### Notification of Storagenode
Similarly to how disqualification is handled, the storagenode dashboard should include a prominent banner indicating that the node is suspended for a particular satellite. This will allow the storagenode operator to check their logs, make any necessary changes, and hopefully exit the suspended state after a few more rounds of audits.

The node operator should also be able to access the suspended state over the API, and upon suspension, they should receive a notification under the bell menu in the node dashboard.

Depending on the complexity involved, we may want to have a satellite chore that runs on some interval and emails storagenode operators if their node is suspended. This would allow operators to act more quickly and would not depend on them checking their dashboard. The same system could be used to email nodes about disqualification. For now, this is lower priority compared to the dashboard notifications, but we should investigate it since some storagenode operators would prefer it.

### Disqualification
If a storagenode remains in the suspended state for longer than a configured interval ("suspension grace period"), we should disqualify them. This should only occur if the interval has passed AND the node has a failed or unknown audit error. This prevents us from disqualifying a node who has fixed the issue causing errors near the end of the grace period - as long as they continue passing audits, they will leave the suspension state eventually, even if the grace period has expired. 

### Becoming Unsuspended
If the storagenode operator fixes the issue causing unknown audit errors within the suspension grace period, they will begin passing audits again, causing their unknown audit reputation to increase. Once the unknown audit reputation rises above the suspension threshold, the node becomes unsuspended and is considered healthy again. The node retains all of its previous characteristics from before being suspended, including its vetted status and its normal audit reputation (not to be confused with unknown audit reputation).

## Implementation
1. Add unknown audit alpha/beta float fields to `nodes` table on satellite. Also add nullable `suspended` timestamp field.
2. Update overlay cache `UpdateStats` to update the new fields when audits are run. Care should be taken to not update old alpha/beta for unknown audits, and not update new alpha/beta for failed audits. Also update audit service to report unknown audits to the overlay.
3. Update overlay cache `SelectStorageNodes` and `SelectNewStorageNodes` to exclude nodes where `suspended` is not `NULL`. Also update `KnownUnreliableOrOffline`, `KnownReliable`, and `Reliable` (and any other relevant overlay functions if they are not covered here).
4. Update the storagenode dashboard to include information about being suspended (this includes updating the satellite to report this information to the storagenode).
5. Add configuration for suspension grace period. Update `UpdateStats` to disqualify nodes who fail an audit and have been suspended for longer than the grace period.
6. Investigate complexity of sending emails to notify storagenodes of being placed in suspension. Implement depending on outcome of investigation.

## Wrapup
* The team working on the implementation must archive this document once finished.

## Open Issues
* Is there any reason we might want to have different values for lambda, _w_, "DQ" threshold for suspension than for normal audit reputation?
* How important is sending an email out to storagenode operators about entering the suspension state? For a first implementation, is it sufficient to simply display this on the dashboard?

