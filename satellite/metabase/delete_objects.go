// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
)

const (
	deleteBatchsizeLimit  = intLimitRange(1000)
	deleteObjectsMaxItems = 1000
)

// DeleteObjects contains options for deleting multiple committed objects from a bucket.
type DeleteObjects struct {
	ProjectID  uuid.UUID
	BucketName BucketName
	Items      []DeleteObjectsItem

	Versioned bool
	Suspended bool

	ObjectLock ObjectLockDeleteOptions
}

// DeleteObjectsItem describes the location of an object in a bucket to be deleted.
type DeleteObjectsItem struct {
	ObjectKey       ObjectKey
	StreamVersionID StreamVersionID
}

// Verify verifies bucket object deletion request fields.
func (opts DeleteObjects) Verify() error {
	itemCount := len(opts.Items)
	switch {
	case opts.Versioned && opts.Suspended:
		return ErrInvalidRequest.New("Versioned and Suspended must not be simultaneously enabled")
	case opts.ObjectLock.Enabled:
		return ErrInvalidRequest.New("deletion from buckets with Object Lock enabled is not yet supported")
	case opts.ProjectID.IsZero():
		return ErrInvalidRequest.New("ProjectID missing")
	case opts.BucketName == "":
		return ErrInvalidRequest.New("BucketName missing")
	case itemCount == 0:
		return ErrInvalidRequest.New("Items missing")
	case itemCount > deleteObjectsMaxItems:
		return ErrInvalidRequest.New("Items is too long; expected <= %d, but got %d", deleteObjectsMaxItems, itemCount)
	}
	for i, item := range opts.Items {
		if item.ObjectKey == "" {
			return ErrInvalidRequest.New("Items[%d].ObjectKey missing", i)
		}
		version := item.StreamVersionID.Version()
		if !item.StreamVersionID.IsZero() && version <= 0 {
			return ErrInvalidRequest.New("Items[%d].StreamVersionID invalid: version is %v", i, version)
		}
	}
	return nil
}

// DeleteObjectsResult contains the results of an attempt to delete specific objects from a bucket.
type DeleteObjectsResult struct {
	Items               []DeleteObjectsResultItem
	DeletedSegmentCount int64
}

// DeleteObjectsStatus represents the success or failure status of an individual DeleteObjects deletion.
type DeleteObjectsStatus int

const (
	// DeleteStatusUnprocessed indicates that the deletion was not processed due to an internal error.
	DeleteStatusUnprocessed DeleteObjectsStatus = iota
	// DeleteStatusNotFound indicates that the object could not be deleted because it didn't exist.
	DeleteStatusNotFound
	// DeleteStatusOK indicates that the object was successfully deleted.
	DeleteStatusOK
)

// DeleteObjectsResultItem contains the result of an attempt to delete a specific object from a bucket.
type DeleteObjectsResultItem struct {
	ObjectKey                ObjectKey
	RequestedStreamVersionID StreamVersionID

	Removed *DeleteObjectsInfo
	Marker  *DeleteObjectsInfo

	Status DeleteObjectsStatus
}

// DeleteObjectsInfo contains information about an object that was deleted or a delete marker that was inserted
// as a result of processing a DeleteObjects request item.
type DeleteObjectsInfo struct {
	StreamVersionID StreamVersionID
	Status          ObjectStatus
}

