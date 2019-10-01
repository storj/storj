# Storage Node Graceful Exit - Gathering Pieces

[Graceful Exit Overview](overview.md)

## Abstract

During Graceful Exit, a satellite needs to find pieces to be transferred from the exiting storage node.

## Background

Graceful Exit contains a process that moves existing pieces from one storage node to another. To accomplish this we need a list of pieces that need to be transferred.

Pieces with lower durability have higher importance to be transferred.

## Design

We need a service on the satellite that finds pieces in the metainfo database that need to be transferred. We'll call this service `gracefulexit.Service` or Graceful Exit service.

The service starts by asking overlay for all exiting nodes where `nodes.exit_loop_completed_at` is null.

Then joins a metainfo loop to iterate over all segments. For any segment that contains nodes that are exiting it will add an entry to a queue (if durability <= optimal). We call this the transfer queue. If durability > optimal, we remove the exiting node from the segment / pointer.


The transfer queue is stored in database. We will need batching when inserting to database to avoid excessive load.

Once metainfo loop has completed successfully it updates `nodes.exit_loop_completed_at` with the current timestamp to indicate the storage nodes is ready for transferring.

In the event that the satellite does not complete the metainfo loop (e.g. satellite is shutdown), the service will re-enter the metainfo loop for all exiting nodes where `nodes.exit_loop_completed_at` is null. Pieces that already exist in the queue should not get duplicated.

## Rationale

We could store the queue in-memory, however there is a danger that it might consume too much memory. We can simplify the queue, by not having batching, however this would significantly increase the database load.

We could keep keep a live summary of the pieces in the queue, however, we can always query the database, which is easier to implement and change.

The metainfo loop `Join` guarantees the observer will only receive events at the beginning of the next loop. Hence, one complete metainfo loop is sufficient to collect all the pieces for a given node. 

## Implementation

1. Add method for finding exiting nodes to overlay.
2. Implement transfer queue for pieces.
3. Implement gracefulexit.Service.
4. Update satellite to ignore exiting storage nodes for repairs and uploads.

## Open issues (if applicable)
