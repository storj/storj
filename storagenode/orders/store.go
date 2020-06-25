// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/common/storj"
)

const (
	unsentFilePrefix  = "unsent-orders-"
	archiveFilePrefix = "archived-orders-"
)

// FileStore implements the orders.Store interface by appending orders to flat files.
type FileStore struct {
	ordersDir  string
	unsentDir  string
	archiveDir string
	// mutex for unsent directory
	unsentMu sync.Mutex
	// mutex for archive directory
	archiveMu sync.Mutex

	// how long after OrderLimit creation date are OrderLimits no longer accepted (piecestore Config)
	orderLimitGracePeriod time.Duration
}

// NewFileStore creates a new orders file store.
func NewFileStore(ordersDir string, orderLimitGracePeriod time.Duration) *FileStore {
	return &FileStore{
		ordersDir:  ordersDir,
		unsentDir:  filepath.Join(ordersDir, "unsent"),
		archiveDir: filepath.Join(ordersDir, "archive"),

		orderLimitGracePeriod: orderLimitGracePeriod,
	}
}

// Enqueue inserts order to be sent at the end of the unsent file for a particular creation hour.
// It assumes the  order is not being queued after the order limit grace period.
func (store *FileStore) Enqueue(info *Info) (err error) {
	store.unsentMu.Lock()
	defer store.unsentMu.Unlock()

	// if the grace period has already passed, do not enqueue this order
	if store.gracePeriodPassed(info.Limit.OrderCreation.Truncate(time.Hour)) {
		return OrderError.New("grace period passed for order limit")
	}

	f, err := store.getUnsentFile(info.Limit.SatelliteId, info.Limit.OrderCreation)
	if err != nil {
		return OrderError.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, OrderError.Wrap(f.Close()))
	}()

	err = writeLimit(f, info.Limit)
	if err != nil {
		return err
	}
	err = writeOrder(f, info.Order)
	if err != nil {
		return err
	}
	return nil
}

// UnsentInfo is a struct containing a window of orders for a satellite and order creation hour.
type UnsentInfo struct {
	CreatedAtHour time.Time
	InfoList      []*Info
}

