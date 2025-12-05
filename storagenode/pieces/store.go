// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"context"
	"database/sql"
	"io"
	"os"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/process"
	"storj.io/common/storj"
	"storj.io/storj/shared/bloomfilter"
	"storj.io/storj/storagenode/blobstore"
	"storj.io/storj/storagenode/blobstore/filestore"
	"storj.io/storj/storagenode/pieces/lazyfilewalker"
)

var (
	// Error is the default error class.
	Error = errs.Class("pieces error")

	mon = monkit.Package()
)

// Info contains all the information we need to know about a Piece to manage them.
type Info struct {
	SatelliteID storj.NodeID

	PieceID         storj.PieceID
	PieceSize       int64
	PieceCreation   time.Time
	PieceExpiration time.Time

	OrderLimit      *pb.OrderLimit
	UplinkPieceHash *pb.PieceHash
}

// ExpiredInfo is a fully namespaced piece id.
type ExpiredInfo struct {
	SatelliteID storj.NodeID
	PieceID     storj.PieceID

	// PieceSize is the size of the piece that was stored with the piece ID; if not zero, this
	// can be used to decrement the used space counters instead of calling os.Stat on the piece.
	PieceSize int64

	// This can be removed when we no longer need to support the pieceinfo db. Its only purpose
	// is to keep track of whether expired entries came from piece_expirations or pieceinfo.
	InPieceInfo bool
}

// ExpirationLimits contains limits used when getting and/or deleting expired pieces.
type ExpirationLimits struct {
	// FlatFileLimit is the maximum number of flat files to read in a single call.
	// This is only used for the flat file expiration store.
	FlatFileLimit int
	// BatchSize is the maximum number of pieces to return or delete in a single call.
	// This is ignored by the flat file store, as it does not make sense for the current implementation.
	BatchSize int
}

// ExpirationOptions contains options used when getting and/or deleting expired pieces.
type ExpirationOptions struct {
	Limits       ExpirationLimits
	ReverseOrder bool
}

// DefaultExpirationLimits returns the default values for ExpirationLimits.
func DefaultExpirationLimits() ExpirationLimits {
	return ExpirationLimits{
		FlatFileLimit: -1,
		BatchSize:     -1,
	}
}

// DefaultExpirationOptions returns the default values for ExpirationOptions.
func DefaultExpirationOptions() ExpirationOptions {
	return ExpirationOptions{
		Limits:       DefaultExpirationLimits(),
		ReverseOrder: false,
	}
}

// PieceExpirationDB stores information about pieces with expiration dates.
//
// architecture: Database
type PieceExpirationDB interface {
	// SetExpiration sets an expiration time for the given piece ID on the given satellite. If pieceSize
	// is non-zero, it may be used later to decrement the used space counters without needing to call
	// os.Stat on the piece.
	SetExpiration(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID, expiresAt time.Time, pieceSize int64) error
	// GetExpired gets piece IDs that expire or have expired before the given time
	GetExpired(ctx context.Context, expiresBefore time.Time, opts ExpirationOptions) ([]*ExpiredInfoRecords, error)
	// DeleteExpirations deletes approximately all the expirations that happen before the given time
	DeleteExpirations(ctx context.Context, expiresAt time.Time) error
	// DeleteExpirationsBatch deletes the pieces in the batch
	DeleteExpirationsBatch(ctx context.Context, now time.Time, opts ExpirationOptions) error
}

// V0PieceInfoDB stores meta information about pieces stored with storage format V0 (where
// metadata goes in the "pieceinfo" table in the storagenodedb). The actual pieces are stored
// behind something providing the blobstore.Blobs interface.
//
// architecture: Database
type V0PieceInfoDB interface {
	// Get returns Info about a piece.
	Get(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID) (*Info, error)
	// Delete deletes Info about a piece.
	Delete(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID) error
	// GetExpired gets piece IDs stored with storage format V0 that expire or have expired
	// before the given time
	GetExpired(ctx context.Context, expiredAt time.Time) ([]*ExpiredInfoRecords, error)
	// DeleteExpirations deletes approximately all the expirations that happen before the given time
	DeleteExpirations(ctx context.Context, expiresAt time.Time) error
	// WalkSatelliteV0Pieces executes walkFunc for each locally stored piece, stored
	// with storage format V0 in the namespace of the given satellite. If walkFunc returns a
	// non-nil error, WalkSatelliteV0Pieces will stop iterating and return the error
	// immediately. The ctx parameter is intended specifically to allow canceling iteration
	// early.
	WalkSatelliteV0Pieces(ctx context.Context, blobStore blobstore.Blobs, satellite storj.NodeID, walkFunc func(StoredPieceAccess) error) error
}

// V0PieceInfoDBForTest is like V0PieceInfoDB, but adds on the Add() method so
// that test environments with V0 piece data can be set up.
type V0PieceInfoDBForTest interface {
	V0PieceInfoDB

	// Add inserts Info to the database. This is only a valid thing to do, now,
	// during tests, to replicate the environment of a storage node not yet fully
	// migrated to V1 storage.
	Add(context.Context, *Info) error
}

