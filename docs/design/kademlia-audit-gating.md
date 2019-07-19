# Kademlia Audit Gating

## Abstract
Storage node B is added to storage node A's routing table only if one of the satellites that A trusts have verified that B has a high enough audit and uptime count (not disqualified). If B was not verified by at least one of these satellites, it is added to A's Routing Table Antechamber. 

## Background

### White paper: 4.6.1 Mitigating Sybil attacks
While we’ve adopted the proof-of-work scheme S/Kademlia proposes to partially address Sybil attacks, we extend Kademlia with an application specific integration to further defend our network.


Given two storage nodes, A and B, storage node B is not allowed to enter storage node A’s routing table until storage node B can present a signed message from a Satellite C, a satellite that node A trusts. This signed message must state that B has passed enough audits [and uptime checks] (sections 4.13 and 4.15). This ensures that only nodes with verified disk space have the opportunity to participate in the routing layer.


A node that is allowed to enter routing tables is considered vetted and lookups only progress through vetted nodes. To make sure unvetted nodes can still be found, vetted nodes keep unbounded lists of their unvetted neighbors provided that the XOR distance to all unvetted neighbors is no farther than the farthest of the k-closest vetted neighbors. Unvetted nodes keep their k-nearest vetted nodes up-to-date.


## Goals
1. Add a signed message from Satellites to verify that a Node has been not been disqualified and that it has a high enough audit and uptime count.

2. Create a trusted Satellite list that contains the IDs of Satellites from which Nodes will accept verification signatures of other Nodes.

3. Implement a Routing Table Antechamber: An XOR-ordered data structure in which unverified nodes are entered if successfully contacted.

4. Modify FindNear to return n XOR-closest nodes from the antechamber in addition to those already returned from the routing table.

5. On deployment, avoid complete erasure of the network's routing tables.


## Terminology
**Routing Table Antechamber** - *XOR-ordered temporary holding place for unverified storage nodes*

**Node Neighborhood** - *The k-closest nodes to self where distance is measured by XOR. A node is within the node neighborhood if it is closer than the furthest node in the neighborhood. The vetted node neighborhood is the k-closest nodes that are currently in the Routing Table.*

<img src="./images/kademlia-audit-gating.jpg" alt="node neighborhood" width="500"/>

## Design

1. Satellite Signatures for Node Verification
   - Identities can sign messages already
   - Create a Voucher (message) that satellites can sign that a Node has been verified: qualification status, audit successes, and uptime
   - Audit success ratio and uptime count thresholds are per-satellite
   - The vouchers issued by the satellite should have an expiration on them (tunable by satellite)
   - Nodes are expected to get up to date vouchers

```go
   // Satellite
   // IsVetted returns whether or not the node reaches reputable thresholds
   IsVetted(ctx context.Context, id storj.NodeID, criteria *NodeCriteria) (bool, error)
  
   // Storagenode
   // DB implements storing and retrieving vouchers
   type DB interface {
      // Put inserts or updates a voucher from a satellite
      Put(context.Context, *pb.Voucher) error
      // GetExpiring retrieves all vouchers that are expired or about to expire
      GetExpiring(context.Context) ([]storj.NodeID, error)
      // GetAll returns all vouchers from the table
      GetAll(context.Context) ([]*pb.Voucher, error)
   }
```

2. Trusted Satellites List
   - Create Whitelist/blacklist with an abstraction layer for trusted/untrusted Satellites
   - These lists will live on each Node
   - NB: We are using `config.Storage.WhitelistedSatelliteIDs` "a comma-separated list of approved satellite node id’s" for now

3. Routing Table Antechamber
   - XOR ordered data structure (boltDB bucket)
   - A node can be added if it would be within the vetted node neighborhood
   - When the routing table in populated/refreshed, it checks the vouchers from the nodes it communicates with. If a node doesn’t have any trustworthy vouchers, it cannot enter the main routing table.
   - A Node may enter a Routing Table directly if at first contact it is already verified by a trusted Satellite. 
   - A Node should be removed from a Routing Table if on bucket refresh it no longer provides a trusted, non-expired voucher.
   - Since the antechamber may only contain nodes that would be part of the node neighborhood, if the network grows, the space in the neighborhood will shrink so we must remove antechamber nodes that no longer fit in the neighborhood.
 4. FindNear
    - should return n XOR-closest nodes from the antechamber in addition to its current behavior
    - During processes like Bootstrapping or Kademlia Lookups, we should only call FindNear on verified nodes, not on those that are in the antechamber.

5. Progressively migrate nodes from the current Routing Table into antechamber until verified.
   - Deploy vetting and signing first
   - Deploy antechamber and nodes will naturally shift


## Rationale

To help prevent Sybill Attacks where bad Nodes fill the routing tables, push out good Nodes, and propagate themselves across the network into the majority of routing tables, essentially voiding the DHT.

## Implementation

1. Implement [Satellite Signatures for Node Verification](https://storjlabs.atlassian.net/browse/V3-1726)
   * [part II](https://storjlabs.atlassian.net/browse/V3-1868)
   * [part III](https://storjlabs.atlassian.net/browse/V3-1833)

2. Make [Trusted Satellites List](https://storjlabs.atlassian.net/browse/V3-1727)

3. Deploy steps 1 and 2 to production

4. Create [Routing Table Antechamber](https://storjlabs.atlassian.net/browse/V3-1728)
   * [data structure](https://storjlabs.atlassian.net/browse/V3-1834)

5. [Update FindNear](https://storjlabs.atlassian.net/browse/V3-1729)

## Open issues

Q: Should closer buckets to self get refreshed more frequently?

A: Yes, but not as part of this Epic. See this [ticket](https://storjlabs.atlassian.net/browse/V3-1907)

Q: If a node is removed from the Routing Table, should it be added back to the antechamber?

A: No, it must start the process from the beginning. No special action should be taken.

Q: Should the routing table antechamber have a maximum size?

A: We've decided not for now.


