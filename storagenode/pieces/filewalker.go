// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"context"
	"database/sql"
	"os"
	"runtime"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/shared/bloomfilter"
	"storj.io/storj/storagenode/blobstore"
	"storj.io/storj/storagenode/blobstore/filestore"
)

const maxPrefixUsedSpaceBatch = 5

var errFileWalker = errs.Class("filewalker")

// FileWalker implements methods to walk over pieces in a storage directory.
type FileWalker struct {
	log *zap.Logger

	blobs        blobstore.Blobs
	v0PieceInfo  V0PieceInfoDB
	gcProgressDB GCFilewalkerProgressDB
	usedSpaceDB  UsedSpacePerPrefixDB
}

// NewFileWalker creates a new FileWalker.
func NewFileWalker(log *zap.Logger, blobs blobstore.Blobs, v0PieceInfoDB V0PieceInfoDB, gcProgressDB GCFilewalkerProgressDB, usedSpaceDB UsedSpacePerPrefixDB) *FileWalker {
	return &FileWalker{
		log:          log,
		blobs:        blobs,
		v0PieceInfo:  v0PieceInfoDB,
		gcProgressDB: gcProgressDB,
		usedSpaceDB:  usedSpaceDB,
	}
}

// WalkSatellitePieces executes walkFunc for each locally stored piece in the namespace of the
// given satellite. If walkFunc returns a non-nil error, WalkSatellitePieces will stop iterating
// and return the error immediately. The ctx parameter is intended specifically to allow canceling
// iteration early.
//
// Note that this method includes all locally stored pieces, both V0 and higher.
// The startPrefix parameter can be used to start the iteration at a specific prefix. If startPrefix
// is empty, the iteration starts at the beginning of the namespace.
func (fw *FileWalker) WalkSatellitePieces(ctx context.Context, satellite storj.NodeID, skipPrefixFn blobstore.SkipPrefixFn, fn func(StoredPieceAccess) error) (err error) {
	defer mon.Task()(&ctx)(&err)
	// iterate over all in V1 storage, skipping v0 pieces
	err = fw.blobs.WalkNamespace(ctx, satellite.Bytes(), skipPrefixFn, func(blobInfo blobstore.BlobInfo) error {
		if blobInfo.StorageFormatVersion() < filestore.FormatV1 {
			// skip v0 pieces, which are handled separately
			return nil
		}
		pieceAccess, err := newStoredPieceAccess(fw.blobs, blobInfo)
		if err != nil {
			// this is not a real piece blob. the blob store can't distinguish between actual piece
			// blobs and stray files whose names happen to decode as valid base32. skip this
			// "blob".
			return nil //nolint: nilerr // we ignore other files
		}
		return fn(pieceAccess)
	})

	if err == nil && fw.v0PieceInfo != nil {
		// iterate over all in V0 storage
		err = fw.v0PieceInfo.WalkSatelliteV0Pieces(ctx, fw.blobs, satellite, fn)
	}

	return errFileWalker.Wrap(err)
}

// WalkAndComputeSpaceUsedBySatellite walks over all pieces for a given satellite, adds up and returns the total space used.
func (fw *FileWalker) WalkAndComputeSpaceUsedBySatellite(ctx context.Context, satelliteID storj.NodeID) (satPiecesTotal int64, satPiecesContentSize int64, satPieceCount int64, err error) {
	return fw.WalkAndComputeSpaceUsedBySatelliteWithWalkFunc(ctx, satelliteID, nil)
}

