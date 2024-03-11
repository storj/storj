// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package retain

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/bloomfilter"
	"storj.io/common/storj"
	"storj.io/storj/storagenode/blobstore/filestore"
)

// RequestStore is a cache of requests to retain pieces.
type RequestStore struct {
	path string
	data map[storj.NodeID]Request
}

// NewRequestStore loads the request caches from disk.
func NewRequestStore(path string) (RequestStore, error) {
	store := RequestStore{
		path: path,
		data: make(map[storj.NodeID]Request),
	}

	err := os.MkdirAll(path, 0777)
	if err != nil {
		return store, errs.New("unable to create retain store directory: %w", err)
	}

	files, err := os.ReadDir(path)
	if err != nil {
		return store, Error.Wrap(err)
	}

	var errsEncountered errs.Group

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filename := file.Name()

		idEncoded, unixTimeStr, found := strings.Cut(filename, "-")
		if !found {
			// skip files that don't match the expected format
			errsEncountered.Add(errs.New("invalid filename: %s", filename))
			continue
		}

		id, err := filestore.PathEncoding.DecodeString(idEncoded)
		if err != nil {
			errsEncountered.Add(errs.New("invalid filename: %s; %w", filename, err))
			continue
		}

		satelliteID, err := storj.NodeIDFromBytes(id)
		if err != nil {
			errsEncountered.Add(errs.New("invalid filename: %s; %w", filename, err))
			continue
		}

		unixTime, err := strconv.ParseInt(unixTimeStr, 10, 64)
		if err != nil {
			errsEncountered.Add(errs.New("invalid filename: %s; %w", filename, err))
			continue
		}

		createdBefore := time.Unix(0, unixTime)
		if createdBefore.IsZero() || createdBefore.Equal(time.Unix(0, 0)) {
			errsEncountered.Add(errs.New("invalid filename: %s; failed time validation", filename))
			continue
		}

		req := Request{
			SatelliteID:   satelliteID,
			CreatedBefore: time.Unix(0, unixTime),
		}

		data, err := os.ReadFile(filepath.Join(path, req.Filename()))
		if err != nil {
			errsEncountered.Add(err)
			continue
		}

		req.Filter, err = bloomfilter.NewFromBytes(data)
		if err != nil {
			errsEncountered.Add(errs.New("malformed bloom filter: %w", err))
			err := DeleteFile(store.path, file.Name())
			if err != nil {
				errsEncountered.Add(errs.New("failed to delete malformed bloom filter from store: %w", err))
			}
			continue
		}

		// check if there is already a request for this satellite
		prevReq, ok := store.data[satelliteID]
		if ok {
			if prevReq.CreatedBefore.After(req.CreatedBefore) {
				// if the new request is older, keep the old one
				continue
			}

			// if the new request is newer, remove the old one
			err := DeleteFile(store.path, prevReq.Filename())
			if err != nil {
				errsEncountered.Add(errs.New("failed to delete old bloomfilter file: %w", err))
			}
		}

		store.data[satelliteID] = req
	}

	return store, errsEncountered.Err()
}

// Data returns the data in the store.
func (store *RequestStore) Data() map[storj.NodeID]Request {
	return store.data
}

// Add adds a request to the store. It returns true if the request was added, and an
// error if the file could not be saved.
func (store *RequestStore) Add(req Request) (bool, error) {
	// if there is already a request for this satellite, remove it
	if prevReq, ok := store.data[req.SatelliteID]; ok {
		err := DeleteFile(store.path, prevReq.Filename())
		if err != nil {
			return false, err
		}
	}
	store.data[req.SatelliteID] = req
	// save the new request
	err := SaveRequest(store.path, req)
	if err != nil {
		return true, err
	}
	return true, nil
}

// Remove removes a request from the queue. It returns true if the request was found
// in the queue.
// It does not remove the cache file from the filesystem.
func (store *RequestStore) Remove(req Request) bool {
	foundReq, ok := store.data[req.SatelliteID]
	if !ok {
		return false
	}

	if foundReq.CreatedBefore != req.CreatedBefore {
		return false
	}

	delete(store.data, req.SatelliteID)
	return true
}

// DeleteCache removes the request from the store and deletes the cache file.
func (store *RequestStore) DeleteCache(req Request) error {
	_ = store.Remove(req)
	return DeleteFile(store.path, req.Filename())
}

// Next returns the next request from the store.
func (store *RequestStore) Next() (Request, bool) {
	for _, req := range store.data {
		return req, true
	}
	return Request{}, false
}

// Len returns the number of requests in the store.
func (store *RequestStore) Len() int {
	return len(store.data)
}

// DeleteFile removes a file from the filesystem.
func DeleteFile(path, filename string) error {
	err := os.Remove(filepath.Join(path, filename))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// SaveRequest stores the request to the filesystem.
func SaveRequest(path string, req Request) error {
	err := os.WriteFile(filepath.Join(path, req.Filename()), req.Filter.Bytes(), 0644)
	if err != nil {
		return err
	}
	return nil
}
