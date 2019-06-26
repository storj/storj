# Reputation and Node Selection

## Abstract

Node selection is the process wherein the set of all possible storage nodes is reduced by the satellite for uploading segments.  Node selection applies to new file uploads via an uplink, as well as repair traffic from a satellite.  The node selection processes endeavors to fairly distribute upload traffic among storage nodes.  Node selection takes into consideration how new a node is, the overall performance characteristic of a storage node as characterized by its reputation score, and the IP address of each node.

## Background

The white paper section 4.15 describes a 'preferences' system used in node selection, based on reputation:

> After disqualified storage nodes have been filtered out, remaining statistics collected during audits will be used to establish a preference for better storage nodes during uploads. These statistics include performance characteristics such as throughput and latency, history of reliability and uptime, geographic location, and other desirable qualities. They will be combined into a load-balancing selection process, such that all uploads are sent to qualified nodes, with a higher likelihood of uploads to preferred nodes, but with a non-zero chance for any qualified node.  Initially, we’ll be load balancing with these preferences via a randomized scheme, such as the Power of Two Choices, which selects two options entirely at random and then chooses the more qualified between those two.
>
> On the Storj network, preferential storage node reputation is only used to select where new data will be stored, both during repair and during the upload of new files, unlike disqualifying events.  If a storage node’s preferential reputation decreases, its file pieces will not be moved or repaired to other nodes.

The existing reputation-like system uses uptime and audit responses.  It does not currently consider geographic location, throughput, or latency.  In addition to factors which affect reputation, there are other factors in node selection.  These considerations currently include IP address, advertised available bandwidth, advertised available disk space, software version compatibility, and whether the node appeared to be online in the latest communication with the satellite.

One final factor involved in node selection is node 'vetting.'  During upload


## Design

Separate reputation scores will be maintained for audits and uptime.  Two sets of configurations will be stored, two different reputation scores will be calculated, etc..  

The reputation _R(n)_ is calculated based on some measured success _v_, two persisted values a and β, and fixed configuration values λ and _w_.  

> α(n) = λ·α(n-1) + _w_(1+_v_)/2
>
> β(n) = λ·β(n-1) + _w_(1-_v_)/2
>
> R(n) = α(n) / (α(n) + β(n))

Initial values for α and β - α0 and β0 - will also be configuration values.  While _v_ may need to vary depending on our "easing" implementation, it will likely vary algorithmically and should not require configuration.  The initial implement may simply assume that _v_ = 1 on success and _v_ = -1 on failure.

Existing codes which updates database audit / uptime success-counts and ratios must be updated to track audit / uptime α and β values.  `TotalAuditCount` and `TotalUptimeCount` will still be needed to determine if the node is new or vetted.

The node selection SQL queries will also change.  Twice as many nodes must returned from these functions to satisfy the "Power of Two Choices" requirement, which gives preference to nodes with better reputation.  For every two nodes returned, the one with the higher reputation scores will be selected returned and the other discarded.

Note that the initial implementation has two different reputation statistics:  audit and uptime.  For the purposes of node selection, we assume that these two reputation can be combined by scaling one of them by some constant.  We further assume that different operations may weigh these reputations differently. For instance, repair may be more concerned that a node is reliable than it is speedy.  New file uploads coming from an uplink may have different criteria.   The initial configuration should include a `uptime_repair_weight`, `audit_repair_weight`, `uptime_uplink_weight`, and `audit_uplink_weight` constants.

> Total Repair Reputation = uptime_repair_weight · uptime R(n) + audit_repair_weight · audit R(n)
>
> Total Uplink Reputation = uptime_uplink_weight · uptime R(n) + audit_uplink_weight · audit R(n)

This design may be refined in the future to prefer storage nodes based on speed, geography, etc..

### Database changes

```DBX
model node (
...
	field audit_reputation_alpha  float64   ( updatable )
	field audit_reputation_beta   float64   ( updatable )
	field total_audit_count       int64     ( updatable )
	field uptime_reputation_alpha float64   ( updatable )
	field uptime_reputation_beta  float64   ( updatable )
	field total_uptime_count      int64     ( updatable )
...
)
```

## Rationale

The Storj Data Science team has currently published two papers on the design of our reputation score:
[Reputation Scoring](https://github.com/storj/datascience/blob/8b02707dceedd4ce20d699a5a9791ce589b303bd/reputation/Reputation_Scoring_Framework_Highlevel.pdf) and [Extending Audit/Uptime Success Ratios](
https://github.com/storj/datascience/blob/2ec82c9ec89263d9348798e8a5d50a7b62782110/reputation/extending%20ratios%20to%20reputation/extending%20ratios%20to%20reputation.pdf).  

These papers put forth a model where reputation chance be determined based on previous 'shape' values α and β, a forgetting factor λ, single value feedback _v_, and a normalization weight _w_.

> We suggest one minor modification from [1] to how _v_ is selected. ... we propose starting with small (but negative) values for failures, and small (but positive) values for successes.... This has the benefit of "easing" a new node's reputation towards its "true" reputation. ... We start with α0 = 1 and β0 = 1 for two reasons:  first, because 0/0 is undefined; second, this assigns new nodes a reputation score of 0.5...

Implementing α0 = β0 = 1 as described above would require some type of relaxation of disqualification criteria for new nodes.  The alternative recommended in this document is to initialize α0 and β0 such that α0 / (α0 + β0) is greater than the disqualification cutoff and less than or equal to 1 (the maximum reputation).  The easing of _v_ described above may lessen in importance, given these starting values.  For the initial implementation, we assume that _v_ is 1 on success and -1 on failure.

## Implementation

* Create configuration elements for audit_alpha0, audit_beta0, audit_λ, audit_w, audit_repair_weight, and audit_uplink_weight
* Create configuration elements for uptime_alpha0, uptime_beta0, uptime_λ, uptime_w, uptime_repair_weight, and uptime_uplink_weight
* Alter DBX model removing audit_success_count, audit_success_ratio, uptime_success_count, uptime_ratio
* Alter DBX model adding audit_reputation_alpha, audit_reputation_beta, uptime_reputation_alpha, uptime_reputation_beta
* Create migration scripts for SQL table changes
* Alter SQL node selection queries to consider new values
* Alter SQL node selection queries to return 2x more nodes
* Implement "Power of Two Choices" logic in node selection query
* Update disqualification code to use reputation instead of checking ratios


## Open issues (if applicable)

