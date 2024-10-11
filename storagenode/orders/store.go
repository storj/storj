// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/private/date"
	"storj.io/storj/storagenode/orders/ordersfile"
)

var nameBatchSize = 1024

// activeWindow represents a window with active operations waiting to finish to enqueue
// their orders.
type activeWindow struct {
	satelliteID storj.NodeID
	timestamp   int64
}

// FileStore implements the orders.Store interface by appending orders to flat files.
type FileStore struct {
	log *zap.Logger

	ordersDir  string
	unsentDir  string
	archiveDir string

	// always acquire the activeMu after the unsentMu to avoid deadlocks. if someone acquires
	// activeMu before unsentMu, then you can be in a situation where two goroutines are waiting for
	// each other forever. similarly, always acquire archiveMu before activeMu and unsentMu to avoid
	// deadlocks.
	//
	// in summary, the priority is: archiveMu -> unsentMu -> activeMu.

	// mutex for the active map
	activeMu sync.Mutex
	active   map[activeWindow]int

	// mutex for unsent directory
	unsentMu          sync.Mutex
	unsentOrdersFiles map[string]ordersfile.Writable
	// mutex for archive directory
	archiveMu sync.Mutex

	// how long after OrderLimit creation date are OrderLimits no longer accepted (piecestore Config)
	orderLimitGracePeriod time.Duration
}

// NewFileStore creates a new orders file store, and the directories necessary for its use.
func NewFileStore(log *zap.Logger, ordersDir string, orderLimitGracePeriod time.Duration) (*FileStore, error) {
	fs := &FileStore{
		log:                   log,
		ordersDir:             ordersDir,
		unsentDir:             filepath.Join(ordersDir, "unsent"),
		archiveDir:            filepath.Join(ordersDir, "archive"),
		active:                make(map[activeWindow]int),
		unsentOrdersFiles:     make(map[string]ordersfile.Writable),
		orderLimitGracePeriod: orderLimitGracePeriod,
	}

	err := fs.ensureDirectories()
	if err != nil {
		return nil, err
	}

	return fs, nil
}

// Close closes the file store.
func (store *FileStore) Close() (err error) {
	store.unsentMu.Lock()
	defer store.unsentMu.Unlock()
	store.activeMu.Lock()
	defer store.activeMu.Unlock()

	for fileName, file := range store.unsentOrdersFiles {
		err = errs.Combine(err, OrderError.Wrap(file.Close()))
		delete(store.unsentOrdersFiles, fileName)
	}

	return err
}

// BeginEnqueue returns a function that can be called to enqueue the passed in Info. If the Info
// is too old to be enqueued, then an error is returned.
func (store *FileStore) BeginEnqueue(satelliteID storj.NodeID, createdAt time.Time) (commit func(*ordersfile.Info) error, err error) {
	store.unsentMu.Lock()
	defer store.unsentMu.Unlock()
	store.activeMu.Lock()
	defer store.activeMu.Unlock()

	// if the order is older than the grace period, reject it. We don't check against what
	// window the order would go into to make the calculation more predictable: if the order
	// is older than the grace limit, it will not be accepted.
	if time.Since(createdAt) > store.orderLimitGracePeriod {
		return nil, OrderError.New("grace period passed for order limit")
	}

	// record that there is an operation in flight for this window
	store.enqueueStartedLocked(satelliteID, createdAt)

	return func(info *ordersfile.Info) error {
		// always acquire the activeMu after the unsentMu to avoid deadlocks
		store.unsentMu.Lock()
		defer store.unsentMu.Unlock()
		store.activeMu.Lock()
		defer store.activeMu.Unlock()

		// always remove the in flight operation
		defer store.enqueueFinishedLocked(satelliteID, createdAt)

		// caller wants to abort; free file for sending and return with no error
		if info == nil {
			return nil
		}

		// check that the info matches what the enqueue was begun with
		if info.Limit.SatelliteId != satelliteID || !info.Limit.OrderCreation.Equal(createdAt) {
			return OrderError.New("invalid info passed in to enqueue commit")
		}

		// write out the data
		of, err := store.getWritableUnsent(store.unsentDir, info.Limit.SatelliteId, info.Limit.OrderCreation)
		if err != nil {
			return OrderError.Wrap(err)
		}

		err = of.Append(info)
		if err != nil {
			return OrderError.Wrap(err)
		}

		return nil
	}, nil
}

