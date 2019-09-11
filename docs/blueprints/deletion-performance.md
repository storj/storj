# Deletion Performance

## Abstract

This document describes design for improvments to deletion speed.

## Background

Current design requires uplinks to send deletion information to all the storage nodes. This ends up taking a considerable amount of time.

There are few goals with regards to deletions:

- We need to ensure that uplinks are responsive.
- We need to ensure that storage nodes aren't storing too much garbage, since it reduces overall capacity of the network.
- We need to ensure that satellites aren't storing too much garbage, since that increases running cost of the satellite.

TODO: where exactly are deletes spending time.

## Design

First we can do reduce timeouts for delete requests. Undeleted pieces will eventually get garbage collected, so we can allow some of them to get lost.

The uplink should be able to issue the request without having to wait for the deletion to happen. Currently deletions is implemented as an RPC, instead, use a call without waiting for a response.

Ensure we delete segments in parallel as much as possible.

## Rationale

[A discussion of alternate approaches and the trade offs, advantages, and disadvantages of the specified approach.]

## Implementation

[A description of the steps in the implementation.]

## Open issues (if applicable)

We could probablistically skip deleting pieces. This would minimize the requests that the uplink has to make, at the same time not leaking too much pieces.

We could track how much each storage node is storing extra due not sending deletes. This would allow paying the storage nodes. However, this would still mean that garbage is being kept in the network.