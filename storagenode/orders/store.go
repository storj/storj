// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/common/storj"
)

const (
	unsentStagingFileName = "unsent-orders-staging"
	unsentReadyFilePrefix = "unsent-orders-ready-"
)

// FileStore implements the orders.Store interface by appending orders to flat files.
type FileStore struct {
	ordersDir  string
	unsentDir  string
	archiveDir string
	mu         sync.Mutex
}

// NewFileStore creates a new orders file store.
func NewFileStore(ordersDir string) *FileStore {
	return &FileStore{
		ordersDir:  ordersDir,
		unsentDir:  filepath.Join(ordersDir, "unsent"),
		archiveDir: filepath.Join(ordersDir, "archive"),
	}
}

// Enqueue inserts order to the list of orders needing to be sent to the satellite.
func (store *FileStore) Enqueue(info *Info) (err error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	f, err := store.getUnsentStagingFile()
	if err != nil {
		return OrderError.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, f.Close())
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

// ListUnsentBySatellite returns orders that haven't been sent yet grouped by satellite.
// It copies the staging file to a read-only "ready to send" file first.
// It should never be called concurrently with DeleteReadyToSendFiles.
func (store *FileStore) ListUnsentBySatellite() (infoMap map[storj.NodeID][]*Info, err error) {
	err = store.convertUnsentStagingToReady()
	if err != nil {
		return infoMap, err
	}

	var errList error
	infoMap = make(map[storj.NodeID][]*Info)

	err = filepath.Walk(store.unsentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			errList = errs.Combine(errList, err)
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if info.Name() == unsentStagingFileName {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return OrderError.Wrap(err)
		}
		defer func() {
			err = errs.Combine(err, f.Close())
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
			infoList := infoMap[limit.SatelliteId]
			infoList = append(infoList, newInfo)
			infoMap[limit.SatelliteId] = infoList
		}
		return nil
	})
	if err != nil {
		errList = errs.Combine(errList, err)
	}

	return infoMap, errList
}

// DeleteReadyToSendFiles deletes all non-staging files in the "unsent" directory.
// It should be called after the order limits have been sent.
// It should never be called concurrently with ListUnsentBySatellite.
func (store *FileStore) DeleteReadyToSendFiles() (err error) {
	var errList error

	err = filepath.Walk(store.unsentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			errList = errs.Combine(errList, OrderError.Wrap(err))
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if info.Name() == unsentStagingFileName {
			return nil
		}
		// delete all non-staging files
		return OrderError.Wrap(os.Remove(path))
	})
	if err != nil {
		errList = errs.Combine(errList, err)
	}

	return errList
}

// convertUnsentStagingToReady converts the unsent staging file to be read only, and renames it.
func (store *FileStore) convertUnsentStagingToReady() error {
	// lock mutex so no one tries to write to the file while we do this
	store.mu.Lock()
	defer store.mu.Unlock()

	oldFileName := unsentStagingFileName
	oldFilePath := filepath.Join(store.unsentDir, oldFileName)
	if _, err := os.Stat(oldFilePath); os.IsNotExist(err) {
		return nil
	}

	// set file to readonly
	err := os.Chmod(oldFilePath, 0444)
	if err != nil {
		return err
	}
	// make new file suffix the current time in case there are other "ready" files already
	timeStr := strconv.FormatInt(time.Now().UnixNano(), 10)
	newFilePath := filepath.Join(store.unsentDir, unsentReadyFilePrefix+timeStr)
	// rename file
	return os.Rename(oldFilePath, newFilePath)
}

// getUnsentStagingFile creates or gets the order limit file for appending unsent orders to.
// it expects the caller to lock the store's mutex before calling, and to handle closing the returned file.
func (store *FileStore) getUnsentStagingFile() (*os.File, error) {
	if _, err := os.Stat(store.unsentDir); os.IsNotExist(err) {
		err = os.Mkdir(store.unsentDir, 0700)
		if err != nil {
			return nil, OrderError.Wrap(err)
		}
	}

	fileName := unsentStagingFileName
	filePath := filepath.Join(store.unsentDir, fileName)
	// create file if not exists or append
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, OrderError.Wrap(err)
	}
	return f, nil
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
