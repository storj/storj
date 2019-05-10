# Kademlia Audit Gating

## Abstract

StorageNode B is added to StorageNode A's routing table only if StorageNode A trusts Satellite C and Satellite C as verified StorageNode B has a high enough identity-generate difficulty (CPU proof-of-work) and disk space. If StorageNode B does not have this verification, it is added to StorageNode A's Routing Table Antechamber.

## Background

### White paper: 4.6.1 Mitigating Sybil attacks
While we’ve adopted the proof-of-work scheme S/Kademlia proposes to partially address Sybil attacks, we extend Kademlia with an application specific integration to further defend our network. 


Given two storage nodes, A and B, storage node B is not allowed to enter storage node A’s routing table until storage node B can present a signed message from a Satellite C that storage node A trusts claiming that B has passed enough audits that C trusts it (sections 4.13 and 4.15). This ensures that only nodes with verified disk space have the opportunity to participate in the routing layer.


A node that is allowed to enter routing tables is considered vetted and lookups only progress through vetted nodes. To make sure unvetted nodes can still be found, vetted nodes keep unbounded lists of their unvetted neighbors provided that the XOR distance to all unvetted neighbors is no farther than the farthest of the k-closest vetted neighbors. Unvetted nodes keep their k-nearest vetted nodes up-to-date.


## Goals
1. Add a signed message from Satellites to authenticate whether a Node has a high enough ID generation difficulty and disk space.

2. Create a trusted Satellite list that contains the IDs of Satellites from which Nodes will accept verification signatures of other Nodes.

3. Implement a Routing Table Antechamber: An XOR-ordered data structure in which unverified nodes are entered if successfully contacted. 

4. Modify FindNear to return n XOR-closest nodes from the antechamber in addition to those already returned from the routing table.

5. On deployment, avoid complete erasure of the network's routing tables.


## Terminology
**Routing Table Antechamber** - *XOR-ordered temporary holding place for unverified storagenodes*

**Node Neighborhood** - *The k-closest nodes to self where distance is measured by XOR. A node is within the node neighborhood if it is closer than the furthest node in the neighborhood. The vetted node neighborhood is the k-closest nodes that are currently in the Routing Table.*

![Node Neighborhood](docs/design/kad-audit.jpg "Node Neighborhood")


## Design

1. Satellite Signatures for Node Verification
    - Identities can sign messages already
    - Create a Message (protobuf) to sign that Satellite C says Node B has been vetted
    - Get info about the Node's difficulty and disk space to add to the message

2. Trusted Satellites List
    - Create Whitelist/blacklist with an abstraction layer for trusted/untrusted Satellites
    - These lists will live on each Node

3. Routing Table Antechamber
    - XOR ordered data structure (perhaps an ordered slice)
    - A node can be added if it would be within the vetted node neighborhood
    - Once a node has been verified, it broadcasts its new status to try to join routing tables
    - A Node may enter a Routing Table directly if at first contact it is already verified by a trusted Satellite.  
    - A Node may be removed from a Routing Table if on bucket refresh it no longer meets qualifications
    - Since the antechamber may only contain nodes that would be part of the node neighborhood, if the network grows, the space in the neighborhood will shrinks so we must remove antechamber nodes that no longer fit in the neighborhood.
  
4. FindNear
    - should return n XOR-closest nodes from the antechamber in addition to its current behavior
    - During processes like Bootrapping or Kademlia Lookups, we should only call FindNear on verified nodes, not on those that are in the antechamber.

5. Progressively migrate nodes from the current Routing Table into antechamber until verified. 
    - Deploy vetting and signing first
    - Deploy antechamber and nodes will naturally shift


## Rationale

To help prevent Sybill Attacks where bad Nodes fill the routing tables and push out good Nodes.

## Implementation

1. Implement [Satellite Signatures for Node Verification](https://storjlabs.atlassian.net/browse/V3-1726)

2. Deploy step 1 to production

3. Make [Trusted Satellites List](https://storjlabs.atlassian.net/browse/V3-1727)

4. Create [Routing Table Antechamber](https://storjlabs.atlassian.net/browse/V3-1728)

5. [Update FindNear](https://storjlabs.atlassian.net/browse/V3-1729)

## Open issues 

Q: Should closer buckets to self get refreshed more frequently?

A: ?

Q: If a node is removed from the Routing Table, should it be added back to the antechamber?

A: ?

Q: Should the routing table antechamber have a maximum size? 

A: We've decided not for now.
