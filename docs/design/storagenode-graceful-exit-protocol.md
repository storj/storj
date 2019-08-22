# Storage Node Graceful Exit - Transferring Pieces

## Abstract

This document describes how Storage Node transfers its pieces during Graceful Exit.

## Background

During Graceful Exit storage node needs to transfer pieces to other nodes. During transfering the storage node or satellite may crash, hence it needs to be able to continue after a restart. 

Satellite gathers transferred pieces list asynchronously, which is described in [Gathering Pieces Document](storagenode-graceful-exit-pieces.md). This may a significant amount of time.

Transferring a piece to another node may fail, hence we need to ensure that critical pieces get transferred. Storage Nodes can be malicious and try to misreport transfer as "failed" or "completed". Storage Node may also try to send wrong data. Which means we need proof that the correct piece was transferred.

After all pieces have been transferred the Storage Node needs a receipt for completing the transfer.

Both storage node and satellite operators need insight into graceful exit progress.

## Design

Storage Node has a Graceful Exit service, which ensures that the process is completed. It queries `graceful_exit_status` to find unfinished exits and starts a per satellite `worker` if needed.

The `worker` polls the satellite, requesting pieces to transfer. The Satellite will initiate a Graceful Exit if not already initiated. When first initiated, the Satellite  will start gathering pieces for the exiting node and return `NotReady`. The Satellite will continue to return `NotReady` until the piece gathering process has completed.

The `worker` should continue to poll the Satellite at a configurable interval until it returns pieces to transfer.  

The Storage Node should concurrently transfer pieces returned by the Satellite.  The Storage Node should send a `TransferSucceeded` message as pieces are successfuly transfered. The Storage node should send a `TransferFailed`, with reason, on failure.

The Satellites should set the `finished_at` on success, and respond with a `DeletePiece` message. Otherwise set the `failed_at` and `failure_status_code` for reprocessing.

The Satellite should respond with a `Completed` message when all pieces have finished processing. 
The Storage Node should store the completion receipt, stop transfer processing, and remove all remaining pieces for the Satellite.


TODO: What are the failure types
1. File not found on node - Cannot be reprocessed
2. New node not available - Need a new addressed order limit
3. Unknown - Retry?
4. ???

### Transferring a Piece

### Verifying Transfer


## Rationale

We could have a separate initiate graceful exit RPC, however this would complicate things when satellite is unresponsive. TODO

## Implementation

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
		        AddressedOrderLimit addressed_order_limit;
		        bytes piece_hash;
            }
            message TransferFailed {
                enum Error {
                    NOT_FOUND = 0;
                    STORAGE_NODE_UNAVAILABLE = 1;
                    UNKNOWN = 2;
                }
                Error error = 0;
            }
        }
	}

	message SatelliteMessage {
        oneof Message {
            message NotReady {} // this could be a grpc error rather than a message

            message TransferPiece {
                bytes private_key;
                AddressedOrderLimit addressed_order_limit;              
            }

            message DeletePiece {
                bytes piece_id;
            }

            message Completed {
                // when everything is completed
                bytes exit_complete_signature;
            }
        }
	}
```


## Open issues (if applicable)

[A discussion of issues relating to this proposal for which the author does not
know the solution. This section may be omitted if there are none.]
