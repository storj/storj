# Storage Node Graceful Exit - Protocol

## Abstract

This document describes the Graceful Exit protocal for Storage Node to Satellite communications.

## Background

For a Storage Node to complete a Graceful Exit, we need a way for Storage Nodes to:
- Initiate a Graceful Exit
- Request order limits for pieces that need to be transferred
- Transfer pieces to new nodes
- Send successful transfers to the Satellite for verification and segment updates
- Send failed transfers for potential reprocessing
- Recover from interrupted transfers
- Receive confirmation when the Graceful Exit is completed

## Design

[A precise statement of the design and its constituent subparts.]

## Rationale

[A discussion of alternate approaches and the trade offs, advantages, and disadvantages of the specified approach.]

## Implementation
TODO: Discuss review comments.
- "Having initiate exit, get put orders and process put orders separate would be more complicated to write. It'll probably easier to have single streaming rpc."
- "Use similar naming as metainfo protocol."
- In reference to `exit_orders` - "Why do we need this table?"

[A description of the steps in the implementation.]

## Open issues (if applicable)

[A discussion of issues relating to this proposal for which the author does not
know the solution. This section may be omitted if there are none.]
