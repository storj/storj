# Node Disqualification

## Abstract

This design doc outlines how we will implement node disqualification, wherein a storage node is deemed permanently ineligible for storing new data via uploads.

Nodes can be disqualified due to audit failures, failures to return data during repair, or poor uptime statistics.

## Main Objectives

1. To stop using and fully demonetize nodes which are not behaving as desired.
2. Efficiently identify and ignore these 'bad' nodes.

## Background

The whitepaper section 4.15 and 4.16 talk about disqualification mode as follows:

> The filtering system is the third subsystem; it blocks bad storage nodes from participating. In addition to simply not having done a sufficient proof of work, certain actions a storage node can take are disqualifying events. The reputation system will be used to filter these nodes out from future uploads, regardless of where the node is in the vetting process. Actions that are disqualifying include: failing too many audits; failing to return data, with reasonable speed; and failing too many uptime checks.
>
> If a storage node is disqualified, that node will no longer be selected for future data storage and the data that node stores will be moved to new storage nodes. Likewise, if a client attempts to download a piece from a storage node that the node should have stored and the node fails to return it, the node will be disqualified. Importantly, storagenodes will be allowed to reject and fail _put_ operations without penalty, as nodes will be allowed to choose which Satellite operators to work with and which data to store.

'Failing to return data' is clarified to mean during an audit or a repair. Failure to return data to uplinks is specifically excluded, as this would imply a robust system of trust that does not currently exist. "Filter these nodes out from future uploads" has been clarified to mean that we want to stop any transactions with these nodes:  upload and download including repair and graceful exit.  In effect, the satellite should do no business with a disqualified node.

'Regardless of... the vetting process' is highlighted to show that both vetted and new nodes may be disqualified.  We explicitly do not want to give new nodes a window where we enforce rules less rigorously.  The data science team's initial whitepapers assumed that reputation should be earned over time.  If this assumption is kept, we will need to develop a sliding-scale algorithm to determine the disqualification cutoff for nodes gaining repuation.  A simpler solution may be to set the initial repuation value (via α0, β0) to above the disqualification cutoff.

> After a storage node is disqualified, the node must go back through the entire vetting process again. If the node decides to start over with a brand-new identity, the node must restart the vetting process from the beginning (in addition to generating a new nodeID via the proof of work system). This strongly disincentivizes storage nodes from being cavalier with their reputation.

Further, the node will be demonetized.

> Provided the storage node hasn't been disqualified, the storage node will be paid by the Satellite for the data it has stored over the course of the month, per the Satellite’s records.

A disqualified SNO should quickly stop participating with a satellite it is disqualified and demonetized on.  However, it may remain in Kademlia as the kademlia network supports multiple satellites.  It will also be found in Node DB / overlay, as nodes are not currently ever removed from that database.  Filtering of Node DB records will be required for most operations.

One option that currently will NOT be allowed for disqualified storage nodes is a Graceful Exit.  "Storage Node Payment and Incentives for V3" describes this feature:

> When a node operator wants to leave the network, if they just shut down the node, the unavailability of the data on the network can contribute to repair costs.  If instead, the node triggers a function to call a Satellite, request new storage nodes to store the pieces stored on the node being shut down, then directly upload those pieces to the new nodes, file repair would be avoided.

## Design

### Use of Disqualified Nodes

Disqualified nodes may be used during download of typical downloads from uplinks or via repair.  Therefore, their IP must be tracked in the overlay.

Disqualified nodes may not be used for upload, therefore they should not be returned from node selection processes of any sort.  There is no reason to update the statistics of disqualified nodes.

The list of disqualified nodes should change infrequently, but could grow large over time.  If in the long run, the list of disqualified nodes becomes very large, it may benefit us to move it to its own data store rather than using a disqualified flag.

### Handling Disqualified Nodes

Disqualification can be handled in our existing SQL implementation by adding a `disqualified` column to the `nodes` table:

```sql
CREATE TABLE nodes (
  id bytea NOT NULL,
  ...
  disqualified timestamp with time zone,
  PRIMARY KEY ( id )
);
```

The type of `disqualified` column is a _timestamp_ because in case that for an unexpected cause several nodes get disqualified, such nodes, and not the ones marked by a normal flow, could be set to not be disqualified, once the causes of such problem be identified.

Existing SQL queries employing logic such as `WHERE audit_success_ratio >= $2 AND uptime_ratio >= $3` would change to `WHERE disqualified IS NULL`.

Existing calls to the DBX `UpdateNodeInfo()` method must set `disqualified` if appropriate.  Care should be employed to not overburden the data structures used to store node info.  In the case of Postgres, these tables may be updated and return their values in a single SQL statement.

### Determining Disqualification

A node is disqualified when its reputation falls below a fixed value.  We are currently envision two distinct reputation check values, one for uptime and another for audit.  These values will represent an idea value minus some standard-deviation.  The proposed system for calculating reputations is based on four values: α0, β0, λ, and v.  Because changing these values will change the expected standard deviation of measurements, the reputation cutoff values will vary as these parameters vary.  At this phase, it is expected that these cutoffs are all configured based on numbers from the data science team.  A node will be disqualified if either the audit or the uptime reputation value falls below their disqualification cutoff value.

## Rationale

Although disqualification is largely an atomic operation that would be handled well by an external hash, the inherent tie-ins with node selection make the above solution the most straightforward.  If we were to refactor node selection in the future, we would likely leave disqualified nodes out of the stats database, leaving them only in the overlay.

## Implementation (Stories)

- Update nodes table using DBX
- Create nodes table migration scripts
- Update node selection SQL (overlaycache.go)
- Update calls to UpdateNodeInfo()
- Refactor tests dependent on offline / unreliable nodes to use disqualification
- Create new disqualification tests as needed
- Send errors to disqualified nodes telling them they're disqualified
- Update tally to demonitize disqualified nodes

## Closed Issues