// PieceSpaceUsedDB stores the most recent totals from the space used cache.
//
// architecture: Database
type PieceSpaceUsedDB interface {
	// Init creates the one total and trash record if it doesn't already exist
	Init(ctx context.Context) error
	// GetPieceTotals returns the space used (total and contentSize) by all pieces stored
	GetPieceTotals(ctx context.Context) (piecesTotal int64, piecesContentSize int64, err error)
	// GetPieceTotalsForAllSatellites returns how much total space used by pieces stored for each satelliteID
	GetPieceTotalsForAllSatellites(ctx context.Context) (map[storj.NodeID]SatelliteUsage, error)
	// UpdatePieceTotalsForAllSatellites updates each record for total spaced used with a new value for each satelliteID
	UpdatePieceTotalsForAllSatellites(ctx context.Context, newTotalsBySatellites map[storj.NodeID]SatelliteUsage) error
	// UpdatePieceTotalsForSatellite updates record with new values for a specific satelliteID.
	// If the usage values are set to zero, the record is deleted.
	UpdatePieceTotalsForSatellite(ctx context.Context, satelliteID storj.NodeID, usage SatelliteUsage) error
	// GetTrashTotal returns the total space used by trash
	GetTrashTotal(ctx context.Context) (int64, error)
	// UpdateTrashTotal updates the record for total spaced used for trash with a new value
	UpdateTrashTotal(ctx context.Context, newTotal int64) error
	// StoreUsageBeforeScan stores the total space used by pieces per satellite before the piece walker starts
}

// StoredPieceAccess allows inspection and manipulation of a piece during iteration with
// WalkSatellitePieces-type methods.
type StoredPieceAccess interface {
	blobstore.BlobInfo

	// PieceID gives the pieceID of the piece
	PieceID() storj.PieceID
	// Satellite gives the nodeID of the satellite which owns the piece
	Satellite() (storj.NodeID, error)
	// Size gives the size of the piece on disk, and the size of the piece
	// content (not including the piece header, if applicable)
	Size(ctx context.Context) (int64, int64, error)
	// CreationTime returns the piece creation time as given in the original PieceHash (which is
	// likely not the same as the file mtime). For non-FormatV0 pieces, this requires opening
	// the file and unmarshaling the piece header. If exact precision is not required, ModTime()
	// may be a better solution.
	CreationTime(ctx context.Context) (time.Time, error)
	// ModTime returns a less-precise piece creation time than CreationTime, but is generally
	// much faster. For non-FormatV0 pieces, this gets the piece creation time from to the
	// filesystem instead of the piece header.
	ModTime(ctx context.Context) (time.Time, error)
}

// SatelliteUsage contains information of how much space is used by a satellite.
type SatelliteUsage struct {
	Total       int64 // the total space used (including headers)
	ContentSize int64 // only content size used (excluding things like headers)
}

// Config is configuration for Store.
type Config struct {
	FileStatCache        string      `help:"optional type of file stat cache. Might be useful for slow disk and limited memory. Available options: badger (EXPERIMENTAL)"`
	WritePreallocSize    memory.Size `help:"deprecated" default:"4MiB"`
	DeleteToTrash        bool        `help:"move pieces to trash upon deletion. Warning: if set to false, you risk disqualification for failed audits if a satellite database is restored from backup." default:"true"`
	EnableLazyFilewalker bool        `help:"run garbage collection and used-space calculation filewalkers as a separate subprocess with lower IO priority" default:"true" testDefault:"false"`

	EnableFlatExpirationStore        bool          `help:"use flat files for the piece expiration store instead of a sqlite database" default:"true"`
	FlatExpirationStoreFileHandles   int           `help:"number of concurrent file handles to use for the flat expiration store" default:"1000"`
	FlatExpirationStorePath          string        `help:"where to store flat piece expiration files, relative to the data directory" default:"piece_expirations"`
	FlatExpirationStoreMaxBufferTime time.Duration `help:"maximum time to buffer writes to the flat expiration store before flushing" default:"5m"`
	FlatExpirationIncludeSQLite      bool          `help:"use and remove piece expirations from the sqlite database _also_ when the flat expiration store is enabled" default:"true"`

	TrashChoreInterval time.Duration `help:"how often to empty check the trash, and delete old files" default:"24h" testDefault:"-1s"`
}

// DefaultConfig is the default value for the Config.
var DefaultConfig = Config{
	WritePreallocSize:  4 * memory.MiB,
	TrashChoreInterval: 24 * time.Hour,
}

// Store implements storing pieces onto a blob storage implementation.
//
// architecture: Database
type Store struct {
	log    *zap.Logger
	config Config

	blobs          blobstore.Blobs
	expirationInfo PieceExpirationDB

	v0PieceInfo V0PieceInfoDB

	Filewalker     *FileWalker
	lazyFilewalker *lazyfilewalker.Supervisor
}

