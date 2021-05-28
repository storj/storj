# Storage Node Graceful Exit - Transferring Pieces

[Graceful Exit Overview](overview.md)

## Abstract

This document describes how storage node transfers its pieces during Graceful Exit.

## Background

During Graceful Exit a storage node needs to transfer pieces to other nodes. During transferring the storage node or satellite may crash, hence it needs to be able to continue after a restart.

Satellite gathers transferred pieces list asynchronously, which is described in [Gathering Pieces Document](pieces.md). This may consume a significant amount of time.

Transferring a piece to another node may fail, hence we need to ensure that critical pieces get transferred. Storage nodes can be malicious and try to misreport transfer as "failed" or "completed". A storage node may also try to send incorrect data. This means we need proof that the correct piece was transferred.

After all pieces have been transferred the storage node needs a receipt for completing the transfer.

Both storage node and satellite operators need insight into the Graceful Exit progress.

## Design

The storage node has a Graceful Exit service, which ensures that the process is completed. It queries `graceful_exit_status` to find unfinished exits and starts a per satellite `worker` if needed.

The `worker` polls the satellite, requesting pieces to transfer. The satellite will initiate a Graceful Exit if not already initiated. When first initiated, the satellite will start gathering pieces for the exiting node and return `NotReady`. The satellite will continue to return `NotReady` until the piece gathering process has completed.

The `worker` should continue to poll the satellite at a configurable interval until it returns pieces to transfer.

The satellite should return pieces to transfer from the transfer queue if piece durability <= optimal. If durability > optimal, we remove the exiting node from the segment / pointer.

The storage node should concurrently transfer pieces returned by the satellite. The storage node should send a `TransferSucceeded` message as pieces are successfully transferred. The Storage node should send a `TransferFailed`, with reason, on failure.

The satellites should set the `finished_at` on success, and respond with a `DeletePiece` message. Otherwise increment `failed_count` and set the `last_failed_at` and `last_failed_code` for reprocessing.

The satellite should respond with an `ExitCompleted` message when all pieces have finished processing.

If the storage node has failed too many transfers overall, failed the same piece over a certain threshold, or has sent incorrect data, the satellite will send an `ExitFailed` message. This indicates that the process has ended ungracefully.

In either case, the storage node should store the completion / failure receipt, stop transfer processing, and remove all remaining pieces for the satellite.

### Transferring a Piece

The satellite picks a piece from the transfer queue and verifies whether it still needs to be transferred. During gathering or the Graceful Exit process the segment may have been changed or deleted.

The satellite will send a `TransferPiece`, which contains sufficient information to upload the piece to a replacement node.

The storage node will start a new piece upload to the replacement node similar to uplink. It will use `uplink/piecestore.Upload`. Once uploaded, the storage node will verify the piece hash sent by the replacement node corresponds to the piece hash stored in the database and verify the signature.

#### Verifying transfer

The storage node sends the piece hash, order limit, original "uploader" signature, and piece hash signed by the replacement node to the satellite for confirmation.

The satellite verifies that the original piece hash matches the replacement piece hash, and verifies both order limit's signature. On success, the satellite will update segment / pointer information. If verification fails, the satellite will send an `ExitFailed` message.

![Transfer Sequence](./images/transfer-sequence.svg)

#### Updating the Segment / Pointer

When the upload has been verified the satellite will use `CompareAndSwap` to only switch the appropriate node without changing others. During the upload there may have been other things that happened:

- Segment / pointer was reuploaded. We'll ignore piece transferred for the exit.
- Segment / pointer pieces were changed, in that case we load the pointer and retry `CompareAndSwap`. When the storage node at that piece number has changed, we'll ignore the piece transferred for the exit, because one of them needs to be discarded.
- Segment / pointer was deleted or is being deleted. We'll ignore the piece transferred for the exit.

After processing the segment / pointer, we'll delete the piece from transfer queue, update progress, and send a `DeletePiece` to the exiting node.

#### Handling transfer failure

If a storage node report a failure to the satellite if it fails to upload to the replacement node, or the replacement node returns a bad piece hash. The satellite will track the failures and retry the transfer to another node.

## Rationale

We could have a separate initiate graceful exit RPC, however this would complicate things when satellite is unresponsive.

## Implementation

1. Add protobuf definitions.
2. Update node selection to ignore exiting nodes for repairs and uploads.
3. Update repairer to repair segments for nodes that failed an exit.
4. Implement verifying a transfer on the satellite.
5. Implement transferring a single piece on storage node.
6. Implement swapping node in segment.
7. Implement non-async protocol for exiting.
8. Implement async protocol for exiting.

### Sketch

```
### storagenode ###

for {
    stream := dial satellite

    for {
        msg := stream.Recv(&msg)
        if msg is NotReady {
            go sleep a bit
            and retry later
        }

        if msg is Completed {
            update database about completion
            delete all pieces
            exit
        }

        if msg is TransferConfirmed {
            update the progress table
            delete the piece from disk
            exit
        }

        if msg is TransferPiece {
            // transfer multiple pieces in parallel, configurable
            limiter.Go(func(){
                result := try transfer msg.PieceID
                stream.Send(result)
                update progress table
            })
        }
    }
}

### satellite ###

if !initiated {
    then add node to the gracefully exiting list
    send NotReady
    return
}

if !pieces collected {
    send NotReady
    return
}

inprogress pieces
more pieces := true

when storage node prematurely exits
    we'll mark every inprogress as failed

go func() {
    for {
        ensure we have only up to N inprogress at the same time

        list transferred piece that is not in progress
        if no pieces {
            morepieces = false
            break
        }

        stream.Send TransferPiece
    }

    more pieces = false
}()

for more pieces && len(inprogress) > 0 {
    response := stream.Recv

    verify that response has proper signatures and things
    update metainfo database with the new storage node
    delete from inprogress

    stream.Send TransferConfirmed
}

stream.Send Completed with receipt
```



``` proto
service GracefulExit {
    rpc Process(stream StorageNodeMessage) returns (stream SatelliteMessage)
}

message StorageNodeMessage {
    oneof Message {
        message TransferSucceeded {
            AddressedOrderLimit addressed_order_limit;
            PieceHash original_piece_hash;
            PieceHash replacement_piece_hash;
        }
        message TransferFailed {
            enum Error {
                NOT_FOUND = 0;
                STORAGE_NODE_UNAVAILABLE = 1;
                UNKNOWN = 2;
            }
            Error error;
        }
    }
}

message SatelliteMessage {
    oneof Message {
        message NotReady {} // this could be a rpc error rather than a message

        message TransferPiece {
            bytes piece_id; // the current piece-id
            bytes private_key;
            // addressed_order_limit contains the new piece id.
            AddressedOrderLimit addressed_order_limit;              
        }

        message DeletePiece {
            bytes piece_id;
        }

        message ExitCompleted {
            // when everything is completed
            bytes exit_complete_signature;
        }

        message ExitFailed {
            enum Reason {
                VERIFICATION_FAILED = 0;
                ...
            }
            // on failure
            bytes exit_failure_signature;
            Reason reason;
        }
    }
}
```


## Open issues (if applicable)

- How many is too many failed?
