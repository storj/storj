# Storage Node Graceful Exit - Reports

## Abstract

A satellite operator needs to know that status of exiting nodes in order to process escrows. 

## Background

As a result of a Graceful Exit, Satellite operators need to release the escrow to the exited Storage Node. This means we need a report about exited nodes. 

This report should contain:
- NodeID
- Wallet address
- The date the node joined the network
- The date the node initiated the Graceful Exit
- The date the node completed the exit
- GB Transferred (amount of data the node transferred during exiting)
  
A Satellite operator also needs a report for nodes that have initiated the exit, but not completed it with a given timeframe.

This report should contain:
- NodeID
- Wallet address
- The date the node joined the network
- The date the node initiated the Graceful Exit
- GB Transferred (amount of data the node transferred during exiting)

TODO: Discuss the original business requirements says "...run a report to get information exited and/or exiting Storage Nodes...".  Question is 1 report that contains both, or have 2 reports.

## Design

Provide satellite CLI command for Graceful Exit completed report which takes start and end date as parameters. The report should query `nodes` table where exit completed date is >= start and <= end date. 

Provide satellite CLI command for Graceful Exit pending report which takes start and end date as parameters. The report should query `nodes` table where exit initiated date is >= start and <= end date and completed date is null.

GB transferred will be retrieved from a new `exiting_nodes_bytes_transferred` table.

## Rationale

Bytes transferred could be stored in `nodes`, but since `nodes` is a heavily accessed, this would add more load. Alternatively, we could use pieceinfo queue used for transferring, however this would require keeping a lot of additional data in the database.

## Implementation
- Add `exiting_nodes_bytes_transferred` table. TODO: move to overview?
- Add "exit" fields to `nodes` TODO: move to overview?
- Add `gracefulexitreport completed` command to satellite CLI.
- Add `gexit.CompletedExitsReport` method. Accepts start and end date as parameters. Dates are inclusive, ignoring time.
- Add `gracefulexitreport pending` command to satellite CLI.
- Add `gexit.PendingExitsReport` method. Accepts start and end date as parameters. Dates are inclusive, ignoring time. 
- See [Protocol for transferring pieces.](storagenode-graceful-exit-protocol.md) for details on `exiting_nodes_bytes_transferred`.

Update `nodes`
```
model nodes (
    ...
    field exit_loop_completed       timestamp ( updateable )
    field exit_initiated_at         timestamp ( updateable )
    field exit_completed_at         timestamp ( updateable )

}
```
Create `exiting_nodes_bytes_transferred`
```
model exiting_nodes_bytes_transferred {
    key node_id

    field node_id              blob
    field bytes_transferred    int64
    field updated_at           timestamp ( updateable )
}
```