// DeleteObjects deletes specific objects from a bucket.
//
// TODO: Support Object Lock.
func (db *DB) DeleteObjects(ctx context.Context, opts DeleteObjects) (result DeleteObjectsResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return DeleteObjectsResult{}, errs.Wrap(err)
	}

	defer func() {
		var deletedObjects int
		for _, item := range result.Items {
			if item.Status == DeleteStatusOK && item.Removed != nil {
				deletedObjects++
			}
		}
		mon.Meter("object_delete").Mark(deletedObjects)
		mon.Meter("segment_delete").Mark64(result.DeletedSegmentCount)
	}()

	adapter := db.ChooseAdapter(opts.ProjectID)
	processedOpts := opts.processResults()
	result.Items = processedOpts.results

	for i := 0; i < processedOpts.lastCommittedCount; i++ {
		resultItem := &processedOpts.results[i]

		deleteOpts := DeleteObjectLastCommitted{
			ObjectLocation: ObjectLocation{
				ProjectID:  opts.ProjectID,
				BucketName: opts.BucketName,
				ObjectKey:  resultItem.ObjectKey,
			},
		}

		var deleteObjectResult DeleteObjectResult
		if opts.Versioned {
			var deleteMarkerStreamID uuid.UUID
			deleteMarkerStreamID, err = generateDeleteMarkerStreamID()
			if err != nil {
				return result, err
			}
			deleteObjectResult, err = adapter.DeleteObjectLastCommittedVersioned(ctx, deleteOpts, deleteMarkerStreamID)
		} else if opts.Suspended {
			var deleteMarkerStreamID uuid.UUID
			deleteMarkerStreamID, err = generateDeleteMarkerStreamID()
			if err != nil {
				return result, err
			}
			deleteObjectResult, err = adapter.DeleteObjectLastCommittedSuspended(ctx, deleteOpts, deleteMarkerStreamID)
			if ErrObjectNotFound.Has(err) {
				err = nil
			}
		} else {
			deleteObjectResult, err = adapter.DeleteObjectLastCommittedPlain(ctx, deleteOpts)
		}

		result.DeletedSegmentCount += int64(deleteObjectResult.DeletedSegmentCount)

		if len(deleteObjectResult.Removed) > 0 {
			removed := deleteObjectResult.Removed[0]
			sv := removed.StreamVersionID()
			deleteInfo := &DeleteObjectsInfo{
				StreamVersionID: sv,
				Status:          CommittedUnversioned,
			}
			resultItem.Removed = deleteInfo
			resultItem.Status = DeleteStatusOK

			if !opts.Versioned {
				// Handle the case where an object was specified twice in the deletion request:
				// once with a version omitted and once with a version set. We must ensure that
				// when the object is deleted, both result items that reference it are updated.
				if i, ok := processedOpts.resultsIndices[DeleteObjectsItem{
					ObjectKey:       resultItem.ObjectKey,
					StreamVersionID: sv,
				}]; ok {
					processedOpts.results[i].Removed = deleteInfo
					processedOpts.results[i].Status = DeleteStatusOK
				}
			}
		}

		if len(deleteObjectResult.Markers) > 0 {
			marker := deleteObjectResult.Markers[0]
			resultItem.Marker = &DeleteObjectsInfo{
				StreamVersionID: marker.StreamVersionID(),
				Status:          marker.Status,
			}
			resultItem.Status = DeleteStatusOK
		}

		if err != nil {
			return result, err
		}

		if resultItem.Status == DeleteStatusUnprocessed {
			resultItem.Status = DeleteStatusNotFound
		}
	}

	for i := processedOpts.lastCommittedCount; i < len(processedOpts.results); i++ {
		resultItem := &processedOpts.results[i]
		if resultItem.Status == DeleteStatusOK {
			continue
		}

		if opts.Versioned || opts.Suspended {
			// Prevent the removal of a delete marker that was added in a previous iteration.
			if linkedItemIdx, ok := processedOpts.resultsIndices[DeleteObjectsItem{
				ObjectKey: resultItem.ObjectKey,
			}]; ok {
				marker := processedOpts.results[linkedItemIdx].Marker
				if marker != nil && marker.StreamVersionID == resultItem.RequestedStreamVersionID {
					continue
				}
			}
		}

		var deleteObjectResult DeleteObjectResult
		deleteObjectResult, err = adapter.DeleteObjectExactVersion(ctx, DeleteObjectExactVersion{
			ObjectLocation: ObjectLocation{
				ProjectID:  opts.ProjectID,
				BucketName: opts.BucketName,
				ObjectKey:  resultItem.ObjectKey,
			},
			Version:        resultItem.RequestedStreamVersionID.Version(),
			StreamIDSuffix: resultItem.RequestedStreamVersionID.StreamIDSuffix(),
		})

		result.DeletedSegmentCount += int64(deleteObjectResult.DeletedSegmentCount)

		if len(deleteObjectResult.Removed) > 0 {
			resultItem.Status = DeleteStatusOK
			resultItem.Removed = &DeleteObjectsInfo{
				StreamVersionID: resultItem.RequestedStreamVersionID,
				Status:          deleteObjectResult.Removed[0].Status,
			}
		}

		if err != nil {
			return result, err
		}

		if resultItem.Status == DeleteStatusUnprocessed {
			resultItem.Status = DeleteStatusNotFound
		}
	}

	return result, err
}

type deleteObjectsSetupInfo struct {
	results            []DeleteObjectsResultItem
	resultsIndices     map[DeleteObjectsItem]int
	lastCommittedCount int
}

// processResults returns data that (*Adapter).DeleteObjects implementations require for executing database queries.
func (opts DeleteObjects) processResults() (info deleteObjectsSetupInfo) {
	info.resultsIndices = make(map[DeleteObjectsItem]int, len(opts.Items))
	for _, item := range opts.Items {
		if _, exists := info.resultsIndices[item]; !exists {
			info.resultsIndices[item] = -1
			if item.StreamVersionID.IsZero() {
				info.lastCommittedCount++
			}
		}
	}

	info.results = make([]DeleteObjectsResultItem, len(info.resultsIndices))

	// We process last committed items first to allow for a simpler implementation
	// than what would otherwise be possible. This shouldn't result in any difference
	// in the result items' contents or the overall effect on the database.
	// If an object is requested for deletion both by last committed and exact version
	// request items, each result item should reflect the effects of processing its
	// respective request item in isolation, so the order in which the request items
	// are processed isn't significant.

	lastCommittedCounter := 0
	versionedCounter := info.lastCommittedCount
	for _, item := range opts.Items {
		if info.resultsIndices[item] == -1 {
			counter := &lastCommittedCounter
			if !item.StreamVersionID.IsZero() {
				counter = &versionedCounter
			}
			info.results[*counter] = DeleteObjectsResultItem{
				ObjectKey:                item.ObjectKey,
				RequestedStreamVersionID: item.StreamVersionID,
			}
			info.resultsIndices[item] = *counter
			*counter++
		}
	}

	return info
}
