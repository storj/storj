// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"

	"storj.io/common/storj"
	"storj.io/storj/shared/lrucache"
	"storj.io/storj/storagenode/blobstore/filestore"
)

// PathEncoding is used to encode satellite IDs for the piece expiration store.
var PathEncoding = filestore.PathEncoding

// PieceExpirationFileNameFormat is the format of filenames used by the piece
// expiration store, as interpreted by time.(*Time).Format.
const PieceExpirationFileNameFormat = "2006-01-02_15.dat"

type hourKey struct {
	satelliteID storj.NodeID
	hour        time.Time
}

func makeHourKey(satelliteID storj.NodeID, hour time.Time) hourKey {
	// UTC() should ensure that all time values have the same location information,
	// and Truncate() should remove any monotonic clock reading, making the time
	// object suitable for use as a map key.
	return hourKey{satelliteID: satelliteID, hour: hour.UTC().Truncate(time.Hour)}
}

type hourFile struct {
	w      *os.File
	buf    *bufio.Writer
	err    error
	closed bool
	mu     sync.Mutex // to be held while doing writes to the file
}

// ErrPieceExpiration represents errors from the piece expiration store.
var ErrPieceExpiration = errs.Class("pieceexpirationstore")

// PieceExpirationStore tracks piece expiration times by storing piece IDs in
// per-satellite flat files named by the hour they expire.
type PieceExpirationStore struct {
	log        *zap.Logger
	dataDir    string
	handlePool *lrucache.HandlePool[hourKey, *hourFile]
	tickerDone chan bool

	maxBufferTime         time.Duration
	concurrentFileHandles int
}

// PieceExpirationConfig contains configuration for the piece expiration store.
type PieceExpirationConfig struct {
	// DataDir is the directory where piece expiration data is stored.
	DataDir string
	// ConcurrentFileHandles is the number of concurrent file handles to use.
	// If more than this number are requested, the least recently used will
	// be closed and evicted.
	ConcurrentFileHandles int
	// MaxBufferTime is the maximum amount of time before piece expiration
	// data is flushed, regardless of how full the buffer is.
	MaxBufferTime time.Duration
}

// NewPieceExpirationStore creates a new piece expiration store.
func NewPieceExpirationStore(log *zap.Logger, config PieceExpirationConfig) (*PieceExpirationStore, error) {
	err := os.MkdirAll(config.DataDir, 0755)
	if err != nil {
		return nil, ErrPieceExpiration.Wrap(err)
	}
	peStore := &PieceExpirationStore{
		log:                   log,
		dataDir:               config.DataDir,
		tickerDone:            make(chan bool),
		maxBufferTime:         config.MaxBufferTime,
		concurrentFileHandles: config.ConcurrentFileHandles,
	}
	peStore.handlePool = lrucache.NewHandlePool[hourKey, *hourFile](config.ConcurrentFileHandles, peStore.openHour, peStore.closeHour)
	go peStore.flushOnTicks()
	return peStore, nil
}

// Close closes the piece expiration store, and all underlying file handles.
func (peStore *PieceExpirationStore) Close() error {
	close(peStore.tickerDone)
	peStore.handlePool.CloseAll()
	return nil
}

func (peStore *PieceExpirationStore) fileForKey(key hourKey) string {
	satelliteDir := PathEncoding.EncodeToString(key.satelliteID[:])
	return filepath.Join(peStore.dataDir, satelliteDir, key.hour.Format(PieceExpirationFileNameFormat))
}

func (peStore *PieceExpirationStore) flushOnTicks() {
	if peStore.maxBufferTime == 0 {
		return
	}
	flushTicker := time.NewTicker(peStore.maxBufferTime)
	for {
		select {
		case <-flushTicker.C:
			peStore.handlePool.ForEach(func(key hourKey, value *hourFile) {
				if err := value.flush(); err != nil {
					peStore.log.Error("failed to flush piece expiration data",
						zap.Stringer("satelliteID", key.satelliteID),
						zap.Time("hour", key.hour),
						zap.String("filename", value.w.Name()),
						zap.Error(err))
				}
			})
		case <-peStore.tickerDone:
			return
		}
	}
}

var monGetExpired = mon.Task()

