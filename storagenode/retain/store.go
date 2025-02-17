// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package retain

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/zeebo/errs"
	"golang.org/x/exp/maps"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/shared/bloomfilter"
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

		// previously, we were storing only the bloom filter in the file, but now we store the entire request
		// so .pb is appended to the filename to indicate that the file contains a protobuf message.
		// To ensure backwards compatibility, we check if the filename ends with .pb and remove it if it does
		isProtobufRequest := strings.HasSuffix(unixTimeStr, ".pb")
		if isProtobufRequest {
			unixTimeStr = strings.TrimSuffix(unixTimeStr, ".pb")
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
			Filename:      filename,
		}

		{
			data, err := os.ReadFile(filepath.Join(path, filename))
			if err != nil {
				errsEncountered.Add(err)
				continue
			}

			if isProtobufRequest {
				pbReq := new(pb.RetainRequest)

				err = pb.Unmarshal(data, pbReq)
				if err != nil {
					errsEncountered.Add(errs.New("failed to parse pb file; %w", err))
					continue
				}

				err = verifyHash(pbReq)
				if err != nil {
					errsEncountered.Add(errs.New("failed to verify hash: %w", err))
					continue
				}

				data = pbReq.Filter
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
		}

		// check if there is already a request for this satellite
		prevReq, ok := store.data[satelliteID]
		if ok {
			if prevReq.CreatedBefore.After(req.CreatedBefore) {
				// if the new request is older, keep the old one
				continue
			}

			// if the new request is newer, remove the old one
			err := DeleteFile(store.path, prevReq.GetFilename())
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
func (store *RequestStore) Add(satelliteID storj.NodeID, pbReq *pb.RetainRequest) (bool, error) {
	// if there is already a request for this satellite, remove it
	if prevReq, ok := store.data[satelliteID]; ok {
		err := DeleteFile(store.path, prevReq.GetFilename())
		if err != nil {
			return false, err
		}
	}

	filter, err := bloomfilter.NewFromBytes(pbReq.GetFilter())
	if err != nil {
		return false, err
	}

	request := Request{
		SatelliteID:   satelliteID,
		CreatedBefore: pbReq.GetCreationDate(),
		Filter:        filter,
	}
	store.data[satelliteID] = request

	// save the new request
	return true, SaveRequest(store.path, request.GetFilename(), pbReq)
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
	return DeleteFile(store.path, req.GetFilename())
}

// Next returns the next request from the store.
func (store *RequestStore) Next() (Request, bool) {
	if len(store.data) == 0 {
		return Request{}, false
	}

	filters := maps.Values(store.data)
	sort.Slice(filters, func(i, j int) bool {
		return filters[i].CreatedBefore.Before(filters[j].CreatedBefore)
	})
	return filters[0], true
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
func SaveRequest(path, filename string, request *pb.RetainRequest) error {
	data, err := pb.Marshal(request)
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(path, filename), data, 0644)
}

// verifyHash calculates and verifies the hash of the filter.
func verifyHash(req *pb.RetainRequest) error {
	if len(req.Hash) == 0 {
		return nil
	}
	hasher := pb.NewHashFromAlgorithm(req.HashAlgorithm)
	_, err := hasher.Write(req.GetFilter())
	if err != nil {
		return errs.Wrap(err)
	}
	if !bytes.Equal(req.Hash, hasher.Sum(nil)) {
		return errs.New("hash mismatch")
	}

	return nil
}