// StoreForTest is a wrapper around Store to be used only in test scenarios. It enables writing
// pieces with older storage formats.
type StoreForTest struct {
	*Store
}

// NewStore creates a new piece store.
func NewStore(log *zap.Logger, fw *FileWalker, lazyFilewalker *lazyfilewalker.Supervisor, blobs blobstore.Blobs, v0PieceInfo V0PieceInfoDB, expirationInfo PieceExpirationDB, config Config) *Store {
	return &Store{
		log:            log,
		config:         config,
		blobs:          blobs,
		expirationInfo: expirationInfo,
		v0PieceInfo:    v0PieceInfo,
		Filewalker:     fw,
		lazyFilewalker: lazyFilewalker,
	}
}

// CreateVerificationFile creates a file to be used for storage directory verification.
func (store *Store) CreateVerificationFile(ctx context.Context, id storj.NodeID) error {
	return store.blobs.CreateVerificationFile(ctx, id)
}

// VerifyStorageDir verifies that the storage directory is correct by checking for the existence and validity
// of the verification file.
func (store *Store) VerifyStorageDir(ctx context.Context, id storj.NodeID) error {
	return store.blobs.VerifyStorageDir(ctx, id)
}

// VerifyStorageDirWithTimeout verifies that the storage directory is correct by checking for the existence and validity
// of the verification file. It uses the provided timeout for the operation.
func (store *Store) VerifyStorageDirWithTimeout(ctx context.Context, id storj.NodeID, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ch := make(chan error, 1)
	go func() {
		ch <- store.VerifyStorageDir(ctx, id)
	}()

	select {
	case err := <-ch:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Writer returns a new piece writer.
func (store *Store) Writer(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID, hashAlgorithm pb.PieceHashAlgorithm) (_ *Writer, err error) {
	defer mon.Task()(&ctx)(&err)
	blobWriter, err := store.blobs.Create(ctx, blobstore.BlobRef{
		Namespace: satellite.Bytes(),
		Key:       pieceID.Bytes(),
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	writer, err := NewWriter(process.NamedLog(store.log, "blob-writer"), blobWriter, store.blobs, satellite, hashAlgorithm)
	return writer, Error.Wrap(err)
}

// WriterForFormatVersion allows opening a piece writer with a specified storage format version.
// This is meant to be used externally only in test situations (thus the StoreForTest receiver
// type).
func (store StoreForTest) WriterForFormatVersion(ctx context.Context, satellite storj.NodeID,
	pieceID storj.PieceID, formatVersion blobstore.FormatVersion, hashAlgorithm pb.PieceHashAlgorithm) (_ *Writer, err error) {

	defer mon.Task()(&ctx)(&err)

	blobRef := blobstore.BlobRef{
		Namespace: satellite.Bytes(),
		Key:       pieceID.Bytes(),
	}
	var blobWriter blobstore.BlobWriter
	switch formatVersion {
	case filestore.FormatV0:
		fStore, ok := store.blobs.(interface {
			TestCreateV0(ctx context.Context, ref blobstore.BlobRef) (_ blobstore.BlobWriter, err error)
		})
		if !ok {
			return nil, Error.New("can't make a WriterForFormatVersion with this blob store (%T)", store.blobs)
		}
		blobWriter, err = fStore.TestCreateV0(ctx, blobRef)
	case filestore.FormatV1:
		blobWriter, err = store.blobs.Create(ctx, blobRef)
	default:
		return nil, Error.New("please teach me how to make V%d pieces", formatVersion)
	}
	if err != nil {
		return nil, Error.Wrap(err)
	}
	writer, err := NewWriter(process.NamedLog(store.log, "blob-writer"), blobWriter, store.blobs, satellite, hashAlgorithm)
	return writer, Error.Wrap(err)
}

// ReaderWithStorageFormat returns a new piece reader for a located piece, which avoids the
// potential need to check multiple storage formats to find the right blob.
func (store *StoreForTest) ReaderWithStorageFormat(ctx context.Context, satellite storj.NodeID,
	pieceID storj.PieceID, formatVersion blobstore.FormatVersion) (_ *Reader, err error) {

	defer mon.Task()(&ctx)(&err)
	ref := blobstore.BlobRef{Namespace: satellite.Bytes(), Key: pieceID.Bytes()}
	blob, err := store.blobs.OpenWithStorageFormat(ctx, ref, formatVersion)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}
		return nil, Error.Wrap(err)
	}

	reader, err := NewReader(blob)
	return reader, Error.Wrap(err)
}

var monReader = mon.Task()

// Reader returns a new piece reader.
func (store *Store) Reader(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) (_ *Reader, err error) {
	defer monReader(&ctx)(&err)

	blob, err := store.blobs.Open(ctx, blobstore.BlobRef{
		Namespace: satellite.Bytes(),
		Key:       pieceID.Bytes(),
	})
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}
		return nil, Error.Wrap(err)
	}

	reader, err := NewReader(blob)
	return reader, Error.Wrap(err)
}

