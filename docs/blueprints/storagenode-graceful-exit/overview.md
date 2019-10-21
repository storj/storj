# Storage Node Graceful Exit Overview

## Abstract

When a storage node wants to leave the network, we would like to ensure that the pieces get transferred to other nodes. This avoids additional repair cost and bandwidth. This design document describes the necessary components for the process.

## Background

Storage nodes may want to leave the network. Taking a storage node directly offline would mean that the satellite needs to repair the lost pieces. To avoid this we need a process to transfer the pieces to other nodes before taking the exiting node offline. We call this process Graceful Exit. Since this process can take time we need an overview of the nodes exiting and the progress.

A storage node may also want to exit a single satellite rather than the whole network. For example, the storage node doesn't have as much bandwidth available or a satellite's payment structure has changed.

The process must also consider that the storage nodes may have limited bandwidth available or time to exit. This means the process should prioritize pieces with lower durability.

A storage node will no longer participate in repairs or uploads once a Graceful Exit has been initiated. In addition, bandwidth used for piece transfers should not count against bandwidth used.

The exiting node must continue to respond to audit, download, and retain (garbage collection) requests.

### Non-Goals

There are scenarios that this design document does not handle:

- Storage node runs out of bandwidth during exiting.
- Storage node may also want to partially exit due to decreased available capacity.
- Storage node wants to rejoin a satellite it previously exited.

## Design

The design is divided into four parts:

- [Process for gathering pieces that need to be transferred.](pieces.md)
- [Protocol for transferring pieces from one storage node to another.](protocol.md)
- [Reporting for graceful exit process.](reports.md)
- [User Interface for interacting with graceful exit.](ui.md)

Overall a good graceful exit process looks like:

1. Storage node Operator initiaties the graceful exit process, which:
    - notifies the satellite about the graceful exit, and
    - adds entry about exiting to storagenode database.
2. Satellite receives graceful exit request, which:
    - adds entry about exiting to satellite database, and
    - starts gathering of pieces that need to be transferred.
3. Satellite finishes gathering pieces that need to be transferred.
4. Storage node keeps polling satellite for pieces to transfer.
5. When the satellite doesn't have any more pieces to transfer, it will respond with a completion receipt.
6. Storage node stores completion information in the database.
7. Satellite Operator creates a report for exited storage nodes in order to release escrows.

For all of these steps we need to ensure that we have sufficient monitoring.

When a Graceful Exit has been started, it must either succeed or fail. The escrow, held from storage node, will be released only on success. We will call the failure scenario an ungraceful exit.

Ungraceful exit can happen when:

- Storage node doesn't transfer pieces,
- Storage node incorrectly transfers pieces,
- Storage node is too slow to transfer pieces,
- Storage node decided to terminate the process.

## Implementation

To coordinate the four parts we need few things implemented:

- Add `satellites` table and interfaces to storage node.
- Add `satellites_exit_progress` table and interfaces to storage node.
- Update `nodes` table on satellite.
- Add `nodes_exit_progress` table to satellite.

### Storage Node Database Changes

Create `satellites` table:

```
model satellites (
    key node_id

    field node_id  blob not null
    field address  text not null
    field added_at timestamp ( autoinsert ) not null

    field status   int not null
)
```

Create `satellites_exit_progress` tables:

```
model satellite_exit_progress (
    fk satellite_id 

    field initiated_at         timestamp ( updateable )
    field finished_at          timestamp ( updateable )
    field starting_disk_usage  int64 not null
    field bytes_deleted        int64 not null
    field completion_receipt   blob
)
```

### Satellite Database Changes

Update `nodes` table:

```
model nodes (
    ...
    field exit_loop_completed_at    timestamp ( updateable )
    field exit_initiated_at         timestamp ( updateable )
    field exit_finished_at          timestamp ( updateable )
}
```

Create `graceful_exit_progress` table:

```
model graceful_exit_progress {
    key node_id

    field node_id              blob
    field bytes_transferred    int64
    field pieces_transferred   int64
    field pieces_failed        int64
    field updated_at           timestamp ( updateable )
}
```

Create `graceful_exit_transfer_queue`:

```
model graceful_exit_transfer_queue (
    key node_id path

    field node_id             blob
    field path                blob
    field piece_num           int
    field durability_ratio    float64
    field queued_at           timestamp ( autoinsert ) // when the the piece info was queued
    field requested_at        timestamp ( updateable ) // when the piece info and orderlimits were requested by the storagenode
    field last_failed_at      timestamp ( updateable ) // when/if it failed
    field last_failed_code    int
    field failed_count        int
    field finished_at         timestamp ( updateable )
)
```

## Rationale

We could have all the information in a single table, but this would make the table more complicated to manage:

```
model satellites (
    key node_id

    field node_id  blob not null
    field address  text not null
    field added_at timestamp ( autoinsert ) not null
    field status   byte not null

    field exit_initiated_at         timestamp ( updateable )
    field exit_finished_at          timestamp ( updateable )
    field exit_starting_disk_usage  int64 not null
    field exit_bytes_deleted        int64
    field exit_completion_receipt   blob
)
```

Bytes transferred could be stored in `nodes`, but since `nodes` is a heavily accessed, this would add more load. Alternatively, we could use `graceful_exit_transfer_queue`, however this would require keeping a lot of additional data in the database.

## Open issues

Some exiting nodes may take too long to transfer pieces, potentially causing the SNO to quit the graceful exit.  We may want to provide a way to adjust segment durability requirements for satellite and/or a specific exiting node.