// GetExpired gets piece IDs that expire or have expired before the given time.
// It returns the lastExpiration time, which can be used to delete all expirations
// up to that time.
func (peStore *PieceExpirationStore) GetExpired(ctx context.Context, now time.Time, opts ExpirationOptions) (infos []*ExpiredInfoRecords, err error) {
	defer monGetExpired(&ctx)(&err)

	satellites, err := peStore.getSatellitesWithExpirations(ctx)
	if err != nil {
		return nil, ErrPieceExpiration.Wrap(err)
	}
	var errList errs.Group
	for _, satelliteID := range satellites {
		satelliteInfos, err := peStore.GetExpiredForSatellite(ctx, satelliteID, now, opts)
		if err != nil {
			errList.Add(ErrPieceExpiration.Wrap(err))
		}
		infos = append(infos, satelliteInfos)
	}
	return infos, errList.Err()
}

var monDeleteExpirations = mon.Task()

// DeleteExpirations deletes information about piece expirations before the
// given time.
func (peStore *PieceExpirationStore) DeleteExpirations(ctx context.Context, expiresAt time.Time) (err error) {
	defer monDeleteExpirations(&ctx)(&err)

	errList := peStore.deleteExpirationsFromAllSatellites(ctx, expiresAt, DefaultExpirationOptions())
	return errList.Err()
}

var monDeleteExpirationsFromAllSatellites = mon.Task()

func (peStore *PieceExpirationStore) deleteExpirationsFromAllSatellites(ctx context.Context, expiresAt time.Time, opts ExpirationOptions) (errList errs.Group) {
	defer monDeleteExpirationsFromAllSatellites(&ctx)(nil)

	satellites, err := peStore.getSatellitesWithExpirations(ctx)
	if err != nil {
		errList.Add(ErrPieceExpiration.Wrap(err))
		return errList
	}
	for _, satelliteID := range satellites {
		err := peStore.DeleteExpirationsForSatellite(ctx, satelliteID, expiresAt, opts)
		errList.Add(ErrPieceExpiration.Wrap(err))
	}
	return errList
}

var monGetSatellitesWithExpirations = mon.Task()

func (peStore *PieceExpirationStore) getSatellitesWithExpirations(ctx context.Context) (satellites []storj.NodeID, err error) {
	defer monGetSatellitesWithExpirations(&ctx)(&err)

	dirfd, err := os.Open(peStore.dataDir)
	if err != nil {
		return nil, ErrPieceExpiration.Wrap(err)
	}
	defer func() { _ = dirfd.Close() }()

	names, err := dirfd.Readdirnames(-1)
	if err != nil {
		return nil, ErrPieceExpiration.Wrap(err)
	}

	for _, satelliteDir := range names {
		satelliteIDBytes, err := PathEncoding.DecodeString(satelliteDir)
		if err != nil {
			// not a satellite directory
			continue
		}
		satelliteID, err := storj.NodeIDFromBytes(satelliteIDBytes)
		if err != nil {
			// not a satellite directory either
			continue
		}
		satellites = append(satellites, satelliteID)
	}
	return satellites, nil
}

var monGetExpiredForSatellite = mon.Task()

// GetExpiredFiles returns the expired files which can be deleted.
func (peStore *PieceExpirationStore) GetExpiredFiles(ctx context.Context, satellite storj.NodeID, before time.Time) ([]string, error) {
	before = before.UTC().Truncate(time.Hour)
	satelliteDir := PathEncoding.EncodeToString(satellite[:])
	peDir := filepath.Join(peStore.dataDir, satelliteDir)
	dirfd, err := os.Open(peDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			peStore.log.Warn("piece_expierations directory doesn't exist", zap.String("file", peDir))
			return nil, nil
		}
		return nil, ErrPieceExpiration.Wrap(err)
	}
	defer func() { _ = dirfd.Close() }()
	names, err := dirfd.Readdirnames(-1)
	if err != nil {
		return nil, ErrPieceExpiration.Wrap(err)
	}
	slices.Sort(names)
	slices.Reverse(names)

	var filtered []string
	for _, name := range names {
		hour, err := time.ParseInLocation(PieceExpirationFileNameFormat, name, time.UTC)
		if err != nil {
			peStore.log.Warn("not a piece expiration file", zap.String("filename", name))
			// not a piece expiration file
			continue
		}
		// the file covers the period of one hour, so it has elapsed only if the whole hour has passed
		if hour.Before(before) {
			filtered = append(filtered, filepath.Join(peDir, name))
		}
	}
	return filtered, nil
}