// TryRestoreTrashPiece attempts to restore a piece from the trash.
// It returns nil if the piece was restored, or an error if the piece was not in the trash.
func (store *Store) TryRestoreTrashPiece(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) (err error) {
	defer mon.Task()(&ctx)(&err)
	err = store.blobs.TryRestoreTrashBlob(ctx, blobstore.BlobRef{
		Namespace: satellite.Bytes(),
		Key:       pieceID.Bytes(),
	})
	if os.IsNotExist(err) {
		return err
	}
	return Error.Wrap(err)
}

// Delete deletes the specified piece.
func (store *Store) Delete(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) (err error) {
	defer mon.Task()(&ctx)(&err)
	err = store.blobs.Delete(ctx, blobstore.BlobRef{
		Namespace: satellite.Bytes(),
		Key:       pieceID.Bytes(),
	})
	if err != nil {
		return Error.Wrap(err)
	}
	if store.v0PieceInfo != nil {
		err := store.v0PieceInfo.Delete(ctx, satellite, pieceID)
		if err != nil {
			return Error.Wrap(err)
		}
	}
	return nil
}

var monDeleteSkipV0 = mon.Task()

// DeleteSkipV0 deletes the specified piece skipping V0 format and pieceinfo database.
func (store *Store) DeleteSkipV0(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID, pieceSize int64) (err error) {
	defer monDeleteSkipV0(&ctx)(&err)

	err = store.blobs.DeleteWithStorageFormat(ctx, blobstore.BlobRef{
		Namespace: satellite.Bytes(),
		Key:       pieceID.Bytes(),
	}, filestore.FormatV1, pieceSize)
	if err != nil {
		return Error.Wrap(err)
	}
	return nil
}

// DeleteExpiredV0 deletes all pieces with an expiration earlier than the provided time.
func (store *Store) DeleteExpiredV0(ctx context.Context, expiresAt time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	if store.v0PieceInfo != nil {
		err = store.v0PieceInfo.DeleteExpirations(ctx, expiresAt)
	}
	return Error.Wrap(err)
}

// DeleteExpiredBatchSkipV0 deletes the pieces in the batch skipping V0 format and pieceinfo database.
func (store *Store) DeleteExpiredBatchSkipV0(ctx context.Context, expireAt time.Time, opts ExpirationOptions) (err error) {
	defer mon.Task()(&ctx)(&err)
	return Error.Wrap(store.expirationInfo.DeleteExpirationsBatch(ctx, expireAt, opts))
}

// DeleteSatelliteBlobs deletes blobs folder of specific satellite after successful GE.
func (store *Store) DeleteSatelliteBlobs(ctx context.Context, satellite storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err = store.blobs.DeleteNamespace(ctx, satellite.Bytes()); err != nil {
		return Error.Wrap(err)
	}

	return Error.Wrap(store.Filewalker.usedSpaceDB.Delete(ctx, satellite))
}

var monTrash = mon.Task()

// Trash moves the specified piece to the blob trash. If necessary, it converts
// the v0 piece to a v1 piece. It also marks the item as "trashed" in the
// pieceExpirationDB.
func (store *Store) Trash(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID, timestamp time.Time) (err error) {
	defer monTrash(&ctx)(&err)

	// Check if the MaxFormatVersionSupported piece exists. If not, we assume
	// this is an old piece version and attempt to migrate it.
	_, err = store.blobs.StatWithStorageFormat(ctx, blobstore.BlobRef{
		Namespace: satellite.Bytes(),
		Key:       pieceID.Bytes(),
	}, filestore.MaxFormatVersionSupported)
	if err != nil && !errs.IsFunc(err, os.IsNotExist) {
		return Error.Wrap(err)
	}

	if errs.IsFunc(err, os.IsNotExist) {
		// MaxFormatVersionSupported does not exist, migrate.
		err = store.MigrateV0ToV1(ctx, satellite, pieceID)
		if err != nil {
			if !errs.Is(err, sql.ErrNoRows) {
				return Error.Wrap(err)
			}
			store.log.Warn("failed to migrate v0 piece. Piece may not be recoverable")
		}
	}

	// if V0 pieces was found we just migrated it so we can trash piece using specific storage format
	return Error.Wrap(store.blobs.TrashWithStorageFormat(ctx, blobstore.BlobRef{
		Namespace: satellite.Bytes(),
		Key:       pieceID.Bytes(),
	}, filestore.MaxFormatVersionSupported, timestamp))
}

