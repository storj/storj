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

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/private/date"
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
	// how long after the grace period passes to start submitting orders
	maxInFlightTime time.Duration
}

// NewFileStore creates a new orders file store, and the directories necessary for its use.
func NewFileStore(ordersDir string, orderLimitGracePeriod, maxInFlightTime time.Duration) (*FileStore, error) {
	fs := &FileStore{
		ordersDir:             ordersDir,
		unsentDir:             filepath.Join(ordersDir, "unsent"),
		archiveDir:            filepath.Join(ordersDir, "archive"),
		orderLimitGracePeriod: orderLimitGracePeriod,
		maxInFlightTime:       maxInFlightTime,
	}

	err := fs.ensureDirectories()
	if err != nil {
		return nil, err
	}

	return fs, nil
}

// Enqueue inserts order to be sent at the end of the unsent file for a particular creation hour.
// It assumes the  order is not being queued after the order limit grace period.
func (store *FileStore) Enqueue(info *Info) (err error) {
	store.unsentMu.Lock()
	defer store.unsentMu.Unlock()

	// if the settle buffer period has already passed, do not enqueue this order
	if store.settleBufferPassed(info.Limit.OrderCreation.Truncate(time.Hour)) {
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
// needs to be called twice, with calls to `Archive` in between each call, to see all unsent orders.
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
		if !store.settleBufferPassed(createdAtHour) {
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

// TestSetSettleBuffer is a function that allows us to modify order limit grace period and max inflight time for testing purposes.
func (store *FileStore) TestSetSettleBuffer(orderLimitGracePeriod, maxInFlightTime time.Duration) {
	store.unsentMu.Lock()
	defer store.unsentMu.Unlock()

	store.orderLimitGracePeriod = orderLimitGracePeriod
	store.maxInFlightTime = maxInFlightTime
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

// settleBufferPassed determines whether enough time has passed that no new orders will be added to a file.
func (store *FileStore) settleBufferPassed(createdHour time.Time) bool {
	// wait until the gracePeriod+maxInFlightTime has passed, to ensure in-flight actions have completed
	canSendCutoff := time.Now().Add(-store.orderLimitGracePeriod).Add(-store.maxInFlightTime)
	// add one hour to include order limits in file added at end of createdHour
	return createdHour.Add(time.Hour).Before(canSendCutoff)
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
