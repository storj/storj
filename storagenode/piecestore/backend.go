// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"encoding/binary"
	"hash"
	"io"
	"io/fs"
	"path/filepath"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/storj/storagenode/hashstore"
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
	io.ReadSeekCloser
	Trash() bool
	Size() int64
	GetHashAndLimit(context.Context) (pb.PieceHash, pb.OrderLimit, error)
}

//
// hash store backend
//

type HashStoreBackend struct {
	dir string
	log *zap.Logger

	mu  sync.Mutex
	dbs map[storj.NodeID]*hashstore.DB
}

func NewHashStoreBackend(dir string, log *zap.Logger) *HashStoreBackend {
	return &HashStoreBackend{
		dir: dir,
		log: log,

		dbs: map[storj.NodeID]*hashstore.DB{},
	}
}

func (hsb *HashStoreBackend) getDB(satellite storj.NodeID) (*hashstore.DB, error) {
	hsb.mu.Lock()
	defer hsb.mu.Unlock()

	if db, exists := hsb.dbs[satellite]; exists {
		return db, nil
	}

	db, err := hashstore.New(
		filepath.Join(hsb.dir, satellite.String()),
		8,
		hsb.log.With(zap.String("satellite", satellite.String())),
		nil, // TODO: trash callback
		nil, // TODO: restore callback
	)
	if err != nil {
		return nil, err
	}

	hsb.dbs[satellite] = db

	return db, nil
}

func (hsb *HashStoreBackend) Writer(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID, hash pb.PieceHashAlgorithm, expires time.Time) (PieceWriter, error) {
	db, err := hsb.getDB(satellite)
	if err != nil {
		return nil, err
	}
	writer, err := db.Create(ctx, pieceID, expires)
	if err != nil {
		return nil, err
	}
	return &HashStoreWriter{
		writer: writer,
		hasher: pb.NewHashFromAlgorithm(hash),
	}, nil
}

func (hsb *HashStoreBackend) Reader(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) (PieceReader, error) {
	db, err := hsb.getDB(satellite)
	if err != nil {
		return nil, err
	}
	reader, err := db.Read(ctx, pieceID)
	if err != nil {
		return nil, err
	}
	return &HashStoreReader{
		sr:     io.NewSectionReader(reader, 0, reader.Size()-512),
		reader: reader,
	}, nil
}

func (hsb *HashStoreBackend) StartRestore(ctx context.Context, satellite storj.NodeID) error {
	// TODO: persist a timestamp or something somehow lol jt anxiety
	return errs.New("TODO")
}

type HashStoreWriter struct {
	writer *hashstore.Writer
	hasher hash.Hash
}

func (hw *HashStoreWriter) Write(p []byte) (int, error) {
	n, err := hw.writer.Write(p)
	hw.hasher.Write(p[:n])
	return n, err
}

func (hw *HashStoreWriter) Size() int64                      { return hw.writer.Size() }
func (hw *HashStoreWriter) Hash() []byte                     { return hw.hasher.Sum(nil) }
func (hw *HashStoreWriter) Cancel(ctx context.Context) error { hw.writer.Cancel(); return nil }

func (hw *HashStoreWriter) Commit(ctx context.Context, header *pb.PieceHeader) error {
	defer func() { _ = hw.Cancel(ctx) }()

	// marshal the header so we can put it as a footer.
	buf, err := pb.Marshal(header)
	if err != nil {
		return err
	} else if len(buf) > 512-2 {
		return errs.New("header too large")
	}

	// make a length prefixed footer and copy the header into it.
	var tmp [512]byte
	binary.BigEndian.PutUint16(tmp[0:2], uint16(len(buf)))
	copy(tmp[2:], buf)

	// write the footer.. header? footer.
	if _, err := hw.writer.Write(tmp[:]); err != nil {
		return err
	}

	// commit the piece.
	return hw.writer.Close()
}

type HashStoreReader struct {
	sr     *io.SectionReader
	reader *hashstore.Reader
}

func (hr *HashStoreReader) Read(p []byte) (int, error) { return hr.sr.Read(p) }
func (hr *HashStoreReader) Seek(offset int64, whence int) (int64, error) {
	return hr.sr.Seek(offset, whence)
}

func (hr *HashStoreReader) Close() error { return hr.reader.Close() }
func (hr *HashStoreReader) Trash() bool  { return hr.reader.Trash() }
func (hr *HashStoreReader) Size() int64  { return hr.reader.Size() - 512 }

func (hr *HashStoreReader) GetHashAndLimit(context.Context) (pb.PieceHash, pb.OrderLimit, error) {
	data, err := io.ReadAll(io.NewSectionReader(hr.reader, hr.reader.Size()-512, 512))
	if err != nil {
		return pb.PieceHash{}, pb.OrderLimit{}, err
	} else if len(data) != 512 {
		return pb.PieceHash{}, pb.OrderLimit{}, errs.New("footer too small")
	}
	l := binary.BigEndian.Uint16(data[0:2])
	if int(l) > len(data) {
		return pb.PieceHash{}, pb.OrderLimit{}, errs.New("footer length field too large: %d > %d", l, len(data))
	}
	var header pb.PieceHeader
	if err := pb.Unmarshal(data[:l], &header); err != nil {
		return pb.PieceHash{}, pb.OrderLimit{}, err
	}
	pieceHash := pb.PieceHash{
		PieceId:       hr.reader.Key(),
		Hash:          header.GetHash(),
		HashAlgorithm: header.GetHashAlgorithm(),
		PieceSize:     hr.Size(),
		Timestamp:     header.GetCreationTime(),
		Signature:     header.GetSignature(),
	}
	return pieceHash, header.OrderLimit, nil
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
