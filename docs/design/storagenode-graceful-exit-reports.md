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

Add satellite CLI command to list gracefully exited nodes between two dates. The report should query `nodes` table where `exit_completed_at >= start AND exit_completed_at <= end date`. 

Add satellite CLI command to list gracefully exiting nodes between two dates. The report should query `nodes` table where `exit_started_at >= start AND exit_started_at <= end date AND exit_completed_at IS NULL`. 

GB transferred will be retrieved from a new `graceful_exit_progress` table.

## Rationale

Bytes transferred could be stored in `nodes`, but since `nodes` is a heavily accessed, this would add more load. Alternatively, we could use `graceful_exit_transfer_queue`, however this would require keeping a lot of additional data in the database.

## Implementation

- Add `graceful_exit_progress` table. TODO: move to overview?
- Add "exit" fields to `nodes` TODO: move to overview?
- Add `cmd/satellite/reports/graceful-exit.go` with methods `GracefullyExited` and `GracefullyExiting`, adding and adjusting database interfaces, if necessary. Accepts start and end date as parameters. Dates are inclusive, ignoring time.
- Add commands `gracefully-exited-report` and `gracefully-exiting-report` to satellite CLI.
    - See [Protocol for transferring pieces.](storagenode-graceful-exit-protocol.md) for details on `graceful_exit_progress`.

Update `nodes` tables:

```
model nodes (
    ...
    field exit_loop_completed       timestamp ( updateable )
    field exit_initiated_at         timestamp ( updateable )
    field exit_completed_at         timestamp ( updateable )
}
```

Create `graceful_exit_progress` table:

```
model graceful_exit_progress {
    key node_id

    field node_id              blob
    field bytes_transferred    int64
    field updated_at           timestamp ( updateable )
}
```