// getWritableUnsent retrieves an already open "unsent orders" file, or otherwise opens a new one.
// Caller must guarantee to obtain the unsent lock before calling this method.
func (store *FileStore) getWritableUnsent(unsentDir string, satelliteID storj.NodeID, creationTime time.Time) (of ordersfile.Writable, err error) {
	fileName := ordersfile.UnsentFileName(satelliteID, creationTime, ordersfile.V1)
	file, ok := store.unsentOrdersFiles[fileName]
	if !ok {
		filePath := filepath.Join(unsentDir, fileName)
		file, err = ordersfile.OpenWritableV1(filePath, satelliteID, creationTime)
		if err != nil {
			return nil, OrderError.Wrap(err)
		}
		store.unsentOrdersFiles[fileName] = file
	}

	return file, nil
}

// enqueueStartedLocked records that there is an order pending to be written to the window.
func (store *FileStore) enqueueStartedLocked(satelliteID storj.NodeID, createdAt time.Time) {
	store.active[activeWindow{
		satelliteID: satelliteID,
		timestamp:   date.TruncateToHourInNano(createdAt),
	}]++
}

// enqueueFinishedLocked informs that there is no longer an order pending to be written to the
// window.
func (store *FileStore) enqueueFinishedLocked(satelliteID storj.NodeID, createdAt time.Time) {
	window := activeWindow{
		satelliteID: satelliteID,
		timestamp:   date.TruncateToHourInNano(createdAt),
	}

	store.active[window]--
	if store.active[window] <= 0 {
		delete(store.active, window)
	}
}

// hasActiveEnqueue returns true if there are active orders enqueued for the requested window.
func (store *FileStore) hasActiveEnqueue(satelliteID storj.NodeID, createdAt time.Time) bool {
	store.activeMu.Lock()
	defer store.activeMu.Unlock()

	return store.active[activeWindow{
		satelliteID: satelliteID,
		timestamp:   date.TruncateToHourInNano(createdAt),
	}] > 0
}

// Enqueue inserts order to be sent at the end of the unsent file for a particular creation hour.
// It ensures the order is not being queued after the order limit grace period.
func (store *FileStore) Enqueue(info *ordersfile.Info) (err error) {
	commit, err := store.BeginEnqueue(info.Limit.SatelliteId, info.Limit.OrderCreation)
	if err != nil {
		return err
	}
	return commit(info)
}

// UnsentInfo is a struct containing a window of orders for a satellite and order creation hour.
type UnsentInfo struct {
	CreatedAtHour time.Time
	Version       ordersfile.Version
	InfoList      []*ordersfile.Info
}

