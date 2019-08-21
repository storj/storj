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

Storage Node has a Graceful Exit service, which ensures that the process is completed.

Query `graceful_exit_status` to find unfinished exits

Start a worker for a particular satellite when it doesn't already exist.

The worker polls the satellite whether it can transfer pieces.

During first call to Process initiate graceful exit on the satellite.

When satellite hasn't completed gathering pieces, return NotReady.

Otherwise concurrently start transferring pieces.

Finally satellite sends the completion message

### Transferring a Piece

### Verifying Transfer


## Rationale

We could have a separate initiate graceful exit RPC, however this would complicate things when satellite is unresponsive. TODO

## Implementation

TODO: Discuss review comments.
- "Having initiate exit, get put orders and process put orders separate would be more complicated to write. It'll probably easier to have single streaming rpc."
- "Use similar naming as metainfo protocol."
- In reference to `exit_orders` - "Why do we need this table?"

[A description of the steps in the implementation.]

### Sketch

```
storagenode

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

satellite

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
                ...
            }
            message TransferFailed {
                ...
            }
        }
	}

	message SatelliteMessage {
        oneof Message {
            message NotReady {} // this could be a grpc error rather than a message

            message TransferPiece {
            }

            message DeletePiece {
            }

            message Completed {
                // when everything is completed
            }
        }
	}
```


## Open issues (if applicable)

[A discussion of issues relating to this proposal for which the author does not
know the solution. This section may be omitted if there are none.]