// WalkAndComputeSpaceUsedBySatelliteWithWalkFunc walks over all pieces for a given satellite, adds up and returns the total space used.
// It also calls the walkFunc for each piece.
// This is useful for testing purposes. Call this method with a walkFunc that collects information about each piece.
func (fw *FileWalker) WalkAndComputeSpaceUsedBySatelliteWithWalkFunc(ctx context.Context, satelliteID storj.NodeID, walkFunc func(StoredPieceAccess) error) (satPiecesTotal int64, satPiecesContentSize int64, satPieceCount int64, err error) {
	satelliteUsedSpacePerPrefix := make(map[string]PrefixUsedSpace)
	var skipPrefixFunc blobstore.SkipPrefixFn
	if fw.usedSpaceDB != nil {
		// hardcoded 7 days, if the used space is not updated in the last 7 days, we will recalculate it.
		// TODO: make this configurable
		lastUpdated := time.Now().Add(-time.Hour * 168)
		usedSpace, err := fw.usedSpaceDB.Get(ctx, satelliteID, &lastUpdated)
		if err != nil && !errs.Is(err, sql.ErrNoRows) {
			return 0, 0, 0, errFileWalker.Wrap(err)
		}

		for _, prefix := range usedSpace {
			satelliteUsedSpacePerPrefix[prefix.Prefix] = prefix
		}

		if len(satelliteUsedSpacePerPrefix) > 0 {
			skipPrefixFunc = func(prefix string) bool {
				if usedSpace, ok := satelliteUsedSpacePerPrefix[prefix]; ok {
					return usedSpace.TotalBytes > 0 && usedSpace.TotalContentSize > 0
				}
				return false
			}
		}
	}

	scannedPrefixesBatch := make([]PrefixUsedSpace, 0, maxPrefixUsedSpaceBatch)
	storeBatch := func() error {
		if fw.usedSpaceDB != nil && len(scannedPrefixesBatch) > 0 {
			err := fw.usedSpaceDB.StoreBatch(ctx, scannedPrefixesBatch)
			if err != nil {
				return err
			}
			scannedPrefixesBatch = scannedPrefixesBatch[:0]
		}
		return nil
	}

	currentPrefix := PrefixUsedSpace{}

	err = fw.WalkSatellitePieces(ctx, satelliteID, skipPrefixFunc, func(access StoredPieceAccess) error {
		if len(scannedPrefixesBatch) >= maxPrefixUsedSpaceBatch {
			if err := storeBatch(); err != nil {
				fw.log.Error("failed to store the batch of prefixes", zap.Error(err))
				return err
			}
		}

		if walkFunc != nil {
			err := walkFunc(access)
			if err != nil {
				return err
			}
		}

		prefix := getKeyPrefix(access.BlobRef())

		pieceTotal, pieceContentSize, err := access.Size(ctx)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		satPiecesTotal += pieceTotal
		satPiecesContentSize += pieceContentSize
		satPieceCount++

		if prefix != currentPrefix.Prefix {
			// add the current prefix to the batch and start a new one
			if currentPrefix.Prefix != "" {
				scannedPrefixesBatch = append(scannedPrefixesBatch, currentPrefix)
			}
			currentPrefix = PrefixUsedSpace{Prefix: prefix, SatelliteID: satelliteID}
		}
		currentPrefix.TotalBytes += pieceTotal
		currentPrefix.TotalContentSize += pieceContentSize
		currentPrefix.PieceCounts++
		currentPrefix.LastUpdated = time.Now().UTC()

		return nil
	})

	if err == nil && currentPrefix.Prefix != "" {
		// if no error occurred, then the last prefix was completely scanned, so we need to store it.
		scannedPrefixesBatch = append(scannedPrefixesBatch, currentPrefix)
	}

	// filewalker is done, store the last batch, if any;
	// at least try even if there was an error, to avoid losing progress.
	if storeErr := storeBatch(); storeErr != nil {
		fw.log.Error("failed to store the last batch of prefixes", zap.Error(err))
		return satPiecesTotal, satPiecesContentSize, satPieceCount, errFileWalker.Wrap(errs.Combine(err, storeErr))
	}

	if err == nil && fw.usedSpaceDB != nil {
		if len(satelliteUsedSpacePerPrefix) > 0 {
			var getErr error
			// if we started from a specific prefix, then the calculated data is incomplete, so let's get
			// the actual total used space for the satellite from the database.
			satPiecesTotal, satPiecesContentSize, satPieceCount, getErr = fw.usedSpaceDB.GetSatelliteUsedSpace(ctx, satelliteID)
			if getErr != nil {
				fw.log.Error("failed to get total used space from the database", zap.Error(err))
				err = errs.Combine(err, getErr)
			}
		}
	}

	return satPiecesTotal, satPiecesContentSize, satPieceCount, errFileWalker.Wrap(err)
}

