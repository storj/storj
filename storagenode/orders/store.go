// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/private/date"
)

const (
	unsentFilePrefix  = "unsent-orders-"
	archiveFilePrefix = "archived-orders-"
)

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
	// activeMu before unsentMu, then you can be in a situation where two goroutines are
	// waiting for each other forever.

	// mutex for the active map
	activeMu sync.Mutex
	active   map[activeWindow]int

	// mutex for unsent directory
	unsentMu sync.Mutex
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
		orderLimitGracePeriod: orderLimitGracePeriod,
	}

	err := fs.ensureDirectories()
	if err != nil {
		return nil, err
	}

	return fs, nil
}

// BeginEnqueue returns a function that can be called to enqueue the passed in Info. If the Info
// is too old to be enqueued, then an error is returned.
func (store *FileStore) BeginEnqueue(satelliteID storj.NodeID, createdAt time.Time) (commit func(*Info) error, err error) {
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

	return func(info *Info) error {
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
		f, err := store.getUnsentFile(info.Limit.SatelliteId, info.Limit.OrderCreation)
		if err != nil {
			return OrderError.Wrap(err)
		}
		defer func() {
			err = errs.Combine(err, OrderError.Wrap(f.Close()))
		}()

		err = writeLimitAndOrder(f, info.Limit, info.Order)
		if err != nil {
			return err
		}

		return nil
	}, nil
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
func (store *FileStore) Enqueue(info *Info) (err error) {
	commit, err := store.BeginEnqueue(info.Limit.SatelliteId, info.Limit.OrderCreation)
	if err != nil {
		return err
	}
	return commit(info)
}

// UnsentInfo is a struct containing a window of orders for a satellite and order creation hour.
type UnsentInfo struct {
	CreatedAtHour time.Time
	InfoList      []*Info
}

// ListUnsentBySatellite returns one window of orders that haven't been sent yet, grouped by satellite.
// It only reads files where the order limit grace period has passed, meaning no new orders will be appended.
// There is a separate window for each created at hour, so if a satellite has 2 windows, `ListUnsentBySatellite`
// needs to be called twice, with calls to `Archive` in between each call, to see all unsent orders.
func (store *FileStore) ListUnsentBySatellite(now time.Time) (infoMap map[storj.NodeID]UnsentInfo, err error) {
	// shouldn't be necessary, but acquire archiveMu to ensure we do not attempt to archive files during list
	store.archiveMu.Lock()
	defer store.archiveMu.Unlock()

	var errList error
	infoMap = make(map[storj.NodeID]UnsentInfo)

	err = filepath.Walk(store.unsentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			errList = errs.Combine(errList, OrderError.Wrap(err))
			return nil
		}
		if info.IsDir() {
			return nil
		}
		satelliteID, createdAtHour, err := getUnsentFileInfo(info.Name())
		if err != nil {
			return err
		}

		// if we already have orders for this satellite, ignore the file
		if _, ok := infoMap[satelliteID]; ok {
			return nil
		}

		// if orders can still be added to file, ignore it. We add an hour because that's
		// the newest order that could be added to that window.
		if now.Sub(createdAtHour.Add(time.Hour)) <= store.orderLimitGracePeriod {
			return nil
		}

		// if there are still active orders for the time, ignore it.
		if store.hasActiveEnqueue(satelliteID, createdAtHour) {
			return nil
		}

		newUnsentInfo := UnsentInfo{
			CreatedAtHour: createdAtHour,
		}

		f, err := os.Open(path)
		if err != nil {
			return OrderError.Wrap(err)
		}
		defer func() {
			err = errs.Combine(err, OrderError.Wrap(f.Close()))
		}()

		for {
			// if at any point we see an unexpected EOF error, return what orders we could read successfully with no error
			// this behavior ensures that we will attempt to archive corrupted files instead of continually failing to read them
			limit, order, err := readLimitAndOrder(f)
			if err != nil {
				if errs.Is(err, io.EOF) {
					break
				}
				if errs.Is(err, io.ErrUnexpectedEOF) {
					store.log.Warn("Unexpected EOF while reading unsent order file", zap.Error(err))
					mon.Meter("orders_unsent_file_corrupted").Mark64(1)
					break
				}
				return err
			}

			newInfo := &Info{
				Limit: limit,
				Order: order,
			}
			newUnsentInfo.InfoList = append(newUnsentInfo.InfoList, newInfo)
		}

		infoMap[satelliteID] = newUnsentInfo
		return nil
	})
	if err != nil {
		errList = errs.Combine(errList, err)
	}

	return infoMap, errList
}

