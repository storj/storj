# Storage Node Graceful Exit - Gathering Pieces

## Abstract

During [Graceful Exit](storagenode-graceful-exit-overview.md) satellite needs to find pieces to be transferred from the exiting Storage Node.

## Background

Graceful Exit contains a process that moves existing pieces from one Storage Node to another. To accomplish this we need a list of pieces that need to be transferred.

Pieces with lower durability have higher importance to be transferred.

## Design

To gather the pieces for transferring we need a service on the satellite that finds the relevant information from the metainfo database. We'll call this service `gexit.Service` or Graceful Exit service.

The service starts by asking overlay for all exiting nodes.

Then joins a metainfo loop to iterate over all segments. For any segment that contains nodes that are exiting it will add an entry to a queue. We call this transfer queue.

The transfer queue is stored in database. We will need batching when inserting to database to avoid excessive load.

Once metainfo loop has completed successfully it updates node to be ready for transferring.

## Rationale

We could store the queue in-memory, however there is a danger that it might get too big. We can simplify the queue, by not having batching, however this would significantly increase the database load.

We could keep keep a live summary of the pieces in the queue, however, we can always query the database, which is easier to implement and change.

The metainfo loop `Join` guarantees the observer will only receive events at the beginning of the next loop. Hence, one complete metainfo loop is sufficient to collect all the pieces for a given node. 

## Implementation

1. Add method for finding exiting nodes to overlay.
2. Implement transfer queue for pieces.
3. Implement gexit.Service.

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
    field failed_at           timestamp ( updateable ) // when/if it failed
    field failed_status_code  int
    field completed_at        timestamp ( updateable )
)
```

## Open issues (if applicable)

- Can pieces with really high durability can be ignored?