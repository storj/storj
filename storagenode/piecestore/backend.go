// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"encoding/binary"
	"errors"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"

	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/storj/storagenode/hashstore"
	"storj.io/storj/storagenode/monitor"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/retain"
)

// PieceBackend is the minimal interface needed for the endpoints to do its job.
type PieceBackend interface {
	Writer(context.Context, storj.NodeID, storj.PieceID, pb.PieceHashAlgorithm, time.Time) (PieceWriter, error)
	Reader(context.Context, storj.NodeID, storj.PieceID) (PieceReader, error)
	StartRestore(context.Context, storj.NodeID) error
}

// PieceWriter is an interface for writing a piece.
type PieceWriter interface {
	io.Writer
	Size() int64
	Hash() []byte
	Cancel(context.Context) error
	Commit(context.Context, *pb.PieceHeader) error
}

// PieceReader is an interface for reading a piece.
type PieceReader interface {
	io.ReadSeekCloser
	Trash() bool
	Size() int64
	GetPieceHeader() (*pb.PieceHeader, error)
}

//
// hash store backend
//

// HashStoreBackend implements PieceBackend using the hashstore.
type HashStoreBackend struct {
	logsPath  string
	tablePath string
	cfg       hashstore.Config

	bfm *retain.BloomFilterManager
	rtm *retain.RestoreTimeManager
	log *zap.Logger

	mu  sync.Mutex
	dbs map[storj.NodeID]*hashstore.DB
}