// WalkSatellitePiecesToTrash walks the satellite pieces and moves the pieces that are trash to the
// trash using the trashFunc provided
//
// ------------------------------------------------------------------------------------------------
//
// On the correctness of using access.ModTime() in place of the more precise access.CreationTime():
//
// ------------------------------------------------------------------------------------------------
//
// Background: for pieces not stored with storage.FormatV0, the access.CreationTime() value can
// only be retrieved by opening the piece file, and reading and unmarshaling the piece header.
// This is far slower than access.ModTime(), which gets the file modification time from the file
// system and only needs to do a stat(2) on the piece file. If we can make Retain() work with
// ModTime, we should.
//
// Possibility of mismatch: We do not force or require piece file modification times to be equal to
// or close to the CreationTime specified by the uplink, but we do expect that piece files will be
// written to the filesystem _after_ the CreationTime. We make the assumption already that storage
// nodes and satellites and uplinks have system clocks that are very roughly in sync (that is, they
// are out of sync with each other by less than an hour of real time, or whatever is configured as
// MaxTimeSkew). So if an uplink is not lying about CreationTime and it uploads a piece that
// makes it to a storagenode's disk as quickly as possible, even in the worst-synchronized-clocks
// case we can assume that `ModTime > (CreationTime - MaxTimeSkew)`. We also allow for storage
// node operators doing file system manipulations after a piece has been written. If piece files
// are copied between volumes and their attributes are not preserved, it will be possible for their
// modification times to be changed to something later in time. This still preserves the inequality
// relationship mentioned above, `ModTime > (CreationTime - MaxTimeSkew)`. We only stipulate
// that storage node operators must not artificially change blob file modification times to be in
// the past.
//
// If there is a mismatch: in most cases, a mismatch between ModTime and CreationTime has no
// effect. In certain remaining cases, the only effect is that a piece file which _should_ be
// garbage collected survives until the next round of garbage collection. The only really
// problematic case is when there is a relatively new piece file which was created _after_ this
// node's Retain bloom filter started being built on the satellite, and is recorded in this
// storage node's blob store before the Retain operation has completed. Then, it might be possible
// for that new piece to be garbage collected incorrectly, because it does not show up in the
// bloom filter and the node incorrectly thinks that it was created before the bloom filter.
// But if the uplink is not lying about CreationTime and its clock drift versus the storage node
// is less than `MaxTimeSkew`, and the ModTime on a blob file is correctly set from the
// storage node system time, then it is still true that `ModTime > (CreationTime -
// MaxTimeSkew)`.
//
// The rule that storage node operators need to be aware of is only this: do not artificially set
// mtimes on blob files to be in the past. Let the filesystem manage mtimes. If blob files need to
// be moved or copied between locations, and this updates the mtime, that is ok. A secondary effect
// of this rule is that if the storage node's system clock needs to be changed forward by a
// nontrivial amount, mtimes on existing blobs should also be adjusted (by the same interval,
// ideally, but just running "touch" on all blobs is sufficient to avoid incorrect deletion of
// data).
func (fw *FileWalker) WalkSatellitePiecesToTrash(ctx context.Context, satelliteID storj.NodeID, createdBefore time.Time, filter *bloomfilter.Filter, trashFunc func(pieceID storj.PieceID) error) (piecesCount, piecesSkipped int64, err error) {
	defer mon.Task()(&ctx)(&err)

	if filter == nil {
		return 0, 0, Error.New("filter not specified")
	}

	var curPrefix string
	if fw.gcProgressDB != nil {
		progress, progressErr := fw.gcProgressDB.Get(ctx, satelliteID)
		if progressErr != nil && !errs.Is(progressErr, sql.ErrNoRows) {
			fw.log.Error("failed to get progress from database", zap.Error(err))
		}
		curPrefix = progress.Prefix

		if curPrefix != "" && !progress.BloomfilterCreatedBefore.Equal(createdBefore) {
			fw.log.Debug("bloomfilter createdBefore time does not match the one used in the last scan",
				zap.Time("lastBloomfilterCreatedBefore", progress.BloomfilterCreatedBefore),
				zap.Time("currentBloomfilterCreatedBefore", createdBefore))

			// The bloomfilter createdBefore time has changed since the last scan which indicates that this is
			// a new bloomfilter. We need to start over.
			curPrefix = ""
		}

		defer func() {
			if err == nil { // reset progress if completed successfully
				fw.log.Debug("resetting progress in database")
				err = fw.gcProgressDB.Reset(ctx, satelliteID)
				if err != nil {
					fw.log.Error("failed to reset progress in database", zap.Error(err))
				}
			}
		}()
	}

	var skipPrefixFunc blobstore.SkipPrefixFn

	if curPrefix != "" {
		foundLastPrefix := false
		skipPrefixFunc = func(prefix string) bool {
			if foundLastPrefix {
				return false
			}
			if prefix == curPrefix {
				foundLastPrefix = true
			}
			return true
		}
	}

	err = fw.WalkSatellitePieces(ctx, satelliteID, skipPrefixFunc, func(access StoredPieceAccess) error {
		piecesCount++

		// We call Gosched() when done because the GC process is expected to be long and we want to keep it at low priority,
		// so other goroutines can continue serving requests.
		defer runtime.Gosched()

		if fw.gcProgressDB != nil {
			keyPrefix := getKeyPrefix(access.BlobRef())
			if keyPrefix != "" && keyPrefix != curPrefix {
				err := fw.gcProgressDB.Store(ctx, GCFilewalkerProgress{
					Prefix:                   keyPrefix,
					SatelliteID:              satelliteID,
					BloomfilterCreatedBefore: createdBefore,
				})
				if err != nil {
					fw.log.Error("failed to save progress in the database", zap.Error(err))
				}
				curPrefix = keyPrefix
			}
		}

		pieceID := access.PieceID()
		if filter.Contains(pieceID) {
			// This piece is explicitly not trash. Move on.
			return nil
		}

		// If the blob's mtime is at or after the createdBefore line, we can't safely delete it;
		// it might not be trash. If it is, we can expect to get it next time.
		//
		// See the comment above the WalkSatellitePiecesToTrash() function for a discussion on the correctness
		// of using ModTime in place of the more precise CreationTime.
		mTime, err := access.ModTime(ctx)
		if err != nil {
			if os.IsNotExist(err) {
				// piece was deleted while we were scanning.
				return nil
			}

			piecesSkipped++
			fw.log.Warn("failed to determine mtime of blob", zap.Error(err))
			// but continue iterating.
			return nil
		}
		if !mTime.Before(createdBefore) {
			return nil
		}

		if trashFunc != nil {
			if err := trashFunc(pieceID); err != nil {
				return err
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		return nil
	})

	return piecesCount, piecesSkipped, errFileWalker.Wrap(err)
}

// WalkCleanupTrash looks at all trash per-day directories owned by the given satellite and
// recursively deletes any of them that correspond to a time before the given dateBefore.
//
// This method returns the number of blobs deleted, the total count of bytes occupied by those
// deleted blobs, and the number of bytes which were freed by the deletion (including filesystem
// overhead).
func (fw *FileWalker) WalkCleanupTrash(ctx context.Context, satelliteID storj.NodeID, dateBefore time.Time) (bytesDeleted int64, keysDeleted []storj.PieceID, err error) {
	defer mon.Task()(&ctx)(&err)

	bytesDeleted, deletedKeysList, err := fw.blobs.EmptyTrash(ctx, satelliteID[:], dateBefore)
	keysDeleted = make([]storj.PieceID, 0, len(deletedKeysList))
	for _, dk := range deletedKeysList {
		pieceID, parseErr := storj.PieceIDFromBytes(dk)
		if parseErr != nil {
			fw.log.Error("stored blob has invalid pieceID", zap.ByteString("deletedKey", dk), zap.Error(parseErr))
			continue
		}
		keysDeleted = append(keysDeleted, pieceID)
	}
	return bytesDeleted, keysDeleted, err
}

func getKeyPrefix(blobRef blobstore.BlobRef) string {
	return filestore.PathEncoding.EncodeToString(blobRef.Key)[:2]
}
