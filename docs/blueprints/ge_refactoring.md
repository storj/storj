Hello!

This article is all about our vision how to make graceful exit code a bit readable and testable.

So, main logic is placed in Worker (storj/storagenode/gracefulexit/worker.go)

This module has giant list of dependencies, goroutines, large methods, lack of comments and tests.

Moreover, in this place we combine all this logic - database calls, business data processing, different transport level dialings, infinite loops, goroutines.

Size of this code is too large to understand it easily. Sometimes we faced such things like missing returns and wrong error handling.

All this factors won't allow us to be sure that it works fine even if there won't be any bug tickets for some time.

We all understand that small pieces of code are much more readable and its easier to test it and to find potential bug just by reading it.

So, our vision is next:

1. **Worker** - its main responsibility to run whole process. It should run goroutines, listen messages, make all needed dialing.
It should not depend on some concrete implementation, have references to database, etc.
This worker should have dependencies interfaces, to be able to mock them in future.
2. **Service** - service should gather data from database, process them or save them in database.

I would expect Service layer to be an interface also, to be able to mock it

```go
type GracefulExit interface {
	TransferPiece(ctx context.Context, satelliteID storj.NodeID, transferPiece *pb.TransferPiece, c gracefulExitStream) error
	DeleteOnePiece(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID) error
	DeletePiece(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID) error
	DeleteAllPieces(ctx context.Context, satelliteID storj.NodeID) error
	Fail(ctx context.Context, satelliteID storj.NodeID, reason pb.ExitFailed_Reason, exitFailedBytes []byte) error
	Complete(ctx context.Context, satelliteID storj.NodeID, exitFailedBytes []byte, wait func()) error
	CancelGracefulExit(ctx context.Context, satelliteID storj.NodeID) error
}
```
its implementation could have all db related depenncies 
```go
var _ GracefulExit = (*Service)(nil)
type Service struct {
	log *zap.Logger

	store       *pieces.Store
	trust       *trust.Pool
	ecclient    ecclient.Client
	satelliteDB satellites.DB

	minDownloadTimeout time.Duration
	minBytesPerSecond  memory.Size
}
```

The Worker after that worker will have less dependencies
```go
type Worker struct {
	log   *zap.Logger

	dialer       rpc.Dialer
	limiter      *sync2.Limiter
	satelliteURL storj.NodeURL

	service GracefulExit
}
```

the very initial refactoring you could find in this pull request
https://review.dev.storj.io/c/storj/storj/+/2499