// ListUnsentBySatellite returns one window of orders that haven't been sent yet, grouped by satellite.
// It only reads files where the order limit grace period has passed, meaning no new orders will be appended.
// There is a separate window for each created at hour, so if a satellite has 2 windows, `ListUnsentBySatellite`
// needs to be called twice, with calls to `DeleteUnsentFile` in between each call, to see all unsent orders.
func (store *FileStore) ListUnsentBySatellite() (infoMap map[storj.NodeID]UnsentInfo, err error) {
	store.unsentMu.Lock()
	defer store.unsentMu.Unlock()

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
		// if orders can still be added to file, ignore it.
		if !store.gracePeriodPassed(createdAtHour) {
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
			limit, err := readLimit(f)
			if err != nil {
				if errs.Is(err, io.EOF) {
					break
				}
				return err
			}
			order, err := readOrder(f)
			if err != nil {
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

// DeleteUnsentFile deletes an unsent-orders file for a satellite ID and created hour.
func (store *FileStore) DeleteUnsentFile(satelliteID storj.NodeID, createdAtHour time.Time) error {
	store.unsentMu.Lock()
	defer store.unsentMu.Unlock()

	fileName := unsentFilePrefix + satelliteID.String() + "-" + getCreationHourString(createdAtHour)
	filePath := filepath.Join(store.unsentDir, fileName)

	return OrderError.Wrap(os.Remove(filePath))
}

// Archive marks order as being settled.
func (store *FileStore) Archive(archivedAt time.Time, requests ...*ArchivedInfo) error {
	store.archiveMu.Lock()
	defer store.archiveMu.Unlock()

	f, err := store.getNewArchiveFile(archivedAt)
	if err != nil {
		return OrderError.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, OrderError.Wrap(f.Close()))
	}()

	for _, info := range requests {
		err = writeStatus(f, info.Status)
		if err != nil {
			return err
		}
		err = writeLimit(f, info.Limit)
		if err != nil {
			return err
		}
		err = writeOrder(f, info.Order)
		if err != nil {
			return err
		}
	}
	return nil
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
		archivedAt, err := getArchivedFileInfo(info.Name())
		if err != nil {
			return err
		}

		f, err := os.Open(path)
		if err != nil {
			return OrderError.Wrap(err)
		}
		defer func() {
			err = errs.Combine(err, OrderError.Wrap(f.Close()))
		}()

		for {
			status, err := readStatus(f)
			if err != nil {
				if errs.Is(err, io.EOF) {
					break
				}
				return err
			}
			limit, err := readLimit(f)
			if err != nil {
				return err
			}
			order, err := readOrder(f)
			if err != nil {
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
		archivedAt, err := getArchivedFileInfo(info.Name())
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

// getUnsentFile creates or gets the order limit file for appending unsent orders to.
// There is a different file for each satellite and creation hour.
// It expects the caller to lock the store's mutex before calling, and to handle closing the returned file.
func (store *FileStore) getUnsentFile(satelliteID storj.NodeID, creationTime time.Time) (*os.File, error) {
	if _, err := os.Stat(store.unsentDir); os.IsNotExist(err) {
		err = os.Mkdir(store.unsentDir, 0700)
		if err != nil {
			return nil, OrderError.Wrap(err)
		}
	}

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
	creationHour := t.Truncate(time.Hour)
	timeStr := strconv.FormatInt(creationHour.UnixNano(), 10)
	return timeStr
}

// gracePeriodPassed determines whether enough time has passed that no new orders will be added to a file.
func (store *FileStore) gracePeriodPassed(createdHour time.Time) bool {
	canSendCutoff := time.Now().Add(-store.orderLimitGracePeriod)
	// add one hour to include order limits in file added at end of createdHour
	return createdHour.Add(time.Hour).Before(canSendCutoff)
}

// getNewArchiveFile creates the order limit file for appending archived orders to.
// it expects the caller to lock the store's mutex before calling, and to handle closing the returned file.
func (store *FileStore) getNewArchiveFile(archivedAt time.Time) (*os.File, error) {
	if _, err := os.Stat(store.archiveDir); os.IsNotExist(err) {
		err = os.Mkdir(store.archiveDir, 0700)
		if err != nil {
			return nil, OrderError.Wrap(err)
		}
	}

	// suffix of filename is the archivedAt time
	timeStr := strconv.FormatInt(archivedAt.UnixNano(), 10)
	newFilePath := filepath.Join(store.archiveDir, archiveFilePrefix+timeStr)
	// create file if not exists or append
	f, err := os.OpenFile(newFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, OrderError.Wrap(err)
	}
	return f, nil
}

// getUnsentFileInfo gets the satellite ID and created hour from a filename.
// it expects the file name to be in the format "unsent-orders-<satelliteID>-<createdAtHour>"
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
// it expects the file name to be in the format "archived-orders-<archviedAtTime>"
func getArchivedFileInfo(name string) (time.Time, error) {
	if !strings.HasPrefix(name, archiveFilePrefix) {
		return time.Time{}, OrderError.New("Not a valid archived file name: %s", name)
	}
	// chop off prefix to get archived at string
	archivedAtStr := name[len(archiveFilePrefix):]
	archivedAtUnixNano, err := strconv.ParseInt(archivedAtStr, 10, 64)
	if err != nil {
		return time.Time{}, OrderError.Wrap(err)
	}
	archivedAt := time.Unix(0, archivedAtUnixNano)
	return archivedAt, nil
}

// writeLimit writes the size of the order limit bytes, followed by the order limit bytes.
// it expects the caller to have locked the mutex.
func writeLimit(f io.Writer, limit *pb.OrderLimit) error {
	limitSerialized, err := pb.Marshal(limit)
	if err != nil {
		return OrderError.Wrap(err)
	}

	sizeBytes := [4]byte{}
	binary.LittleEndian.PutUint32(sizeBytes[:], uint32(len(limitSerialized)))
	if _, err = f.Write(sizeBytes[:]); err != nil {
		return OrderError.New("Error writing serialized limit size: %w", err)
	}

	if _, err = f.Write(limitSerialized); err != nil {
		return OrderError.New("Error writing serialized limit: %w", err)
	}
	return nil
}

// readLimit reads the size of the limit followed by the serialized limit, and returns the unmarshalled limit.
func readLimit(f io.Reader) (*pb.OrderLimit, error) {
	sizeBytes := [4]byte{}
	_, err := io.ReadFull(f, sizeBytes[:])
	if err != nil {
		return nil, OrderError.Wrap(err)
	}
	limitSize := binary.LittleEndian.Uint32(sizeBytes[:])
	limitSerialized := make([]byte, limitSize)
	_, err = io.ReadFull(f, limitSerialized)
	if err != nil {
		return nil, OrderError.Wrap(err)
	}
	limit := &pb.OrderLimit{}
	err = pb.Unmarshal(limitSerialized, limit)
	if err != nil {
		return nil, OrderError.Wrap(err)
	}
	return limit, nil
}

// writeOrder writes the size of the order bytes, followed by the order bytes.
// it expects the caller to have locked the mutex.
func writeOrder(f io.Writer, order *pb.Order) error {
	orderSerialized, err := pb.Marshal(order)
	if err != nil {
		return OrderError.Wrap(err)
	}

	sizeBytes := [4]byte{}
	binary.LittleEndian.PutUint32(sizeBytes[:], uint32(len(orderSerialized)))
	if _, err = f.Write(sizeBytes[:]); err != nil {
		return OrderError.New("Error writing serialized order size: %w", err)
	}
	if _, err = f.Write(orderSerialized); err != nil {
		return OrderError.New("Error writing serialized order: %w", err)
	}
	return nil
}

// readOrder reads the size of the order followed by the serialized order, and returns the unmarshalled order.
func readOrder(f io.Reader) (*pb.Order, error) {
	sizeBytes := [4]byte{}
	_, err := io.ReadFull(f, sizeBytes[:])
	if err != nil {
		return nil, OrderError.Wrap(err)
	}
	orderSize := binary.LittleEndian.Uint32(sizeBytes[:])
	orderSerialized := make([]byte, orderSize)
	_, err = io.ReadFull(f, orderSerialized)
	if err != nil {
		return nil, OrderError.Wrap(err)
	}
	order := &pb.Order{}
	err = pb.Unmarshal(orderSerialized, order)
	if err != nil {
		return nil, OrderError.Wrap(err)
	}
	return order, nil
}

// writeStatus  writes the satellite response status of an archived order.
// it expects the caller to have locked the mutex.
func writeStatus(f io.Writer, status Status) error {
	if _, err := f.Write([]byte{byte(status)}); err != nil {
		return OrderError.New("Error writing status: %w", err)
	}
	return nil
}

// readStatus reads the status of an archived order limit.
func readStatus(f io.Reader) (Status, error) {
	statusBytes := [1]byte{}
	_, err := io.ReadFull(f, statusBytes[:])
	if err != nil {
		return 0, OrderError.Wrap(err)
	}
	return Status(statusBytes[0]), nil
}