// Archive moves a file from "unsent" to "archive". The filename/path changes from
// unsent/unsent-orders-<satelliteID>-<createdAtHour>
// to
// archive/archived-orders-<satelliteID>-<createdAtHour>-<archivedTime>-<ACCEPTED/REJECTED>.
func (store *FileStore) Archive(satelliteID storj.NodeID, createdAtHour, archivedAt time.Time, status pb.SettlementWithWindowResponse_Status) error {
	store.unsentMu.Lock()
	defer store.unsentMu.Unlock()
	store.archiveMu.Lock()
	defer store.archiveMu.Unlock()

	oldFileName := unsentFilePrefix + satelliteID.String() + "-" + getCreationHourString(createdAtHour)
	oldFilePath := filepath.Join(store.unsentDir, oldFileName)

	newFileName := fmt.Sprintf("%s%s-%s-%s-%s",
		archiveFilePrefix,
		satelliteID.String(),
		getCreationHourString(createdAtHour),
		strconv.FormatInt(archivedAt.UnixNano(), 10),
		pb.SettlementWithWindowResponse_Status_name[int32(status)],
	)
	newFilePath := filepath.Join(store.archiveDir, newFileName)

	return OrderError.Wrap(os.Rename(oldFilePath, newFilePath))
}

// ListArchived returns orders that have been sent.
func (store *FileStore) ListArchived() ([]*ArchivedInfo, error) {
	store.archiveMu.Lock()
	defer store.archiveMu.Unlock()

	var errList error
	archivedList := []*ArchivedInfo{}
	err := filepath.Walk(store.archiveDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			errList = errs.Combine(errList, OrderError.Wrap(err))
			return nil
		}
		if info.IsDir() {
			return nil
		}
		_, _, archivedAt, statusText, err := getArchivedFileInfo(info.Name())
		if err != nil {
			return err
		}

		status := StatusUnsent
		switch statusText {
		case pb.SettlementWithWindowResponse_ACCEPTED.String():
			status = StatusAccepted
		case pb.SettlementWithWindowResponse_REJECTED.String():
			status = StatusRejected
		}

		f, err := os.Open(path)
		if err != nil {
			return OrderError.Wrap(err)
		}
		defer func() {
			err = errs.Combine(err, OrderError.Wrap(f.Close()))
		}()

		for {
			limit, order, err := readLimitAndOrder(f)
			if err != nil {
				if errs.Is(err, io.EOF) {
					break
				}
				if errs.Is(err, io.ErrUnexpectedEOF) {
					store.log.Warn("Unexpected EOF while reading archived order file", zap.Error(err))
					mon.Meter("orders_archive_file_corrupted").Mark64(1)
					break
				}
				return err
			}

			newInfo := &ArchivedInfo{
				Limit:      limit,
				Order:      order,
				Status:     status,
				ArchivedAt: archivedAt,
			}
			archivedList = append(archivedList, newInfo)
		}
		return nil
	})
	if err != nil {
		errList = errs.Combine(errList, err)
	}

	return archivedList, errList
}

// CleanArchive deletes all entries archvied before the provided time.
func (store *FileStore) CleanArchive(deleteBefore time.Time) error {
	store.archiveMu.Lock()
	defer store.archiveMu.Unlock()

	// we want to delete everything older than ttl
	var errList error
	err := filepath.Walk(store.archiveDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			errList = errs.Combine(errList, OrderError.Wrap(err))
			return nil
		}
		if info.IsDir() {
			return nil
		}
		_, _, archivedAt, _, err := getArchivedFileInfo(info.Name())
		if err != nil {
			errList = errs.Combine(errList, err)
			return nil
		}
		if archivedAt.Before(deleteBefore) {
			return OrderError.Wrap(os.Remove(path))
		}
		return nil
	})
	return errs.Combine(errList, err)
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

// getUnsentFile creates or gets the order limit file for appending unsent orders to.
// There is a different file for each satellite and creation hour.
// It expects the caller to lock the store's mutex before calling, and to handle closing the returned file.
func (store *FileStore) getUnsentFile(satelliteID storj.NodeID, creationTime time.Time) (*os.File, error) {
	fileName := unsentFilePrefix + satelliteID.String() + "-" + getCreationHourString(creationTime)
	filePath := filepath.Join(store.unsentDir, fileName)
	// create file if not exists or append
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, OrderError.Wrap(err)
	}
	return f, nil
}

func getCreationHourString(t time.Time) string {
	creationHour := date.TruncateToHourInNano(t)
	timeStr := strconv.FormatInt(creationHour, 10)
	return timeStr
}

// getUnsentFileInfo gets the satellite ID and created hour from a filename.
// it expects the file name to be in the format "unsent-orders-<satelliteID>-<createdAtHour>".
func getUnsentFileInfo(name string) (satellite storj.NodeID, createdHour time.Time, err error) {
	if !strings.HasPrefix(name, unsentFilePrefix) {
		return storj.NodeID{}, time.Time{}, OrderError.New("Not a valid unsent order file name: %s", name)
	}
	// chop off prefix to get satellite ID and created hours
	infoStr := name[len(unsentFilePrefix):]
	infoSlice := strings.Split(infoStr, "-")
	if len(infoSlice) != 2 {
		return storj.NodeID{}, time.Time{}, OrderError.New("Not a valid unsent order file name: %s", name)
	}

	satelliteIDStr := infoSlice[0]
	satelliteID, err := storj.NodeIDFromString(satelliteIDStr)
	if err != nil {
		return storj.NodeID{}, time.Time{}, OrderError.New("Not a valid unsent order file name: %s", name)
	}

	timeStr := infoSlice[1]
	createdHourUnixNano, err := strconv.ParseInt(timeStr, 10, 64)
	if err != nil {
		return satelliteID, time.Time{}, OrderError.Wrap(err)
	}
	createdAtHour := time.Unix(0, createdHourUnixNano)

	return satelliteID, createdAtHour, nil
}