// EmptyTrash deletes pieces in the trash that have been in there longer than trashExpiryInterval.
func (store *Store) EmptyTrash(ctx context.Context, satelliteID storj.NodeID, trashedBefore time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	if store.lazyFilewalkerEnabled() {
		bytesDeleted, _, err := store.lazyFilewalker.WalkCleanupTrash(ctx, satelliteID, trashedBefore)
		// The lazy filewalker does not update the space used by the trash so we need to update it here.
		if cache, ok := store.blobs.(*BlobsUsageCache); ok {
			cache.Update(ctx, satelliteID, 0, 0, -bytesDeleted)
		}
		return Error.Wrap(err)
	}
	_, _, err = store.blobs.EmptyTrash(ctx, satelliteID[:], trashedBefore)
	return Error.Wrap(err)
}

// RestoreTrash restores all pieces in the trash.
func (store *Store) RestoreTrash(ctx context.Context, satelliteID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = store.blobs.RestoreTrash(ctx, satelliteID.Bytes())
	return Error.Wrap(err)
}

// MigrateV0ToV1 will migrate a piece stored with storage format v0 to storage
// format v1. If the piece is not stored as a v0 piece it will return an error.
// The follow failures are possible:
//   - sql.ErrNoRows if the v0pieceInfoDB was corrupted or recreated.
//   - Fail to open or read v0 piece. In this case no artifacts remain.
//   - Fail to Write or Commit v1 piece. In this case no artifacts remain.
//   - Fail to Delete v0 piece. In this case v0 piece may remain,
//     but v1 piece will exist and be preferred in future calls.
func (store *Store) MigrateV0ToV1(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID) (err error) {
	defer mon.Task()(&ctx)(&err)

	info, err := store.v0PieceInfo.Get(ctx, satelliteID, pieceID)
	if err != nil {
		return Error.Wrap(err)
	}

	err = func() (err error) {
		r, err := store.Reader(ctx, satelliteID, pieceID)
		if err != nil {
			return err
		}
		defer func() { err = errs.Combine(err, r.Close()) }()

		w, err := store.Writer(ctx, satelliteID, pieceID, pb.PieceHashAlgorithm_SHA256)
		if err != nil {
			return err
		}

		_, err = io.Copy(w, r)
		if err != nil {
			return errs.Combine(err, w.Cancel(ctx))
		}

		header := &pb.PieceHeader{
			Hash:         w.Hash(),
			CreationTime: info.PieceCreation,
			Signature:    info.UplinkPieceHash.GetSignature(),
			OrderLimit:   *info.OrderLimit,
		}

		return w.Commit(ctx, header)
	}()
	if err != nil {
		return Error.Wrap(err)
	}

	err = store.blobs.DeleteWithStorageFormat(ctx, blobstore.BlobRef{
		Namespace: satelliteID.Bytes(),
		Key:       pieceID.Bytes(),
	}, filestore.FormatV0, info.PieceSize)

	if store.v0PieceInfo != nil {
		err = errs.Combine(err, store.v0PieceInfo.Delete(ctx, satelliteID, pieceID))
	}

	return Error.Wrap(err)
}

// GetV0PieceInfoDBForTest returns this piece-store's reference to the V0 piece info DB (or nil,
// if this piece-store does not have one). This is ONLY intended for use with testing
// functionality.
func (store StoreForTest) GetV0PieceInfoDBForTest() V0PieceInfoDBForTest {
	if store.v0PieceInfo == nil {
		return nil
	}
	return store.v0PieceInfo.(V0PieceInfoDBForTest)
}

// GetHashAndLimit returns the PieceHash and OrderLimit associated with the specified piece. The
// piece must already have been opened for reading, and the associated *Reader passed in.
//
// Once we have migrated everything off of V0 storage and no longer need to support it, this can
// cleanly become a method directly on *Reader and will need only the 'pieceID' parameter.
func (store *Store) GetHashAndLimit(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID, reader *Reader) (pb.PieceHash, pb.OrderLimit, error) {
	if reader.StorageFormatVersion() == filestore.FormatV0 {
		info, err := store.GetV0PieceInfo(ctx, satellite, pieceID)
		if err != nil {
			return pb.PieceHash{}, pb.OrderLimit{}, err // err is already wrapped as a storagenodedb.ErrPieceInfo
		}
		return *info.UplinkPieceHash, *info.OrderLimit, nil
	}
	header, err := reader.GetPieceHeader()
	if err != nil {
		return pb.PieceHash{}, pb.OrderLimit{}, Error.Wrap(err)
	}
	pieceHash := pb.PieceHash{
		PieceId:       pieceID,
		Hash:          header.GetHash(),
		HashAlgorithm: header.GetHashAlgorithm(),
		PieceSize:     reader.Size(),
		Timestamp:     header.GetCreationTime(),
		Signature:     header.GetSignature(),
	}
	return pieceHash, header.OrderLimit, nil
}

// WalkSatellitePieces wraps FileWalker.WalkSatellitePieces.
func (store *Store) WalkSatellitePieces(ctx context.Context, satellite storj.NodeID, walkFunc func(StoredPieceAccess) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	return store.WalkSatellitePiecesWithSkipPrefix(ctx, satellite, nil, walkFunc)
}