// ListUnsentBySatellite returns one window of orders that haven't been sent yet, grouped by satellite.
// It only reads files where the order limit grace period has passed, meaning no new orders will be appended.
// There is a separate window for each created at hour, so if a satellite has 2 windows, `ListUnsentBySatellite`
// needs to be called twice, with calls to `Archive` in between each call, to see all unsent orders.
func (store *FileStore) ListUnsentBySatellite(ctx context.Context, now time.Time) (infoMap map[storj.NodeID]UnsentInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	// ensure no one modifies the archive directory while listing. this implicitly protects the
	// unsent directory from losing files because to add to the archive directory it must come from
	// the unsent directory. we don't need a long term lock on the unsent directory this way which
	// allows adding orders to proceed.
	store.archiveMu.Lock()
	defer store.archiveMu.Unlock()

	var errList errs.Group
	infoMap = make(map[storj.NodeID]UnsentInfo)

	errList.Add(walkFilenamesInPath(store.unsentDir, func(name string) error {
		fileInfo, err := ordersfile.GetUnsentInfo(name)
		if err != nil {
			errList.Add(OrderError.Wrap(err))
			return nil
		}

		// if we already have orders for this satellite, ignore the file
		if _, ok := infoMap[fileInfo.SatelliteID]; ok {
			return nil
		}

		// if orders can still be added to file, ignore it. We add an hour because that's
		// the newest order that could be added to that window.
		if now.Sub(fileInfo.CreatedAtHour.Add(time.Hour)) <= store.orderLimitGracePeriod {
			return nil
		}

		// if there are still active orders for the time, ignore it.
		if store.hasActiveEnqueue(fileInfo.SatelliteID, fileInfo.CreatedAtHour) {
			return nil
		}

		// read in the unsent orders into memory and store it for the satellite
		newUnsentInfo, err := store.getUnsentInfoFromUnsentFile(store.unsentDir, name, fileInfo)
		if err != nil {
			errList.Add(OrderError.Wrap(err))
			return nil
		}
		infoMap[fileInfo.SatelliteID] = newUnsentInfo

		return nil
	}))

	return infoMap, errList.Err()
}

func (store *FileStore) getUnsentInfoFromUnsentFile(dir, fileName string, fileInfo *ordersfile.UnsentInfo) (UnsentInfo, error) {
	// close writable file and delete from map since we are done with it. we drop the mutex before
	// doing file operations to avoid holding unsentMu because that is used to add new orders.
	// there's no need to worry about keeping the file inside of the map if the Close call errors,
	// because the file descriptor will be invalidated either way.

	store.unsentMu.Lock()
	file, ok := store.unsentOrdersFiles[fileName]
	delete(store.unsentOrdersFiles, fileName)
	store.unsentMu.Unlock()

	if ok {
		if err := file.Close(); err != nil {
			return UnsentInfo{}, OrderError.Wrap(err)
		}
	}

	newUnsentInfo := UnsentInfo{
		CreatedAtHour: fileInfo.CreatedAtHour,
		Version:       fileInfo.Version,
	}

	of, err := ordersfile.OpenReadable(filepath.Join(dir, fileName), fileInfo.Version)
	if err != nil {
		return UnsentInfo{}, OrderError.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, OrderError.Wrap(of.Close()))
	}()

	for {
		// if at any point we see an unexpected EOF error, return what orders we could read successfully with no error
		// this behavior ensures that we will attempt to archive corrupted files instead of continually failing to read them
		newInfo, err := of.ReadOne()
		if err != nil {
			if errs.Is(err, io.EOF) {
				break
			}
			// if last entry read is corrupt, attempt to read again
			if ordersfile.ErrEntryCorrupt.Has(err) {
				store.log.Warn("Corrupted order detected in orders file", zap.Error(err))
				mon.Meter("orders_unsent_file_corrupted").Mark64(1)
				// if the error is unexpected EOF, we want the metrics and logs, but there
				// is no use in trying to read from the file again.
				if errs.Is(err, io.ErrUnexpectedEOF) {
					break
				}
				continue
			}
			return UnsentInfo{}, err
		}

		newUnsentInfo.InfoList = append(newUnsentInfo.InfoList, newInfo)
	}

	return newUnsentInfo, err
}

// Archive moves a file from "unsent" to "archive".
func (store *FileStore) Archive(satelliteID storj.NodeID, unsentInfo UnsentInfo, archivedAt time.Time, status pb.SettlementWithWindowResponse_Status) error {
	store.archiveMu.Lock()
	defer store.archiveMu.Unlock()
	store.unsentMu.Lock()
	defer store.unsentMu.Unlock()

	return OrderError.Wrap(ordersfile.MoveUnsent(
		store.unsentDir,
		store.archiveDir,
		satelliteID,
		unsentInfo.CreatedAtHour,
		archivedAt,
		status,
		unsentInfo.Version,
	))
}