// NewHashStoreBackend constructs a new HashStoreBackend with the provided values. The log and hash
// directory are allowed to be the same.
func NewHashStoreBackend(
	ctx context.Context,
	cfg hashstore.Config,
	logsPath string,
	tablePath string,
	bfm *retain.BloomFilterManager,
	rtm *retain.RestoreTimeManager,
	log *zap.Logger,
) (*HashStoreBackend, error) {

	if tablePath == "" {
		tablePath = logsPath
	}

	hsb := &HashStoreBackend{
		logsPath:  logsPath,
		tablePath: tablePath,
		cfg:       cfg,
		bfm:       bfm,
		rtm:       rtm,
		log:       log,

		dbs: map[storj.NodeID]*hashstore.DB{},
	}

	// open any existing databases
	entries, err := os.ReadDir(logsPath)
	if errors.Is(err, fs.ErrNotExist) {
		return hsb, nil
	} else if err != nil {
		return nil, errs.Wrap(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		satellite, err := storj.NodeIDFromString(entry.Name())
		if err != nil {
			continue // ignore directories that aren't node IDs
		}
		if _, err := hsb.getDB(ctx, satellite); err != nil {
			return nil, errs.Wrap(err)
		}
	}

	return hsb, nil
}

// TestingCompact calls Compact on all of the hashstore databases.
func (hsb *HashStoreBackend) TestingCompact(ctx context.Context) error {
	hsb.mu.Lock()
	defer hsb.mu.Unlock()

	for _, db := range hsb.dbs {
		if err := db.Compact(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Close closes the HashStoreBackend.
func (hsb *HashStoreBackend) Close() error {
	hsb.mu.Lock()
	defer hsb.mu.Unlock()

	var eg errs.Group
	for _, db := range hsb.dbs {
		eg.Add(db.Close())
	}
	return eg.Err()
}

func (hsb *HashStoreBackend) dbsCopy() map[storj.NodeID]*hashstore.DB {
	hsb.mu.Lock()
	defer hsb.mu.Unlock()

	return maps.Clone(hsb.dbs)
}

// Stats implements monkit.StatSource.
func (hsb *HashStoreBackend) Stats(cb func(key monkit.SeriesKey, field string, val float64)) {
	type IDDB struct {
		id storj.NodeID
		db *hashstore.DB
	}

	dbs := hsb.dbsCopy()
	iddbs := make([]IDDB, 0, len(hsb.dbs))
	for id, db := range dbs {
		iddbs = append(iddbs, IDDB{id, db})
	}

	sort.Slice(iddbs, func(i, j int) bool {
		return iddbs[i].id.String() < iddbs[j].id.String()
	})

	for _, iddb := range iddbs {
		dbStat, s0Stat, s1Stat := iddb.db.Stats()
		taggedSeries := monkit.NewSeriesKey("hashstore").WithTag("satellite", iddb.id.String())
		monkit.StatSourceFromStruct(taggedSeries, dbStat).Stats(cb)
		monkit.StatSourceFromStruct(taggedSeries.WithTag("db", "s0"), s0Stat).Stats(cb)
		monkit.StatSourceFromStruct(taggedSeries.WithTag("db", "s1"), s1Stat).Stats(cb)
	}
}

// SpaceUsage gets a monitor.SpaceUsage from the HashStoreBackend.
func (hsb *HashStoreBackend) SpaceUsage() (subs monitor.SpaceUsage) {
	for _, db := range hsb.dbsCopy() {
		stats, _, _ := db.Stats()
		subs.UsedTotal += int64(stats.LenLogs + stats.TableSize)
		subs.UsedForPieces += int64(stats.LenSet - stats.LenTrash)
		subs.UsedForTrash += int64(stats.LenTrash)
		subs.UsedForMetadata += int64(stats.TableSize)
		subs.UsedReclaimable += int64(stats.LenLogs - stats.LenSet)
	}
	return subs
}

// ForgetSatellite closes the database for the satellite and removes the directory.
func (hsb *HashStoreBackend) ForgetSatellite(ctx context.Context, satellite storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	hsb.mu.Lock()
	defer hsb.mu.Unlock()

	db, exists := hsb.dbs[satellite]
	if !exists {
		return nil
	}
	delete(hsb.dbs, satellite)

	_ = db.Close()

	err = os.RemoveAll(filepath.Join(hsb.logsPath, satellite.String()))
	if err != nil {
		return errs.Wrap(err)
	}

	err = os.RemoveAll(filepath.Join(hsb.tablePath, satellite.String()))
	if err != nil {
		return errs.Wrap(err)
	}
	return nil
}

func (hsb *HashStoreBackend) getDB(ctx context.Context, satellite storj.NodeID) (*hashstore.DB, error) {
	hsb.mu.Lock()
	defer hsb.mu.Unlock()

	if db, exists := hsb.dbs[satellite]; exists {
		return db, nil
	}

	start := time.Now()

	var log *zap.Logger
	if hsb.log != nil {
		log = hsb.log.With(zap.String("satellite", satellite.String()))
	} else {
		log = zap.NewNop()
	}

	var (
		shouldTrash func(ctx context.Context, pieceID storj.PieceID, created time.Time) bool
		lastRestore func(ctx context.Context) time.Time
	)
	if hsb.bfm != nil {
		shouldTrash = hsb.bfm.GetBloomFilter(satellite)
	}
	if hsb.rtm != nil {
		lastRestore = func(ctx context.Context) time.Time {
			return hsb.rtm.GetRestoreTime(ctx, satellite, time.Now())
		}
	}

	db, err := hashstore.New(
		ctx,
		hsb.cfg,
		filepath.Join(hsb.logsPath, satellite.String()),
		filepath.Join(hsb.tablePath, satellite.String()),
		log,
		shouldTrash,
		lastRestore,
	)
	if err != nil {
		return nil, err
	}

	hsb.dbs[satellite] = db

	log.Info("hashstore opened successfully", zap.Duration("open_time", time.Since(start)))
	return db, nil
}

// Writer implements PieceBackend.
func (hsb *HashStoreBackend) Writer(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID, hashAlgo pb.PieceHashAlgorithm, expires time.Time) (_ PieceWriter, err error) {
	defer mon.Task()(&ctx)(&err)

	db, err := hsb.getDB(ctx, satellite)
	if err != nil {
		return nil, err
	}
	writer, err := db.Create(ctx, pieceID, expires)
	if err != nil {
		return nil, err
	}
	var hasher hash.Hash
	if hashAlgo == -1 {
		hasher = nohash{}
	} else {
		hasher = pb.NewHashFromAlgorithm(hashAlgo)
	}
	return &hashStoreWriter{
		writer: writer,
		hasher: hasher,
	}, nil
}

// Reader implements PieceBackend.
func (hsb *HashStoreBackend) Reader(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) (_ PieceReader, err error) {
	defer mon.Task()(&ctx)(&err)

	db, err := hsb.getDB(ctx, satellite)
	if err != nil {
		return nil, err
	}
	ttfb := newTimer(mon.DurationVal("download_time_to_first_byte_read"))
	reader, err := db.Read(ctx, pieceID)
	if err != nil {
		return nil, err
	}
	return &hashStoreReader{
		sr:     io.NewSectionReader(reader, 0, reader.Size()-512),
		reader: reader,
		ttfb:   ttfb,
	}, nil
}

// StartRestore implements PieceBackend.
func (hsb *HashStoreBackend) StartRestore(ctx context.Context, satellite storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	return hsb.rtm.SetRestoreTime(ctx, satellite, time.Now())
}

type hashStoreWriter struct {
	writer *hashstore.Writer
	size   int64

	hasher hash.Hash
}

func (hw *hashStoreWriter) Write(p []byte) (int, error) {
	n, err := hw.writer.Write(p)
	hw.size += int64(n)
	hw.hasher.Write(p[:n])
	return n, err
}

func (hw *hashStoreWriter) Size() int64                      { return hw.size }
func (hw *hashStoreWriter) Hash() []byte                     { return hw.hasher.Sum(nil) }
func (hw *hashStoreWriter) Cancel(ctx context.Context) error { hw.writer.Cancel(); return nil }

func (hw *hashStoreWriter) Commit(ctx context.Context, header *pb.PieceHeader) (err error) {
	defer mon.Task()(&ctx)(&err)

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

type hashStoreReader struct {
	sr     *io.SectionReader
	reader *hashstore.Reader
	ttfb   *timer
}

func (hr *hashStoreReader) Read(p []byte) (int, error) {
	defer hr.ttfb.Trigger()
	return hr.sr.Read(p)
}

func (hr *hashStoreReader) Seek(offset int64, whence int) (int64, error) {
	return hr.sr.Seek(offset, whence)
}

func (hr *hashStoreReader) Close() error { return hr.reader.Close() }
func (hr *hashStoreReader) Trash() bool  { return hr.reader.Trash() }
func (hr *hashStoreReader) Size() int64  { return hr.reader.Size() - 512 }

func (hr *hashStoreReader) GetPieceHeader() (_ *pb.PieceHeader, err error) {
	data, err := io.ReadAll(io.NewSectionReader(hr.reader, hr.reader.Size()-512, 512))
	hr.ttfb.Trigger()
	if err != nil {
		return nil, err
	}
	if len(data) != 512 {
		return nil, errs.New("footer too small")
	}
	l := binary.BigEndian.Uint16(data[0:2])
	if int(l) > len(data) {
		return nil, errs.New("footer length field too large: %d > %d", l, len(data))
	}
	var header pb.PieceHeader
	if err := pb.Unmarshal(data[2:2+l], &header); err != nil {
		return nil, err
	}
	return &header, nil
}

//
// the old stuff
//

// OldPieceBackend takes a bunch of pieces the endpoint used and packages them into a PieceBackend.
type OldPieceBackend struct {
	store      *pieces.Store
	trashChore RestoreTrash
	monitor    *monitor.Service
}

// NewOldPieceBackend constructs an OldPieceBackend.
func NewOldPieceBackend(store *pieces.Store, trashChore RestoreTrash, monitor *monitor.Service) *OldPieceBackend {
	return &OldPieceBackend{
		store:      store,
		trashChore: trashChore,
		monitor:    monitor,
	}
}

// Writer implements PieceBackend and returns a PieceWriter for a piece.
func (opb *OldPieceBackend) Writer(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID, hashAlgorithm pb.PieceHashAlgorithm, expiration time.Time) (_ PieceWriter, err error) {
	defer mon.Task()(&ctx)(&err)

	writer, err := opb.store.Writer(ctx, satellite, pieceID, hashAlgorithm)
	if err != nil {
		return nil, err
	}
	return &oldPieceWriter{
		Writer:      writer,
		store:       opb.store,
		satelliteID: satellite,
		pieceID:     pieceID,
		expiration:  expiration,
	}, nil
}

// Reader implements PieceBackend and returns a PieceReader for a piece.
func (opb *OldPieceBackend) Reader(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) (_ PieceReader, err error) {
	defer mon.Task()(&ctx)(&err)

	reader, err := opb.store.Reader(ctx, satellite, pieceID)
	if err == nil {
		return &oldPieceReader{
			Reader:    reader,
			store:     opb.store,
			satellite: satellite,
			pieceID:   pieceID,
			trash:     false,
		}, nil
	}
	if !errs.Is(err, fs.ErrNotExist) {
		return nil, rpcstatus.NamedWrap("old-piece-backend-open-fail", rpcstatus.Internal, err)
	}

	// check if the file is in trash, if so, restore it and
	// continue serving the download request.
	tryRestoreErr := opb.store.TryRestoreTrashPiece(ctx, satellite, pieceID)
	if tryRestoreErr != nil {
		opb.monitor.VerifyDirReadableLoop.TriggerWait()

		// we want to return the original "file does not exist" error to the rpc client
		return nil, rpcstatus.NamedWrap("not-found", rpcstatus.NotFound, err)
	}

	// try to open the file again
	reader, err = opb.store.Reader(ctx, satellite, pieceID)
	if err != nil {
		return nil, rpcstatus.NamedWrap("old-piece-backend-open-fail-after-trash-restore", rpcstatus.Internal, err)
	}
	return &oldPieceReader{
		Reader:    reader,
		store:     opb.store,
		satellite: satellite,
		pieceID:   pieceID,
		trash:     true,
	}, nil
}

// StartRestore implements PieceBackend and starts a restore operation for a satellite.
func (opb *OldPieceBackend) StartRestore(ctx context.Context, satellite storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	return opb.trashChore.StartRestore(ctx, satellite)
}

type oldPieceWriter struct {
	*pieces.Writer
	store       *pieces.Store
	satelliteID storj.NodeID
	pieceID     storj.PieceID
	expiration  time.Time
}

func (o *oldPieceWriter) Commit(ctx context.Context, header *pb.PieceHeader) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := o.Writer.Commit(ctx, header); err != nil {
		return err
	}
	if !o.expiration.IsZero() {
		return o.store.SetExpiration(ctx, o.satelliteID, o.pieceID, o.expiration, o.Writer.Size())
	}
	return nil
}

type oldPieceReader struct {
	*pieces.Reader
	store     *pieces.Store
	satellite storj.NodeID
	pieceID   storj.PieceID
	trash     bool
}

func (o *oldPieceReader) Trash() bool { return o.trash }

type nohash struct {
}

func (n2 nohash) Write(p []byte) (n int, err error) {
	return 0, nil
}

func (n2 nohash) Sum(b []byte) []byte {
	return []byte{}
}

func (n2 nohash) Reset() {
}

func (n2 nohash) Size() int {
	return 0
}

func (n2 nohash) BlockSize() int {
	return 0
}
