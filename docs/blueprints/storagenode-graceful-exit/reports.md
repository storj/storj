# Storage Node Graceful Exit - Reports

[Graceful Exit Overview](overview.md)

## Abstract

A satellite operator needs to know that status of exiting nodes in order to process held amount. 

## Background

As a result of a Graceful Exit, satellite operators need to release the held amount to the exited storage node. This means we need a report for exited nodes. 

This report should contain:
- NodeID
- Wallet address
- The date the node joined the network
- The date the node initiated the Graceful Exit
- The date the node completed the exit
- GB Transferred (amount of data the node transferred during exiting)

A satellite operator needs a list of nodes that have initiated an exit, but have not completed. This means we need a report for exiting nodes.

This report should contain:
- NodeID
- Wallet address
- The date the node joined the network
- The date the node initiated the Graceful Exit
- GB Transferred (amount of data the node transferred during exiting)

## Design

Add satellite CLI command to list gracefully exited nodes between two dates. The report should query `nodes` table where `exit_finished_at >= start AND exit_finished_at < end date`. 

Add satellite CLI command to list gracefully exiting nodes between two dates. The report should query `nodes` table where `exit_started_at >= start AND exit_started_at < end date AND exit_finished_at IS NULL`. 

GB transferred will be retrieved from a new `graceful_exit_progress` table.

## Implementation

- Add `graceful_exit_progress` table.
- Add "exit" fields to `nodes` table.
- Add `cmd/satellite/reports/graceful-exit.go` with methods `GracefullyExited` and `GracefullyExiting`, adding and adjusting database interfaces, if necessary. Accepts start and end date as parameters. Dates are inclusive, ignoring time.
- Add commands `gracefully-exited-report` and `gracefully-exiting-report` to satellite CLI.
    - See [Protocol for transferring pieces.](protocol.md) for details on `graceful_exit_progress`.