// GetExpiredFromFile read one piece expiration file and calls callback for each piece.
func GetExpiredFromFile(ctx context.Context, filename string, callback func(id storj.PieceID, size uint64)) error {
	readF, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer func() { _ = readF.Close() }()

	var pieceID storj.PieceID
	var sizeBuf [8]byte
	buf := bufio.NewReader(readF)
	for {
		if err := ctx.Err(); err != nil {
			return ctx.Err()
		}
		_, err := io.ReadFull(buf, pieceID[:])
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return ErrPieceExpiration.New("reading piece expiration file %s: %w", filename, err)
		}
		_, err = io.ReadFull(buf, sizeBuf[:])
		// shouldn't happen, but not a big deal, as we supposed to process all the pieces anyway (except last one without size)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return ErrPieceExpiration.New("reading piece expiration file %s: %w", filename, err)
		}
		size := binary.BigEndian.Uint64(sizeBuf[:])
		callback(pieceID, size)
	}
}

// DeleteExpiredFile deletes the piece expiration file.
func DeleteExpiredFile(ctx context.Context, file string) error {
	return os.Remove(file)
}

// GetExpiredForSatellite gets piece IDs that expire or have expired before the
// given time for a specific satellite.
func (peStore *PieceExpirationStore) GetExpiredForSatellite(ctx context.Context, satellite storj.NodeID, now time.Time, opts ExpirationOptions) (infos *ExpiredInfoRecords, err error) {
	defer monGetExpiredForSatellite(&ctx)(&err)

	elapsed, err := peStore.getElapsedHoursWithExpirations(ctx, satellite, now, opts)
	if err != nil {
		return nil, ErrPieceExpiration.Wrap(err)
	}

	// flush all open applicable files
	var errList errs.Group
	for _, elapsedHour := range elapsed {
		hourKey := makeHourKey(satellite, elapsedHour)
		hourFile, release, ok := peStore.handlePool.Peek(hourKey)
		if !ok {
			continue
		}
		func() {
			defer release()
			err := hourFile.flush()
			if err != nil {
				peStore.log.Error("failed to flush piece expiration data",
					zap.Stringer("satelliteID", satellite),
					zap.Time("hour", elapsedHour),
					zap.String("filename", hourFile.w.Name()),
					zap.Error(err))
				// generally don't return errors when flushing, because it's
				// not directly relevant to the caller. a small amount of data
				// loss in expiration times is acceptable; pieces simply won't
				// be garbage collected as quickly as they could be.
				//
				// We will still continue to try and read the piece expiration
				// data from the file.
			}
		}()
	}

	// stat all files we're about to read, to get a reasonable estimate of the
	// total size we'll need
	var totalRecords int
	for _, elapsedHour := range elapsed {
		hourKey := makeHourKey(satellite, elapsedHour)
		filename := peStore.fileForKey(hourKey)
		stat, err := os.Stat(filename)
		if err != nil {
			// If this error is important, it will arise again when we try to
			// open or read the file below. For now, at worst, it is going to
			// leave us with a short count and possibly make us do another
			// allocation.
			continue
		}
		totalRecords += int(stat.Size() / int64(expirationRecordSize))
	}

	// and just in case the count is off by a little bit, add a fudge factor
	// because we really don't want to reallocate a large slice.
	infos = NewExpiredInfoRecords(satellite, false, totalRecords+1000)

	// open and read all applicable files (whether they are open in the pool or not)
	for _, elapsedHour := range elapsed {
		hourKey := makeHourKey(satellite, elapsedHour)
		filename := peStore.fileForKey(hourKey)
		readF, err := os.Open(filename)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			errList.Add(err)
			continue
		}
		func() {
			defer func() { _ = readF.Close() }()
			var pieceID storj.PieceID
			var timeBuf [8]byte
			buf := bufio.NewReader(readF)
			for {
				if err := ctx.Err(); err != nil {
					break
				}
				_, err := io.ReadFull(buf, pieceID[:])
				if err != nil {
					if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
						return
					}
					errList.Add(ErrPieceExpiration.New("reading piece expiration file %s: %w", filename, err))
					return
				}
				_, err = io.ReadFull(buf, timeBuf[:])
				if err != nil {
					if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
						return
					}
					errList.Add(ErrPieceExpiration.New("reading piece expiration file %s: %w", filename, err))
					return
				}
				infos.Append(pieceID, int64(binary.BigEndian.Uint64(timeBuf[:])))
			}
		}()
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}

	return infos, errList.Err()
}

var monDeleteExpirationsForSatellite = mon.Task()

