# Storagenode Graceful Exit Code Refactor Blueprint

## Abstract

This article is all about our vision how to make graceful exit code on the storagenode side a bit readable and testable.

## Background

Currently, all main graceful exit logic is placed in Worker (storj/storagenode/gracefulexit/worker.go)

This module has giant list of dependencies, goroutines, large methods, lack of comments and tests.

Moreover, in this place we combine all this logic - database calls, business data processing, different transport level dialings, infinite loops, goroutines.

Size of this code is too large to understand it easily. Sometimes we faced such things like missing returns and wrong error handling.

All this factors won't allow us to be sure that it works fine even if there won't be any bug tickets for some time.

We all understand that small pieces of code are much more readable and its easier to test it and to find potential bug just by reading it.

## Design

So, our vision is next:

1. **Internode Transfer Controller** - the TransferController implements TransferPiece (do all order and piece validations, then transfer a piece from local node to a remote node). To be shared by Planned Downtime code.
1. **Graceful Exit Controller** - the GE controller acts as the gateway to the ``satellites`` db (querying it and updating it as necessary). The ``satellites`` db tracks the storagenode's relationship with various satellites. (Not to be confused with the ``satelliteDB``, which lives on the satellite.)
1. **Worker** - Responsible for communication with the satellite: receiving instructions about pieces to transfer or delete, carrying out those actions (while managing the number of goroutines used) and sending responses. Worker uses the Graceful Exit Controller in order to update the database and the Internode Transfer Controller for actually sending pieces. This worker should have dependencies interfaces, to be able to mock them in future.

Both of these controller types should be used by way of interface types, to be able to mock them as necessary.

```go
type /* internode. */ TransferController interface {
	// TransferPiece validates a transfer order, validates the locally stored
	// piece, and then (if appropriate) transfers the piece to the specified
	// destination node, obtaining a signed receipt. TransferPiece returns a
	// message appropriate for responding to the transfer order (whether the
	// transfer succeeded or failed).
	TransferPiece(ctx context.Context, satelliteID storj.NodeID, transferPiece *pb.TransferPiece) *pb.StorageNodeMessage
}
```

```go
type /* gracefulexit. */ Controller interface {
	// ListGracefulExits returns a slice with one record for every satellite
	// from which this node is gracefully exiting. Each record includes the
	// satellite's ID/address and information about the graceful exit status
	// and progress.
	ListGracefulExits(ctx context.Context) ([]ExitingSatellite, error)

	// DeleteOnePiece deletes one piece stored for a satellite.
	DeleteOnePiece(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID) error

	// DeleteAllPieces deletes all pieces stored for a satellite.
	DeleteAllPieces(ctx context.Context, satelliteID storj.NodeID) error

	// Fail updates the database when a graceful exit has failed.
	Fail(ctx context.Context, satelliteID storj.NodeID, reason pb.ExitFailed_Reason, exitFailedBytes []byte) error

	// Complete updates the database when a graceful exit is completed. It also
	// deletes all pieces and blobs for that satellite.
	Complete(ctx context.Context, satelliteID storj.NodeID, completionReceipt []byte, wait func()) error

	// Cancel deletes the entry from satellite table and inform graceful exit
	// has failed to start.
	Cancel(ctx context.Context, satelliteID storj.NodeID) error
}
```

The implementation of internode.TransferController would get the ecclient.Client and its related parameters:

```go
var _ TransferController = (*transferController)(nil)

type transferController struct {
	log      *zap.Logger
	store    *pieces.Store
	trust    *trust.Pool
	ecClient ecclient.Client

	minDownloadTimeout time.Duration
	minBytesPerSecond  memory.Size
}
```

And the implementation of gracefulexit.Controller would take all of the db related dependencies away from Worker.

```go
var _ Controller = (*controller)(nil)

type controller struct {
	log         *zap.Logger
	store       *pieces.Store
	trust       *trust.Pool
	satelliteDB satellites.DB

	nowFunc func() time.Time  // for easier testing
}
```

After that, the Worker is left with fewer dependencies:

```go
type Worker struct {
	log          *zap.Logger
	geController Controller
	txController internode.TransferController
	dialer       rpc.Dialer
	limiter      *sync2.Limiter
	satelliteURL storj.NodeURL
}
```

An example of this refactoring you can find in this pull request:
https://review.dev.storj.io/c/storj/storj/+/2499

## Rationale

## Implementation

## Wrapup

## Open issues
