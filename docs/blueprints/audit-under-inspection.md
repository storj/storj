# Storagenode "Under Inspection" Blueprint

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
The "under inspection" feature will be very similar to the already existing audit disqualification functionality. We will add a concept of "unknown audit reputation", with alpha and beta values that are very similar to those used for [audit reputation](./node-selection.md):

> α(n) = λ·α(n-1) + _w_(1+_v_)/2
>
> β(n) = λ·β(n-1) + _w_(1-_v_)/2
>
> R(n) = α(n) / (α(n) + β(n))

Initial values for alpha (α(0)) and beta (β(0)) will be the same as those for normal audit reputation.

Values for the forgetting factor (λ) and weight (_w_) will be the same as those for normal audit reputation. The threshold used to trigger "under inspection" mode will be the same as the threshold for audit disqualification. When R(n) falls below this threshold, the node goes into "under inspection".

For every audit success, the equations above will be updated with _v_=1. For unknown audits, the equations above will be updated with _v_=-1. For any other situation, the equations are not updated.

The only difference between these alpha/beta values and the currently existing audit alpha/beta values is that the new values keep track of the relationship between audit successes and _unknown_ audit errors, while the old values keep track of the relationship between audit successes and _failed_ audit errors.

When the "unknown audit reputation" falls below the threshold, the node will be flagged as "under inspection".

### Satellite Treatment of an "Under Inspection" Node
When a node is under inspection, it can still be used to for downloading data, but new data will not be uploaded to it until it passes enough audits so that its "unknown audit reputation" returns to above the threshold. This is very similar to the behavior of a [gracefully exiting node](./storagenode-graceful-exit/overview.md):

Permitted requests: `GET`, `GET_AUDIT`, `DELETE`

Unpermitted requests: `PUT`, `PUT_REPAIR`, `PUT_GRACEFUL_EXIT`, `GET_REPAIR`

#### Audit Service
The satellite will not treat under inspection nodes any differently during audits. Audits will continue being conducted on them, which allows for them to exit the "under inspection" status if the node stops returning unknown errors.

#### Repair Service
The checker and repairer will count "under inspection" nodes as "unhealthy", along with disqualified and offline nodes, meaning these nodes will be removed from the segment on a successful repair.

#### Overlay Service
When nodes are selected for new data (uplink upload, repair upload, or graceful exit transfer), the overlay service will not include nodes that are under inspection in its query.

When the overlay cache receives a request to update a node's stats, if it was for an unknown or successful audit, it will update the unknown audit alpha/beta values, calculate reputation, and set or unset the "under inspection" timestamp for that node (if applicable). It should not update normal audit alpha/beta for an unknown audit, and it should not update unknown audit alpha/beta for a failed audit.

### Notification of Storagenode
Similarly to how disqualification is handled, the storagenode dashboard should include information about whether a node is "under inspection" for a particular satellite. This will allow the storagenode operator to check their logs, make any necessary changes, and hopefully exit the "under inspection" state after a few more rounds of audits.

Depending on the complexity involved, we may want to have a satellite chore that runs on some interval and emails storagenode operators if their node is under inspection. This would allow operators to act more quickly and would not depend on them checking their dashboard.

### Disqualification
If a storagenode remains in the "under inspection" state for longer than a configured interval, we should disqualify them. This can occur in the overlay service, or if we have a satellite chore for the under inspection state (see above section about emailing operators), we can do the check and update there.

## Implementation
1. Add unknown audit alpha/beta float fields to `nodes` table on satellite. Also add nullable `under_inspection` timestamp field.
2. Update overlay cache `UpdateStats` to update the new fields when audits are run. Care should be taken to not update old alpha/beta for unknown audits, and not update new alpha/beta for failed audits. Also update audit service to report unknown audits to the overlay.
3. Update overlay cache `SelectStorageNodes` and `SelectNewStorageNodes` to exclude nodes where `under_inspection` is not `NULL`. Also update `KnownUnreliableOrOffline`, `KnownReliable`, and `Reliable` (and any other relevant overlay functions if they are not covered here).
4. Update the storagenode dashboard to include information about being under inspection (this includes updating the satellite to report this information to the storagenode).
5. Disqualify nodes that are under inspection for longer than some configured threshold.
6. (optional?) Send emails to notify storagenodes of being placed under  inspection.

## Wrapup
* The team working on the implementation must archive this document once finished.

## Open Issues
* Is there any reason we might want to have different values for lambda, _w_, "DQ" threshold for under inspection than for normal audit reputation?
* How important is sending an email out to storagenode operators about entering the "under inspection" state? For a first implementation, is it sufficient to simply display this on the dashboard?
* Is "soft disqualification" a better name for this feature than "under inspection"?