// DeleteExpirationsForSatellite deletes information about piece expirations
// before the given time for a specific satellite.
func (peStore *PieceExpirationStore) DeleteExpirationsForSatellite(ctx context.Context, satellite storj.NodeID, now time.Time, opts ExpirationOptions) (err error) {
	defer monDeleteExpirationsForSatellite(&ctx)(&err)

	elapsed, err := peStore.getElapsedHoursWithExpirations(ctx, satellite, now, opts)
	if err != nil {
		return ErrPieceExpiration.Wrap(err)
	}

	var errList errs.Group
	for _, elapsedHour := range elapsed {
		hourKey := makeHourKey(satellite, elapsedHour)
		filename := peStore.fileForKey(hourKey)

		hourFile, ok := peStore.handlePool.Delete(hourKey)
		if ok {
			// the file must be closed before we try to delete it, for proper
			// operation on Windows
			peStore.closeHour(hourKey, hourFile)
		}
		err := os.Remove(filename)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			errList.Add(err)
		}
	}
	return errList.Err()
}

// DeleteExpirationsBatch removes expiration records for pieces that have expired before the given time.
func (peStore *PieceExpirationStore) DeleteExpirationsBatch(ctx context.Context, now time.Time, opts ExpirationOptions) error {
	errList := peStore.deleteExpirationsFromAllSatellites(ctx, now, opts)
	return errList.Err()
}

var monGetElapsedHoursWithExpirations = mon.Task()