// WalkSatellitePiecesWithSkipPrefix is like WalkSatellitePieces, but accepts a skipPrefixFn.
func (store *Store) WalkSatellitePiecesWithSkipPrefix(ctx context.Context, satellite storj.NodeID, skipPrefixFn blobstore.SkipPrefixFn, walkFunc func(StoredPieceAccess) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	return store.Filewalker.WalkSatellitePieces(ctx, satellite, skipPrefixFn, walkFunc)
}

// WalkSatellitePiecesToTrash walks the satellite pieces and moves the pieces that are trash to the
// trash using the trashFunc provided.
//
// If the lazy filewalker is enabled, it will be used to find the pieces to trash, otherwise
// the regular filewalker will be used. If the lazy filewalker fails, the regular filewalker
// will be used as a fallback.
func (store *Store) WalkSatellitePiecesToTrash(ctx context.Context, satelliteID storj.NodeID, createdBefore time.Time, filter *bloomfilter.Filter, trashFunc func(pieceID storj.PieceID) error) (piecesCount, piecesSkipped int64, err error) {
	defer mon.Task()(&ctx, satelliteID, createdBefore)(&err)

	if store.lazyFilewalkerEnabled() {
		piecesCount, piecesSkipped, err = store.lazyFilewalker.WalkSatellitePiecesToTrash(ctx, satelliteID, createdBefore, filter, trashFunc)
		if err == nil {
			return piecesCount, piecesSkipped, nil
		}
		store.log.Error("lazyfilewalker failed", zap.Error(err))
	}
	// fallback to the regular filewalker
	return store.Filewalker.WalkSatellitePiecesToTrash(ctx, satelliteID, createdBefore, filter, trashFunc)
}

// GetExpired gets piece IDs that are expired and were created before the given time.
func (store *Store) GetExpired(ctx context.Context, expiredAt time.Time) (info []*ExpiredInfoRecords, err error) {
	defer mon.Task()(&ctx)(&err)

	info, err = store.GetExpiredBatchSkipV0(ctx, expiredAt, DefaultExpirationOptions())
	if err != nil {
		return nil, Error.Wrap(err)
	}
	if store.v0PieceInfo != nil {
		expired, err := store.v0PieceInfo.GetExpired(ctx, expiredAt)
		if err != nil {
			return nil, err
		}

		if expired != nil {
			info = append(info, expired...)
		}
	}
	return info, nil
}

// GetExpiredBatchSkipV0 gets piece IDs that are expired and were created before the given time
// limiting the number of pieces returned to the batch size.
// This method skips V0 pieces.
func (store *Store) GetExpiredBatchSkipV0(ctx context.Context, expiredAt time.Time, opts ExpirationOptions) (batch []*ExpiredInfoRecords, err error) {
	defer mon.Task()(&ctx)(&err)

	batch, err = store.expirationInfo.GetExpired(ctx, expiredAt, opts)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return batch, nil
}

// SetExpiration records an expiration time for the specified piece ID owned by the specified satellite.
func (store *Store) SetExpiration(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID, expiresAt time.Time, pieceSize int64) (err error) {
	return store.expirationInfo.SetExpiration(ctx, satellite, pieceID, expiresAt, pieceSize)
}

// SpaceUsedForPieces returns *an approximation of* the disk space used by all local pieces (both
// V0 and later). This is an approximation because changes may be being applied to the filestore as
// this information is collected, and because it is possible that various errors in directory
// traversal could cause this count to be undersized.
//
// Returns:
// - piecesTotal: the total space used by pieces, including headers
// - piecesContentSize: the space used by piece content, not including headers
//
// This returns both the total size of pieces plus the contentSize of pieces.
func (store *Store) SpaceUsedForPieces(ctx context.Context) (piecesTotal int64, piecesContentSize int64, err error) {
	if cache, ok := store.blobs.(*BlobsUsageCache); ok {
		return cache.SpaceUsedForPieces(ctx)
	}
	satellites, err := store.getAllStoringSatellites(ctx)
	if err != nil {
		return 0, 0, err
	}
	for _, satellite := range satellites {
		pieceTotal, pieceContentSize, err := store.SpaceUsedBySatellite(ctx, satellite)
		if err != nil {
			return 0, 0, err
		}
		piecesTotal += pieceTotal
		piecesContentSize += pieceContentSize
	}
	return piecesTotal, piecesContentSize, nil
}

// SpaceUsedForTrash returns the total space used by the piece store's
// trash, including all headers.
func (store *Store) SpaceUsedForTrash(ctx context.Context) (int64, error) {
	// If the blobs is cached, it will return the cached value
	return store.blobs.SpaceUsedForTrash(ctx)
}

