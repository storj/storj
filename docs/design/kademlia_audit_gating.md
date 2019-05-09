# Title: Kademlia Audit Gating

## Abstract

TODO

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
**Routing Table Antechamber** - *Definition TODO*

**Node Neighborhood** - *Definition TODO*

## Design

1. Satellite Signatures for Node Verification
    - Identities can sign messages already
    - Message (protobuf) to sign that Satellite A says Node B has been vetted
    - Get info about audits to add to the message

2. Trusted Satellites List
    - Whitelist/blacklist w abstraction layer for trusted/untrusted Satellites

3. Routing Table Antechamber
    - XOR ordered data structure 
    - A node can be added if it would be within vetted node neighborhood
    - Nodes are moved to the Routing Table once both successfully re-contacted and verified
        - Once a node has been verified, it broadcasts its new status to try to join routing tables
    - Only contains nodes within the XOR range of the closest k nodes from self. Nodes are moved to the Routing Table once both successfully re-contacted and verified. A Node may enter the Routing Table directly if at first contact it is already verified by a trusted Satellite.  
    Node gets kicked out of RT if disqualified
    - If the network grows, the space in your neighborhood shrinks, remove antechamber nodes that no longer fit in this neighborhood
  
4. FindNear
    - also returns x XOR-closest nodes from the antechamber: 
    - Only call find near on the nodes that are verified from satellites you trust

5. Progressively migrate nodes from the current Routing Table into antechamber until verified. 
    - Deploy vetting and signing first
    - Deploy antechamber and nodes will naturally shift


## Rationale

TODO

## Implementation

1. Implement [Satellite Signatures for Node Verification](https://storjlabs.atlassian.net/browse/V3-1726)

2. Deploy step 1 to production

3. Make [Trusted Satellites List](https://storjlabs.atlassian.net/browse/V3-1727)

4. Create [Routing Table Antechamber](https://storjlabs.atlassian.net/browse/V3-1728)

5. [Update FindNear](https://storjlabs.atlassian.net/browse/V3-1729)

## Open issues 

Q: Should closer buckets to self get refreshed more frequently?

A: ?

Q: Should the routing table antechamber have a maximum size? 

A: We've decided not for now.