func (peStore *PieceExpirationStore) getElapsedHoursWithExpirations(ctx context.Context, satellite storj.NodeID, now time.Time, opts ExpirationOptions) (elapsed []time.Time, err error) {
	defer monGetElapsedHoursWithExpirations(&ctx)(&err)

	satelliteDir := PathEncoding.EncodeToString(satellite[:])
	dirfd, err := os.Open(filepath.Join(peStore.dataDir, satelliteDir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, ErrPieceExpiration.Wrap(err)
	}
	defer func() { _ = dirfd.Close() }()
	names, err := dirfd.Readdirnames(-1)
	if err != nil {
		return nil, ErrPieceExpiration.Wrap(err)
	}
	slices.Sort(names)
	if opts.ReverseOrder {
		slices.Reverse(names)
	}
	for _, name := range names {
		hour, err := time.ParseInLocation(PieceExpirationFileNameFormat, name, time.UTC)
		if err != nil {
			// not a piece expiration file
			peStore.log.Warn("unexpected file in piece expiration directory", zap.String("filename", name))
			continue
		}
		// the file covers the period of one hour, so it has elapsed only if the whole hour has passed
		if hour.Add(time.Hour).Compare(now) <= 0 {
			elapsed = append(elapsed, hour)
		}

		// if we have a limit, we can stop once we have enough elapsed hours
		if opts.Limits.FlatFileLimit > 0 && len(elapsed) >= opts.Limits.FlatFileLimit {
			break
		}
	}
	return elapsed, nil
}

var monSetExpiration = mon.Task()

// SetExpiration sets an expiration time for the given piece ID on the given satellite.
func (peStore *PieceExpirationStore) SetExpiration(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID, expiresAt time.Time, pieceSize int64) (err error) {
	defer monSetExpiration(&ctx)(&err)

	expirationHour := expiresAt.Truncate(time.Hour)
	hourFile, release := peStore.handlePool.Get(makeHourKey(satellite, expirationHour))
	defer release()
	return hourFile.appendEntry(ctx, pieceID, pieceSize)
}

func (peStore *PieceExpirationStore) openHour(key hourKey) *hourFile {
	hourFile := &hourFile{}

	fileName := peStore.fileForKey(key)

	err := os.MkdirAll(filepath.Dir(fileName), 0o755)
	if err != nil {
		err = ErrPieceExpiration.Wrap(err)
		peStore.log.Error("failed to create piece expiration directory",
			zap.Stringer("satelliteID", key.satelliteID),
			zap.Time("hour", key.hour),
			zap.String("dirname", filepath.Dir(fileName)),
			zap.Error(err))
		hourFile.err = err
		// this is an unfortunate case, but the best we can do is to save the
		// error until this record is evicted from the handlePool.
		return hourFile
	}

	w, err := openHourFile(fileName)
	if err != nil {
		err = ErrPieceExpiration.Wrap(err)
		peStore.log.Error("failed to open piece expiration file",
			zap.Stringer("satelliteID", key.satelliteID),
			zap.Time("hour", key.hour),
			zap.String("filename", fileName),
			zap.Error(err))
		hourFile.err = err
		// to make things simple, we will just wait until this record is evicted
		// from the handlePool before we try to open it again.
		return hourFile
	}
	// it's possible that we failed to make a write previously in appendEntry,
	// leaving this file in an inconsistent state. we need to truncate it at
	// the right multiple of the record size to minimize the chance of
	// corruption.
	pos, err := w.Seek(0, io.SeekEnd)
	if err != nil {
		hourFile.err = ErrPieceExpiration.Wrap(err)
		_ = w.Close()
		return hourFile
	}
	const recordSize = int64(len(storj.PieceID{})) + 8
	if pos%recordSize != 0 {
		peStore.log.Warn("truncating piece expiration file to a multiple of record size",
			zap.Int64("openedAtSize", pos),
			zap.Int64("recordSize", recordSize),
			zap.Stringer("satelliteID", key.satelliteID),
			zap.Time("hour", key.hour),
			zap.String("filename", w.Name()))
		_, errSeek := w.Seek(pos-(pos%recordSize), io.SeekStart)
		errTrunc := w.Truncate(pos - (pos % recordSize))
		if errSeek != nil || errTrunc != nil {
			if errSeek != nil {
				err = ErrPieceExpiration.Wrap(errSeek)
			} else {
				err = ErrPieceExpiration.Wrap(errTrunc)
			}
			peStore.log.Error("failed to truncate piece expiration file",
				zap.Int64("openedAtSize", pos),
				zap.Int64("recordSize", recordSize),
				zap.Stringer("satelliteID", key.satelliteID),
				zap.Time("hour", key.hour),
				zap.String("filename", w.Name()),
				zap.Error(err))
			hourFile.err = err
			_ = w.Close()
			return hourFile
		}
	}
	hourFile.w = w
	hourFile.buf = bufio.NewWriter(w)
	return hourFile
}

func (peStore *PieceExpirationStore) closeHour(key hourKey, hourFile *hourFile) {
	hourFile.mu.Lock()
	defer hourFile.mu.Unlock()

	// if hourfile.w is nil, the file may have failed to open, or it may have
	// encountered an error during write. In either case, it's not open now.
	if hourFile.w != nil {
		if hourFile.buf != nil {
			err := hourFile.buf.Flush()
			if err != nil {
				// this isn't fatal; piece expiration data may be lost,
				// but those pieces will still be garbage collected.
				peStore.log.Error("failed to flush writes to piece expiration list",
					zap.Stringer("satelliteID", key.satelliteID),
					zap.Time("hour", key.hour),
					zap.String("filename", hourFile.w.Name()),
					zap.Error(err),
				)
			}
		}

		err := hourFile.w.Close()
		if err != nil {
			// also not fatal
			peStore.log.Error("failed to complete writes to piece expiration list",
				zap.Stringer("satelliteID", key.satelliteID),
				zap.Time("hour", key.hour),
				zap.String("filename", hourFile.w.Name()),
				zap.Error(err),
			)
		}
	}

	hourFile.w = nil
	hourFile.buf = nil
	hourFile.closed = true
}

func (hourFile *hourFile) flush() error {
	hourFile.mu.Lock()
	defer hourFile.mu.Unlock()

	if hourFile.buf == nil {
		// not opened; nothing to flush
		return nil
	}
	return hourFile.buf.Flush()
}

var monAppendEntry = mon.Task()

func (hourFile *hourFile) appendEntry(ctx context.Context, pieceID storj.PieceID, pieceSize int64) (err error) {
	defer monAppendEntry(&ctx)(&err)

	var timeBuf [8]byte
	binary.BigEndian.PutUint64(timeBuf[:], uint64(pieceSize))

	hourFile.mu.Lock()
	defer hourFile.mu.Unlock()

	if hourFile.closed {
		return ErrPieceExpiration.New("hour file is closed")
	}
	if hourFile.buf == nil {
		return ErrPieceExpiration.New("could not append to hour file: %w", hourFile.err)
	}
	_, err = hourFile.buf.Write(pieceID[:])
	if err == nil {
		_, err = hourFile.buf.Write(timeBuf[:])
	}
	if err != nil {
		// a failed write in either case means we don't know how much from
		// previous writes actually made it through to the file. we can't
		// recover from this without a lot of extra complexity, so we will close
		// the file and let the error condition sit until this record is evicted
		// from the handlePool.
		err = ErrPieceExpiration.Wrap(err)
		_ = hourFile.w.Close()
		hourFile.buf = nil
		hourFile.w = nil
		hourFile.closed = true
		hourFile.err = err
		return err
	}
	return ErrPieceExpiration.Wrap(err)
}

var _ PieceExpirationDB = (*PieceExpirationStore)(nil)