// getArchivedFileInfo gets the archived at time from an archive file name.
// it expects the file name to be in the format "archived-orders-<satelliteID>-<createdAtHour>-<archviedAtTime>-<status>".
func getArchivedFileInfo(name string) (satelliteID storj.NodeID, createdAtHour, archivedAt time.Time, status string, err error) {
	if !strings.HasPrefix(name, archiveFilePrefix) {
		return storj.NodeID{}, time.Time{}, time.Time{}, "", OrderError.New("Not a valid archived order file name: %s", name)
	}
	// chop off prefix to get satellite ID, created hour, archive time, and status
	infoStr := name[len(archiveFilePrefix):]
	infoSlice := strings.Split(infoStr, "-")
	if len(infoSlice) != 4 {
		return storj.NodeID{}, time.Time{}, time.Time{}, "", OrderError.New("Not a valid archived order file name: %s", name)
	}

	satelliteIDStr := infoSlice[0]
	satelliteID, err = storj.NodeIDFromString(satelliteIDStr)
	if err != nil {
		return storj.NodeID{}, time.Time{}, time.Time{}, "", OrderError.New("Not a valid archived order file name: %s", name)
	}

	createdAtStr := infoSlice[1]
	createdHourUnixNano, err := strconv.ParseInt(createdAtStr, 10, 64)
	if err != nil {
		return satelliteID, time.Time{}, time.Time{}, "", OrderError.New("Not a valid archived order file name: %s", name)
	}
	createdAtHour = time.Unix(0, createdHourUnixNano)

	archivedAtStr := infoSlice[2]
	archivedAtUnixNano, err := strconv.ParseInt(archivedAtStr, 10, 64)
	if err != nil {
		return satelliteID, createdAtHour, time.Time{}, "", OrderError.New("Not a valid archived order file name: %s", name)
	}
	archivedAt = time.Unix(0, archivedAtUnixNano)

	status = infoSlice[3]

	return satelliteID, createdAtHour, archivedAt, status, nil
}

// writeLimitAndOrder writes limit and order to the file as
// [limitSize][limitBytes][orderSize][orderBytes]
// It expects the caller to have locked the mutex.
func writeLimitAndOrder(f io.Writer, limit *pb.OrderLimit, order *pb.Order) error {
	toWrite := []byte{}

	limitSerialized, err := pb.Marshal(limit)
	if err != nil {
		return OrderError.Wrap(err)
	}
	orderSerialized, err := pb.Marshal(order)
	if err != nil {
		return OrderError.Wrap(err)
	}

	limitSizeBytes := [4]byte{}
	binary.LittleEndian.PutUint32(limitSizeBytes[:], uint32(len(limitSerialized)))

	orderSizeBytes := [4]byte{}
	binary.LittleEndian.PutUint32(orderSizeBytes[:], uint32(len(orderSerialized)))

	toWrite = append(toWrite, limitSizeBytes[:]...)
	toWrite = append(toWrite, limitSerialized...)
	toWrite = append(toWrite, orderSizeBytes[:]...)
	toWrite = append(toWrite, orderSerialized...)

	if _, err = f.Write(toWrite); err != nil {
		return OrderError.New("Error writing serialized order size: %w", err)
	}
	return nil
}

// readLimitAndOrder reads the next limit and order from the file and returns them.
func readLimitAndOrder(f io.Reader) (*pb.OrderLimit, *pb.Order, error) {
	sizeBytes := [4]byte{}
	_, err := io.ReadFull(f, sizeBytes[:])
	if err != nil {
		return nil, nil, OrderError.Wrap(err)
	}
	limitSize := binary.LittleEndian.Uint32(sizeBytes[:])
	limitSerialized := make([]byte, limitSize)
	_, err = io.ReadFull(f, limitSerialized)
	if err != nil {
		return nil, nil, OrderError.Wrap(err)
	}
	limit := &pb.OrderLimit{}
	err = pb.Unmarshal(limitSerialized, limit)
	if err != nil {
		return nil, nil, OrderError.Wrap(err)
	}

	_, err = io.ReadFull(f, sizeBytes[:])
	if err != nil {
		return nil, nil, OrderError.Wrap(err)
	}
	orderSize := binary.LittleEndian.Uint32(sizeBytes[:])
	orderSerialized := make([]byte, orderSize)
	_, err = io.ReadFull(f, orderSerialized)
	if err != nil {
		return nil, nil, OrderError.Wrap(err)
	}
	order := &pb.Order{}
	err = pb.Unmarshal(orderSerialized, order)
	if err != nil {
		return nil, nil, OrderError.Wrap(err)
	}

	return limit, order, nil
}
