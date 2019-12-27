// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/storage"
	"storj.io/storj/storage/filestore"
)

const (
	preallocSize = 4 * memory.MiB
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

// ExpiredInfo is a fully namespaced piece id
type ExpiredInfo struct {
	SatelliteID storj.NodeID
	PieceID     storj.PieceID

	// This can be removed when we no longer need to support the pieceinfo db. Its only purpose
	// is to keep track of whether expired entries came from piece_expirations or pieceinfo.
	InPieceInfo bool
}

// PieceExpirationDB stores information about pieces with expiration dates.
//
// architecture: Database
type PieceExpirationDB interface {
	// GetExpired gets piece IDs that expire or have expired before the given time
	GetExpired(ctx context.Context, expiresBefore time.Time, limit int64) ([]ExpiredInfo, error)
	// SetExpiration sets an expiration time for the given piece ID on the given satellite
	SetExpiration(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID, expiresAt time.Time) error
	// DeleteExpiration removes an expiration record for the given piece ID on the given satellite
	DeleteExpiration(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) (found bool, err error)
	// DeleteFailed marks an expiration record as having experienced a failure in deleting the
	// piece from the disk
	DeleteFailed(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID, failedAt time.Time) error
	// Trash marks a piece as in the trash
	Trash(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID) error
	// RestoreTrash marks all piece as not being in trash
	RestoreTrash(ctx context.Context, satelliteID storj.NodeID) error
}

// V0PieceInfoDB stores meta information about pieces stored with storage format V0 (where
// metadata goes in the "pieceinfo" table in the storagenodedb). The actual pieces are stored
// behind something providing the storage.Blobs interface.
//
// architecture: Database
type V0PieceInfoDB interface {
	// Get returns Info about a piece.
	Get(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID) (*Info, error)
	// Delete deletes Info about a piece.
	Delete(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID) error
	// DeleteFailed marks piece deletion from disk failed
	DeleteFailed(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID, failedAt time.Time) error
	// GetExpired gets piece IDs stored with storage format V0 that expire or have expired
	// before the given time
	GetExpired(ctx context.Context, expiredAt time.Time, limit int64) ([]ExpiredInfo, error)
	// WalkSatelliteV0Pieces executes walkFunc for each locally stored piece, stored
	// with storage format V0 in the namespace of the given satellite. If walkFunc returns a
	// non-nil error, WalkSatelliteV0Pieces will stop iterating and return the error
	// immediately. The ctx parameter is intended specifically to allow canceling iteration
	// early.
	WalkSatelliteV0Pieces(ctx context.Context, blobStore storage.Blobs, satellite storj.NodeID, walkFunc func(StoredPieceAccess) error) error
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

// PieceSpaceUsedDB stores the most recent totals from the space used cache
//
// architecture: Database
type PieceSpaceUsedDB interface {
	// Init creates the one total and trash record if it doesn't already exist
	Init(ctx context.Context) error
	// GetPieceTotal returns the total space used by all pieces stored on disk
	GetPieceTotal(ctx context.Context) (int64, error)
	// UpdatePieceTotal updates the record for total spaced used for pieces with a new value
	UpdatePieceTotal(ctx context.Context, newTotal int64) error
	// GetTotalsForAllSatellites returns how much total space used by pieces stored on disk for each satelliteID
	GetPieceTotalsForAllSatellites(ctx context.Context) (map[storj.NodeID]int64, error)
	// UpdatePieceTotalsForAllSatellites updates each record for total spaced used with a new value for each satelliteID
	UpdatePieceTotalsForAllSatellites(ctx context.Context, newTotalsBySatellites map[storj.NodeID]int64) error
	// GetTrashTotal returns the total space used by trash
	GetTrashTotal(ctx context.Context) (int64, error)
	// UpdateTrashTotal updates the record for total spaced used for trash with a new value
	UpdateTrashTotal(ctx context.Context, newTotal int64) error
}

// StoredPieceAccess allows inspection and manipulation of a piece during iteration with
// WalkSatellitePieces-type methods.
type StoredPieceAccess interface {
	storage.BlobInfo

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

// Store implements storing pieces onto a blob storage implementation.
//
// architecture: Database
type Store struct {
	log            *zap.Logger
	blobs          storage.Blobs
	v0PieceInfo    V0PieceInfoDB
	expirationInfo PieceExpirationDB
	spaceUsedDB    PieceSpaceUsedDB
}

// StoreForTest is a wrapper around Store to be used only in test scenarios. It enables writing
// pieces with older storage formats
type StoreForTest struct {
	*Store
}

// NewStore creates a new piece store
func NewStore(log *zap.Logger, blobs storage.Blobs, v0PieceInfo V0PieceInfoDB, expirationInfo PieceExpirationDB, pieceSpaceUsedDB PieceSpaceUsedDB) *Store {
	return &Store{
		log:            log,
		blobs:          blobs,
		v0PieceInfo:    v0PieceInfo,
		expirationInfo: expirationInfo,
		spaceUsedDB:    pieceSpaceUsedDB,
	}
}

// Writer returns a new piece writer.
func (store *Store) Writer(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) (_ *Writer, err error) {
	defer mon.Task()(&ctx)(&err)
	blobWriter, err := store.blobs.Create(ctx, storage.BlobRef{
		Namespace: satellite.Bytes(),
		Key:       pieceID.Bytes(),
	}, preallocSize.Int64())
	if err != nil {
		return nil, Error.Wrap(err)
	}

	writer, err := NewWriter(blobWriter, store.blobs, satellite)
	return writer, Error.Wrap(err)
}

// WriterForFormatVersion allows opening a piece writer with a specified storage format version.
// This is meant to be used externally only in test situations (thus the StoreForTest receiver
// type).
func (store StoreForTest) WriterForFormatVersion(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID, formatVersion storage.FormatVersion) (_ *Writer, err error) {
	defer mon.Task()(&ctx)(&err)

	blobRef := storage.BlobRef{
		Namespace: satellite.Bytes(),
		Key:       pieceID.Bytes(),
	}
	var blobWriter storage.BlobWriter
	switch formatVersion {
	case filestore.FormatV0:
		fStore, ok := store.blobs.(interface {
			TestCreateV0(ctx context.Context, ref storage.BlobRef) (_ storage.BlobWriter, err error)
		})
		if !ok {
			return nil, Error.New("can't make a WriterForFormatVersion with this blob store (%T)", store.blobs)
		}
		blobWriter, err = fStore.TestCreateV0(ctx, blobRef)
	case filestore.FormatV1:
		blobWriter, err = store.blobs.Create(ctx, blobRef, preallocSize.Int64())
	default:
		return nil, Error.New("please teach me how to make V%d pieces", formatVersion)
	}
	if err != nil {
		return nil, Error.Wrap(err)
	}
	writer, err := NewWriter(blobWriter, store.blobs, satellite)
	return writer, Error.Wrap(err)
}

// Reader returns a new piece reader.
func (store *Store) Reader(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) (_ *Reader, err error) {
	defer mon.Task()(&ctx)(&err)
	blob, err := store.blobs.Open(ctx, storage.BlobRef{
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

// ReaderWithStorageFormat returns a new piece reader for a located piece, which avoids the
// potential need to check multiple storage formats to find the right blob.
func (store *Store) ReaderWithStorageFormat(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID, formatVersion storage.FormatVersion) (_ *Reader, err error) {
	defer mon.Task()(&ctx)(&err)
	ref := storage.BlobRef{Namespace: satellite.Bytes(), Key: pieceID.Bytes()}
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

// Delete deletes the specified piece.
func (store *Store) Delete(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) (err error) {
	defer mon.Task()(&ctx)(&err)
	err = store.blobs.Delete(ctx, storage.BlobRef{
		Namespace: satellite.Bytes(),
		Key:       pieceID.Bytes(),
	})
	if err != nil {
		return Error.Wrap(err)
	}

	// delete records in both the piece_expirations and pieceinfo DBs, wherever we find it.
	// both of these calls should return no error if the requested record is not found.
	if store.expirationInfo != nil {
		_, err = store.expirationInfo.DeleteExpiration(ctx, satellite, pieceID)
	}
	if store.v0PieceInfo != nil {
		err = errs.Combine(err, store.v0PieceInfo.Delete(ctx, satellite, pieceID))
	}

	return Error.Wrap(err)
}

// Trash moves the specified piece to the blob trash. If necessary, it converts
// the v0 piece to a v1 piece. It also marks the item as "trashed" in the
// pieceExpirationDB.
func (store *Store) Trash(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) (err error) {
	defer mon.Task()(&ctx)(&err)

	// Check if the MaxFormatVersionSupported piece exists. If not, we assume
	// this is an old piece version and attempt to migrate it.
	_, err = store.blobs.StatWithStorageFormat(ctx, storage.BlobRef{
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
			return Error.Wrap(err)
		}
	}

	err = store.expirationInfo.Trash(ctx, satellite, pieceID)
	err = errs.Combine(err, store.blobs.Trash(ctx, storage.BlobRef{
		Namespace: satellite.Bytes(),
		Key:       pieceID.Bytes(),
	}))

	return Error.Wrap(err)
}

// EmptyTrash deletes pieces in the trash that have been in there longer than trashExpiryInterval
func (store *Store) EmptyTrash(ctx context.Context, satelliteID storj.NodeID, trashedBefore time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, deletedIDs, err := store.blobs.EmptyTrash(ctx, satelliteID[:], trashedBefore)
	if err != nil {
		return Error.Wrap(err)
	}

	for _, deletedID := range deletedIDs {
		pieceID, pieceIDErr := storj.PieceIDFromBytes(deletedID)
		if pieceIDErr != nil {
			return Error.Wrap(pieceIDErr)
		}
		_, deleteErr := store.expirationInfo.DeleteExpiration(ctx, satelliteID, pieceID)
		err = errs.Combine(err, deleteErr)
	}
	return Error.Wrap(err)
}

// RestoreTrash restores all pieces in the trash
func (store *Store) RestoreTrash(ctx context.Context, satelliteID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = store.blobs.RestoreTrash(ctx, satelliteID.Bytes())
	if err != nil {
		return Error.Wrap(err)
	}
	return Error.Wrap(store.expirationInfo.RestoreTrash(ctx, satelliteID))
}

// MigrateV0ToV1 will migrate a piece stored with storage format v0 to storage
// format v1. If the piece is not stored as a v0 piece it will return an error.
// The follow failures are possible:
// - Fail to open or read v0 piece. In this case no artifacts remain.
// - Fail to Write or Commit v1 piece. In this case no artifacts remain.
// - Fail to Delete v0 piece. In this case v0 piece may remain, but v1 piece
//   will exist and be preferred in future calls.
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

		w, err := store.Writer(ctx, satelliteID, pieceID)
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

	err = store.blobs.DeleteWithStorageFormat(ctx, storage.BlobRef{
		Namespace: satelliteID.Bytes(),
		Key:       pieceID.Bytes(),
	}, filestore.FormatV0)

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
		PieceId:   pieceID,
		Hash:      header.GetHash(),
		PieceSize: reader.Size(),
		Timestamp: header.GetCreationTime(),
		Signature: header.GetSignature(),
	}
	return pieceHash, header.OrderLimit, nil
}

// WalkSatellitePieces executes walkFunc for each locally stored piece in the namespace of the
// given satellite. If walkFunc returns a non-nil error, WalkSatellitePieces will stop iterating
// and return the error immediately. The ctx parameter is intended specifically to allow canceling
// iteration early.
//
// Note that this method includes all locally stored pieces, both V0 and higher.
func (store *Store) WalkSatellitePieces(ctx context.Context, satellite storj.NodeID, walkFunc func(StoredPieceAccess) error) (err error) {
	defer mon.Task()(&ctx)(&err)
	// first iterate over all in V1 storage, then all in V0
	err = store.blobs.WalkNamespace(ctx, satellite.Bytes(), func(blobInfo storage.BlobInfo) error {
		if blobInfo.StorageFormatVersion() < filestore.FormatV1 {
			// we'll address this piece while iterating over the V0 pieces below.
			return nil
		}
		pieceAccess, err := newStoredPieceAccess(store, blobInfo)
		if err != nil {
			// this is not a real piece blob. the blob store can't distinguish between actual piece
			// blobs and stray files whose names happen to decode as valid base32. skip this
			// "blob".
			return nil
		}
		return walkFunc(pieceAccess)
	})
	if err == nil && store.v0PieceInfo != nil {
		err = store.v0PieceInfo.WalkSatelliteV0Pieces(ctx, store.blobs, satellite, walkFunc)
	}
	return err
}

// GetExpired gets piece IDs that are expired and were created before the given time
func (store *Store) GetExpired(ctx context.Context, expiredAt time.Time, limit int64) (_ []ExpiredInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	expired, err := store.expirationInfo.GetExpired(ctx, expiredAt, limit)
	if err != nil {
		return nil, err
	}
	if int64(len(expired)) < limit && store.v0PieceInfo != nil {
		v0Expired, err := store.v0PieceInfo.GetExpired(ctx, expiredAt, limit-int64(len(expired)))
		if err != nil {
			return nil, err
		}
		expired = append(expired, v0Expired...)
	}
	return expired, nil
}

// SetExpiration records an expiration time for the specified piece ID owned by the specified satellite
func (store *Store) SetExpiration(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID, expiresAt time.Time) (err error) {
	return store.expirationInfo.SetExpiration(ctx, satellite, pieceID, expiresAt)
}

// DeleteFailed marks piece as a failed deletion.
func (store *Store) DeleteFailed(ctx context.Context, expired ExpiredInfo, when time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	if expired.InPieceInfo {
		return store.v0PieceInfo.DeleteFailed(ctx, expired.SatelliteID, expired.PieceID, when)
	}
	return store.expirationInfo.DeleteFailed(ctx, expired.SatelliteID, expired.PieceID, when)
}

// SpaceUsedForPieces returns *an approximation of* the disk space used by all local pieces (both
// V0 and later). This is an approximation because changes may be being applied to the filestore as
// this information is collected, and because it is possible that various errors in directory
// traversal could cause this count to be undersized.
//
// Important note: this metric does not include space used by piece headers, whereas
// storj/filestore/store.(*Store).SpaceUsedForBlobs() *does* include all space used by the blobs.
func (store *Store) SpaceUsedForPieces(ctx context.Context) (int64, error) {
	if cache, ok := store.blobs.(*BlobsUsageCache); ok {
		return cache.SpaceUsedForPieces(ctx)
	}
	satellites, err := store.getAllStoringSatellites(ctx)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, satellite := range satellites {
		spaceUsed, err := store.SpaceUsedBySatellite(ctx, satellite)
		if err != nil {
			return 0, err
		}
		total += spaceUsed
	}
	return total, nil
}

// SpaceUsedForTrash returns the total space used by the the piece store's trash
func (store *Store) SpaceUsedForTrash(ctx context.Context) (int64, error) {
	// If the blobs is cached, it will return the cached value
	return store.blobs.SpaceUsedForTrash(ctx)
}

// SpaceUsedForPiecesAndTrash returns the total space used by both active
// pieces and the trash directory
func (store *Store) SpaceUsedForPiecesAndTrash(ctx context.Context) (int64, error) {
	pieces, err := store.SpaceUsedForPieces(ctx)
	if err != nil {
		return 0, err
	}

	trash, err := store.SpaceUsedForTrash(ctx)
	if err != nil {
		return 0, err
	}

	return pieces + trash, nil
}

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
// Important note: this metric does not include space used by piece headers, whereas
// storj/filestore/store.(*Store).SpaceUsedForBlobsInNamespace() *does* include all space used by the
// blobs.
func (store *Store) SpaceUsedBySatellite(ctx context.Context, satelliteID storj.NodeID) (int64, error) {
	if cache, ok := store.blobs.(*BlobsUsageCache); ok {
		return cache.SpaceUsedBySatellite(ctx, satelliteID)
	}

	var totalUsed int64
	err := store.WalkSatellitePieces(ctx, satelliteID, func(access StoredPieceAccess) error {
		_, contentSize, statErr := access.Size(ctx)
		if statErr != nil {
			store.log.Error("failed to stat", zap.Error(statErr), zap.Stringer("Piece ID", access.PieceID()), zap.Stringer("Satellite ID", satelliteID))
			// keep iterating; we want a best effort total here.
			return nil
		}
		totalUsed += contentSize
		return nil
	})
	if err != nil {
		return 0, err
	}
	return totalUsed, nil
}

// SpaceUsedTotalAndBySatellite adds up the space used by and for all satellites for blob storage
func (store *Store) SpaceUsedTotalAndBySatellite(ctx context.Context) (total int64, totalBySatellite map[storj.NodeID]int64, err error) {
	defer mon.Task()(&ctx)(&err)

	satelliteIDs, err := store.getAllStoringSatellites(ctx)
	if err != nil {
		return total, totalBySatellite, Error.New("failed to enumerate satellites: %v", err)
	}

	totalBySatellite = map[storj.NodeID]int64{}
	for _, satelliteID := range satelliteIDs {
		var totalUsed int64

		err := store.WalkSatellitePieces(ctx, satelliteID, func(access StoredPieceAccess) error {
			_, contentSize, err := access.Size(ctx)
			if err != nil {
				return err
			}
			totalUsed += contentSize
			return nil
		})
		if err != nil {
			return total, totalBySatellite, err
		}

		total += totalUsed
		totalBySatellite[satelliteID] = totalUsed
	}
	return total, totalBySatellite, nil
}

// GetV0PieceInfo fetches the Info record from the V0 piece info database. Obviously,
// of no use when a piece does not have filestore.FormatV0 storage.
func (store *Store) GetV0PieceInfo(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) (*Info, error) {
	return store.v0PieceInfo.Get(ctx, satellite, pieceID)
}

// StorageStatus contains information about the disk store is using.
type StorageStatus struct {
	DiskUsed int64
	DiskFree int64
}

// StorageStatus returns information about the disk.
func (store *Store) StorageStatus(ctx context.Context) (_ StorageStatus, err error) {
	defer mon.Task()(&ctx)(&err)
	diskFree, err := store.blobs.FreeSpace()
	if err != nil {
		return StorageStatus{}, err
	}
	return StorageStatus{
		DiskUsed: -1, // TODO set value
		DiskFree: diskFree,
	}, nil
}

type storedPieceAccess struct {
	storage.BlobInfo
	store   *Store
	pieceID storj.PieceID
}

func newStoredPieceAccess(store *Store, blobInfo storage.BlobInfo) (storedPieceAccess, error) {
	pieceID, err := storj.PieceIDFromBytes(blobInfo.BlobRef().Key)
	if err != nil {
		return storedPieceAccess{}, err
	}
	return storedPieceAccess{
		BlobInfo: blobInfo,
		store:    store,
		pieceID:  pieceID,
	}, nil
}

// PieceID returns the piece ID of the piece
func (access storedPieceAccess) PieceID() storj.PieceID {
	return access.pieceID
}

// Satellite returns the satellite ID that owns the piece
func (access storedPieceAccess) Satellite() (storj.NodeID, error) {
	return storj.NodeIDFromBytes(access.BlobRef().Namespace)
}

// Size gives the size of the piece on disk, and the size of the content (not including the piece header, if applicable)
func (access storedPieceAccess) Size(ctx context.Context) (size, contentSize int64, err error) {
	defer mon.Task()(&ctx)(&err)
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
	satellite, err := access.Satellite()
	if err != nil {
		return time.Time{}, err
	}
	reader, err := access.store.ReaderWithStorageFormat(ctx, satellite, access.PieceID(), access.StorageFormatVersion())
	if err != nil {
		return time.Time{}, err
	}
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
	defer mon.Task()(&ctx)(&err)
	stat, err := access.Stat(ctx)
	if err != nil {
		return time.Time{}, err
	}

	return stat.ModTime(), nil
}