// ListArchived returns orders that have been sent.
func (store *FileStore) ListArchived() ([]*ArchivedInfo, error) {
	store.archiveMu.Lock()
	defer store.archiveMu.Unlock()

	var errList errs.Group
	var archivedList []*ArchivedInfo

	errList.Add(walkFilenamesInPath(store.archiveDir, func(name string) error {
		fileInfo, err := ordersfile.GetArchivedInfo(name)
		if err != nil {
			errList.Add(OrderError.Wrap(err))
			return nil
		}
		path := filepath.Join(store.archiveDir, name)
		archiveInfo, err := store.getArchiveInfosFromArchiveFile(path, fileInfo)
		if err != nil {
			errList.Add(OrderError.Wrap(err))
			return nil
		}

		archivedList = append(archivedList, archiveInfo...)
		return nil
	}))

	return archivedList, errList.Err()
}

func (store *FileStore) getArchiveInfosFromArchiveFile(path string, fileInfo *ordersfile.ArchivedInfo) ([]*ArchivedInfo, error) {
	of, err := ordersfile.OpenReadable(path, fileInfo.Version)
	if err != nil {
		return nil, OrderError.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, OrderError.Wrap(of.Close()))
	}()

	status := StatusUnsent
	switch fileInfo.StatusText {
	case pb.SettlementWithWindowResponse_ACCEPTED.String():
		status = StatusAccepted
	case pb.SettlementWithWindowResponse_REJECTED.String():
		status = StatusRejected
	}

	var archivedList []*ArchivedInfo

	for {
		info, err := of.ReadOne()
		if err != nil {
			if errs.Is(err, io.EOF) {
				break
			}
			// if last entry read is corrupt, attempt to read again
			if ordersfile.ErrEntryCorrupt.Has(err) {
				store.log.Warn("Corrupted order detected in orders file", zap.Error(err))
				mon.Meter("orders_archive_file_corrupted").Mark64(1)
				continue
			}
			return nil, err
		}

		newInfo := &ArchivedInfo{
			Limit:      info.Limit,
			Order:      info.Order,
			Status:     status,
			ArchivedAt: fileInfo.ArchivedAt,
		}
		archivedList = append(archivedList, newInfo)
	}

	return archivedList, nil
}

// CleanArchive deletes all entries archvied before the provided time.
func (store *FileStore) CleanArchive(deleteBefore time.Time) error {
	store.archiveMu.Lock()
	defer store.archiveMu.Unlock()

	// we want to delete everything older than ttl
	var errList errs.Group

	errList.Add(walkFilenamesInPath(store.archiveDir, func(name string) error {
		fileInfo, err := ordersfile.GetArchivedInfo(name)
		if err != nil {
			errList.Add(OrderError.Wrap(err))
			return nil
		}

		if fileInfo.ArchivedAt.Before(deleteBefore) {
			err = os.Remove(filepath.Join(store.archiveDir, name))
			if err != nil {
				errList.Add(OrderError.Wrap(err))
			}
		}
		return nil
	}))

	return errList.Err()
}

func walkFilenamesInPath(path string, cb func(name string) error) error {
	root, err := os.Open(path)
	if err != nil {
		return OrderError.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, OrderError.Wrap(root.Close()))
	}()

	for {
		entries, err := root.Readdirnames(nameBatchSize)
		if err != nil {
			if err == io.EOF {
				break
			}
			return OrderError.Wrap(err)
		}
		for _, name := range entries {
			if err := cb(name); err != nil {
				return err
			}
		}

	}
	return nil
}

// ensureDirectories checks for the existence of the unsent and archived directories, and creates them if they do not exist.
func (store *FileStore) ensureDirectories() error {
	if _, err := os.Stat(store.unsentDir); os.IsNotExist(err) {
		err = os.MkdirAll(store.unsentDir, 0700)
		if err != nil {
			return OrderError.Wrap(err)
		}
	}
	if _, err := os.Stat(store.archiveDir); os.IsNotExist(err) {
		err = os.MkdirAll(store.archiveDir, 0700)
		if err != nil {
			return OrderError.Wrap(err)
		}
	}
	return nil
}