// SpaceUsedForPiecesAndTrash returns the total space used by both active
// pieces and the trash directory.
func (store *Store) SpaceUsedForPiecesAndTrash(ctx context.Context) (int64, error) {
	piecesTotal, _, err := store.SpaceUsedForPieces(ctx)
	if err != nil {
		return 0, err
	}

	trashTotal, err := store.SpaceUsedForTrash(ctx)
	if err != nil {
		return 0, err
	}

	return piecesTotal + trashTotal, nil
}

// getAllStoringSatellites returns all the satellite IDs that have pieces stored in the blob store.
// This does not exclude untrusted satellites.
func (store *Store) getAllStoringSatellites(ctx context.Context) ([]storj.NodeID, error) {
	namespaces, err := store.blobs.ListNamespaces(ctx)
	if err != nil {
		return nil, err
	}
	satellites := make([]storj.NodeID, len(namespaces))
	for i, namespace := range namespaces {
		satellites[i], err = storj.NodeIDFromBytes(namespace)
		if err != nil {
			return nil, err
		}
	}
	return satellites, nil
}

// SpaceUsedBySatellite calculates *an approximation of* how much disk space is used for local
// piece storage in the given satellite's namespace. This is an approximation because changes may
// be being applied to the filestore as this information is collected, and because it is possible
// that various errors in directory traversal could cause this count to be undersized.
//
// This returns both the total size of pieces plus the contentSize of pieces.
func (store *Store) SpaceUsedBySatellite(ctx context.Context, satelliteID storj.NodeID) (piecesTotal, piecesContentSize int64, err error) {
	defer mon.Task()(&ctx)(&err)
	if cache, ok := store.blobs.(*BlobsUsageCache); ok {
		return cache.SpaceUsedBySatellite(ctx, satelliteID)
	}

	return store.WalkAndComputeSpaceUsedBySatellite(ctx, satelliteID, false)
}

// WalkAndComputeSpaceUsedBySatellite walks over all pieces for a given satellite, adds up and returns the total space used.
func (store *Store) WalkAndComputeSpaceUsedBySatellite(ctx context.Context, satelliteID storj.NodeID, lowerIOPriority bool) (piecesTotal, piecesContentSize int64, err error) {
	defer mon.Task()(&ctx)(&err)
	start := time.Now()

	var satPiecesTotal int64
	var satPiecesContentSize int64
	var satPiecesCount int64

	log := store.log.With(zap.Stringer("Satellite ID", satelliteID))

	log.Info("used-space-filewalker started")

	failover := true
	if lowerIOPriority {
		satPiecesTotal, satPiecesContentSize, satPiecesCount, err = store.lazyFilewalker.WalkAndComputeSpaceUsedBySatellite(ctx, satelliteID)
		if err != nil {
			log.Error("used-space-filewalker failed", zap.Bool("Lazy File Walker", true), zap.Error(err))
		} else {
			failover = false
		}
	}

	if failover {
		satPiecesTotal, satPiecesContentSize, satPiecesCount, err = store.Filewalker.WalkAndComputeSpaceUsedBySatellite(ctx, satelliteID)
		if err != nil {
			log.Error("used-space-filewalker failed", zap.Bool("Lazy File Walker", false), zap.Error(err))
		}
	}

	if err != nil {
		return 0, 0, err
	}

	log.Info("used-space-filewalker completed",
		zap.Bool("Lazy File Walker", !failover),
		zap.Int64("Total Pieces Size", satPiecesTotal),
		zap.Int64("Total Pieces Content Size", satPiecesContentSize),
		zap.Int64("Total Pieces Count", satPiecesCount),
		zap.Duration("Duration", time.Since(start)),
	)

	return satPiecesTotal, satPiecesContentSize, nil
}

func (store *Store) lazyFilewalkerEnabled() bool {
	return store.config.EnableLazyFilewalker && store.lazyFilewalker != nil
}

// SpaceUsedTotalAndBySatellite adds up the space used by and for all satellites for blob storage.
func (store *Store) SpaceUsedTotalAndBySatellite(ctx context.Context) (piecesTotal, piecesContentSize int64, totalBySatellite map[storj.NodeID]SatelliteUsage, err error) {
	defer mon.Task()(&ctx)(&err)

	satelliteIDs, err := store.getAllStoringSatellites(ctx)
	if err != nil {
		return 0, 0, nil, Error.New("failed to enumerate satellites: %w", err)
	}

	totalBySatellite = map[storj.NodeID]SatelliteUsage{}

	var group errs.Group
	for _, satelliteID := range satelliteIDs {
		satPiecesTotal, satPiecesContentSize, err := store.WalkAndComputeSpaceUsedBySatellite(ctx, satelliteID, store.lazyFilewalkerEnabled())
		if err != nil {
			group.Add(err)
			continue
		}

		piecesTotal += satPiecesTotal
		piecesContentSize += satPiecesContentSize
		totalBySatellite[satelliteID] = SatelliteUsage{
			Total:       satPiecesTotal,
			ContentSize: satPiecesContentSize,
		}
	}

	err = group.Err()
	if err != nil {
		return 0, 0, nil, Error.Wrap(err)
	}

	return piecesTotal, piecesContentSize, totalBySatellite, nil
}

