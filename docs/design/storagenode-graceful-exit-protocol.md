# Storage Node Graceful Exit - Transferring Pieces

## Abstract

This document describes how Storage Node transfers its pieces during Graceful Exit.

## Background

During Graceful Exit storage node needs to transfer pieces to other nodes. During transfering the storage node or satellite may crash, hence it needs to be able to continue after a restart. 

Satellite gathers transferred pieces list asynchronously, which is described in [Gathering Pieces Document](#TODO). This may significant amount of time.

Transferring a piece to another node may fail, hence we need to ensure that critical pieces get transferred. Storage Nodes can be malicious and try to misreport transfer as "failed" or "completed". Storage Node may also try to send wrong data. Which means we need proof that the correct piece was transferred.

After all pieces have been transferred the Storage Node needs a receipt for completing the transfer.

Both storage node and satellite operators need insight into graceful exit progress.

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
