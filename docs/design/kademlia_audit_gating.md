# Title: Kademlia Audit Gating

## Abstract



## Background

### White paper: 4.6.1 Mitigating Sybil attacks
While we’ve adopted the proof-of-work scheme S/Kademlia proposes to partially address Sybil attacks, we extend Kademlia with an application specific integration to further defend our network. 


Given two storage nodes, A and B, storage node B is not allowed to enter storage node A’s routing table until storage node B can present a signed message from a Satellite C that storage node A trusts claiming that B has passed enough audits that C trusts it (sections 4.13 and 4.15). This ensures that only nodes with verified disk space have the opportunity to participate in the routing layer.


A node that is allowed to enter routing tables is considered vetted and lookups only progress through vetted nodes. To make sure unvetted nodes can still be found, vetted nodes keep unbounded lists of their unvetted neighbors provided that the XOR distance to all unvetted neighbors is no farther than the farthest of the k-closest vetted neighbors. Unvetted nodes keep their k-nearest vetted nodes up-to-date.


## Goals
1. A Satellite-Node signature service that authenticates which Nodes have passed enough audits (CPU + Disk Space).
2. Nodes have a trusted Satellite list that contains the IDs of Satellites from which they will accept Node verification signatures.
3. Routing Table Antechamber: XOR ordered data structure where unverified nodes are entered if successfully contacted. Only contains nodes within the XOR range of the closest k nodes from self. Nodes are moved to the Routing Table once both successfully re-contacted and verified. A Node may enter the Routing Table directly if at first contact it is already verified by a trusted Satellite.
4. FindNear also returns x XOR-closest nodes from the antechamber
5. Progressively migrate nodes from the current Routing Table into antechamber until verified. Need to ensure this is done at a slow enough rate to allow for verification of nodes before Routing Tables are emptied completely.


## Terminology
Routing Table Antechamber - Definition
Node Neighborhood - Definition

## Design

1. A Satellite-Node signature service that authenticates which Nodes have passed enough audits (CPU + Disk Space).
    - Identities can sign messages already
    - Message (protobuf) to sign that Satellite A says Node B has been vetted
    - Get info about audits to add to the message
2. Nodes have a trusted Satellite list that contains the IDs of Satellites from which they will accept Node verification signatures.
    - Whitelist/blacklist w abstraction layer for trusted/untrusted Satellites
3. Routing Table Antechamber
    - XOR ordered data structure 
    - A node can be added if it would be within vetted node neighborhood
    - Nodes are moved to the Routing Table once both successfully re-contacted and verified
        - Once a node has been verified, it broadcasts its new status to try to join routing tables
4. FindNear also returns x XOR-closest nodes from the antechamber
5. Progressively migrate nodes from the current Routing Table into antechamber until verified. 
    - Deploy vetting and signing first
    - Deploy antechamber and nodes will naturally shift


## Rationale

[A discussion of alternate approaches and the trade offs, advantages, and disadvantages of the specified approach.]

## Implementation

1.
2.
3.
4.
5.

## Open issues 

Q: Should the routing table antechamber have a maximum size? 
A: We've decided not for now.