// GetV0PieceInfo fetches the Info record from the V0 piece info database. Obviously,
// of no use when a piece does not have filestore.FormatV0 storage.
func (store *Store) GetV0PieceInfo(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) (*Info, error) {
	return store.v0PieceInfo.Get(ctx, satellite, pieceID)
}

// StorageStatus contains information about the disk store is using.
type StorageStatus struct {
	// DiskTotal is the actual disk size (not just the allocated disk space), in bytes.
	DiskTotal int64
	DiskUsed  int64
	// DiskFree is the actual amount of free space on the whole disk, not just allocated disk space, in bytes.
	DiskFree int64
}

// StorageStatus returns information about the disk.
func (store *Store) StorageStatus(ctx context.Context) (_ StorageStatus, err error) {
	defer mon.Task()(&ctx)(&err)
	info, err := store.blobs.DiskInfo(ctx)
	if err != nil {
		return StorageStatus{}, err
	}
	return StorageStatus{
		DiskTotal: info.TotalSpace,
		DiskUsed:  -1, // TODO set value
		DiskFree:  info.AvailableSpace,
	}, nil
}

// CheckWritability tests writability of the storage directory by creating and deleting a file.
func (store *Store) CheckWritability(ctx context.Context) error {
	return store.blobs.CheckWritability(ctx)
}

// CheckWritabilityWithTimeout tests writability of the storage directory by creating and deleting a file with a timeout.
func (store *Store) CheckWritabilityWithTimeout(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ch := make(chan error, 1)
	go func() {
		ch <- store.CheckWritability(ctx)
	}()

	select {
	case err := <-ch:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stat looks up disk metadata on the blob file.
func (store *Store) Stat(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) (blobstore.BlobInfo, error) {
	return store.blobs.Stat(ctx, blobstore.BlobRef{
		Namespace: satellite.Bytes(),
		Key:       pieceID.Bytes(),
	})
}

type storedPieceAccess struct {
	blobstore.BlobInfo
	pieceID storj.PieceID
	blobs   blobstore.Blobs
}

func newStoredPieceAccess(blobs blobstore.Blobs, blobInfo blobstore.BlobInfo) (storedPieceAccess, error) {
	ref := blobInfo.BlobRef()
	pieceID, err := storj.PieceIDFromBytes(ref.Key)
	if err != nil {
		return storedPieceAccess{}, err
	}

	return storedPieceAccess{
		BlobInfo: blobInfo,
		blobs:    blobs,
		pieceID:  pieceID,
	}, nil
}

// PieceID returns the piece ID of the piece.
func (access storedPieceAccess) PieceID() storj.PieceID {
	return access.pieceID
}

// Satellite returns the satellite ID that owns the piece.
func (access storedPieceAccess) Satellite() (storj.NodeID, error) {
	return storj.NodeIDFromBytes(access.BlobRef().Namespace)
}

// Size gives the size of the piece on disk, and the size of the content (not including the piece header, if applicable).
func (access storedPieceAccess) Size(ctx context.Context) (size, contentSize int64, err error) {
	// mon.Task() isn't used here because this operation can be executed milions of times.
	stat, err := access.Stat(ctx)
	if err != nil {
		return 0, 0, err
	}
	size = stat.Size()
	contentSize = size
	if access.StorageFormatVersion() >= filestore.FormatV1 {
		contentSize -= V1PieceHeaderReservedArea
	}
	return size, contentSize, nil
}

// CreationTime returns the piece creation time as given in the original PieceHash (which is likely
// not the same as the file mtime). This requires opening the file and unmarshaling the piece
// header. If exact precision is not required, ModTime() may be a better solution.
func (access storedPieceAccess) CreationTime(ctx context.Context) (cTime time.Time, err error) {
	defer mon.Task()(&ctx)(&err)

	blob, err := access.blobs.OpenWithStorageFormat(ctx, access.BlobInfo.BlobRef(), access.BlobInfo.StorageFormatVersion())
	if err != nil {
		if os.IsNotExist(err) {
			return time.Time{}, err
		}
		return time.Time{}, err
	}

	reader, err := NewReader(blob)
	if err != nil {
		return time.Time{}, err
	}
	defer func() {
		err = errs.Combine(err, reader.Close())
	}()

	header, err := reader.GetPieceHeader()
	if err != nil {
		return time.Time{}, err
	}
	return header.CreationTime, nil
}

// ModTime returns a less-precise piece creation time than CreationTime, but is generally
// much faster. This gets the piece creation time from to the filesystem instead of the
// piece header.
func (access storedPieceAccess) ModTime(ctx context.Context) (mTime time.Time, err error) {
	// mon.Task() isn't used here because this operation can be executed milions of times.
	stat, err := access.Stat(ctx)
	if err != nil {
		return time.Time{}, err
	}

	return stat.ModTime(), nil
}
