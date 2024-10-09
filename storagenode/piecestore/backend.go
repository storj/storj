// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"io"
	"io/fs"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/storj/storagenode/monitor"
	"storj.io/storj/storagenode/pieces"
)

type PieceBackend interface {
	Writer(context.Context, storj.NodeID, storj.PieceID, pb.PieceHashAlgorithm, time.Time) (PieceWriter, error)
	Reader(context.Context, storj.NodeID, storj.PieceID) (PieceReader, error)
	StartRestore(context.Context, storj.NodeID) error
}

type PieceWriter interface {
	io.Writer
	Size() int64
	Hash() []byte
	Cancel(context.Context) error
	Commit(context.Context, *pb.PieceHeader) error
}

type PieceReader interface {
	io.ReadCloser
	Trash() bool
	Size() int64
	Seek(int64, int) (int64, error)
	GetHashAndLimit(context.Context) (pb.PieceHash, pb.OrderLimit, error)
}

//
// the old stuff
//

type OldPieceBackend struct {
	store      *pieces.Store
	trashChore *pieces.TrashChore
	monitor    *monitor.Service
}

func NewOldPieceBackend(store *pieces.Store, trashChore *pieces.TrashChore, monitor *monitor.Service) *OldPieceBackend {
	return &OldPieceBackend{
		store:      store,
		trashChore: trashChore,
		monitor:    monitor,
	}
}

func (opb *OldPieceBackend) Writer(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID, hashAlgorithm pb.PieceHashAlgorithm, expiration time.Time) (PieceWriter, error) {
	writer, err := opb.store.Writer(ctx, satellite, pieceID, hashAlgorithm)
	if err != nil {
		return nil, err
	}
	return &OldPieceWriter{
		Writer:      writer,
		store:       opb.store,
		satelliteID: satellite,
		pieceID:     pieceID,
		expiration:  expiration,
	}, nil
}

func (opb *OldPieceBackend) Reader(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) (PieceReader, error) {
	reader, err := opb.store.Reader(ctx, satellite, pieceID)
	if err == nil {
		return &OldPieceReader{
			Reader:    reader,
			store:     opb.store,
			satellite: satellite,
			pieceID:   pieceID,
			trash:     false,
		}, nil
	}
	if !errs.Is(err, fs.ErrNotExist) {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	// check if the file is in trash, if so, restore it and
	// continue serving the download request.
	tryRestoreErr := opb.store.TryRestoreTrashPiece(ctx, satellite, pieceID)
	if tryRestoreErr != nil {
		opb.monitor.VerifyDirReadableLoop.TriggerWait()

		// we want to return the original "file does not exist" error to the rpc client
		return nil, rpcstatus.Wrap(rpcstatus.NotFound, err)
	}

	// try to open the file again
	reader, err = opb.store.Reader(ctx, satellite, pieceID)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}
	return &OldPieceReader{
		Reader:    reader,
		store:     opb.store,
		satellite: satellite,
		pieceID:   pieceID,
		trash:     true,
	}, nil
}

func (opb *OldPieceBackend) StartRestore(ctx context.Context, satellite storj.NodeID) error {
	return opb.trashChore.StartRestore(ctx, satellite)
}

type OldPieceWriter struct {
	*pieces.Writer
	store       *pieces.Store
	satelliteID storj.NodeID
	pieceID     storj.PieceID
	expiration  time.Time
}

func (o *OldPieceWriter) Commit(ctx context.Context, header *pb.PieceHeader) error {
	if err := o.Writer.Commit(ctx, header); err != nil {
		return err
	}
	if !o.expiration.IsZero() {
		return o.store.SetExpiration(ctx, o.satelliteID, o.pieceID, o.expiration, o.Writer.Size())
	}
	return nil
}

type OldPieceReader struct {
	*pieces.Reader
	store     *pieces.Store
	satellite storj.NodeID
	pieceID   storj.PieceID
	trash     bool
}

func (o *OldPieceReader) Trash() bool { return o.trash }

func (o *OldPieceReader) GetHashAndLimit(ctx context.Context) (pb.PieceHash, pb.OrderLimit, error) {
	return o.store.GetHashAndLimit(ctx, o.satellite, o.pieceID, o.Reader)
}
