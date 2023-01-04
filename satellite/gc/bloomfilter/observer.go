// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomfilter

import (
	"archive/zip"
	"context"
	"strconv"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/bloomfilter"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/metabase/segmentloop"
	"storj.io/storj/satellite/overlay"
	"storj.io/uplink"
)

// Observer implements a rangedloop observer to collect bloom filters for the garbage collection.
//
// architecture: Observer
type Observer struct {
	log     *zap.Logger
	config  Config
	overlay overlay.DB

	// The following fields are reset for each loop.
	startTime          time.Time
	lastPieceCounts    map[storj.NodeID]int64
	retainInfos        map[storj.NodeID]*RetainInfo
	latestCreationTime time.Time
	seed               byte
}

var _ (rangedloop.Observer) = (*Observer)(nil)

// NewObserver creates a new instance of the gc rangedloop observer.
func NewObserver(log *zap.Logger, config Config, overlay overlay.DB) *Observer {
	return &Observer{
		log:     log,
		config:  config,
		overlay: overlay,
	}
}

// Start is called at the beginning of each segment loop.
func (obs *Observer) Start(ctx context.Context, startTime time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	switch {
	case obs.config.AccessGrant == "":
		return errs.New("Access Grant is not set")
	case obs.config.Bucket == "":
		return errs.New("Bucket is not set")
	}

	obs.log.Debug("collecting bloom filters started")

	// load last piece counts from overlay db
	lastPieceCounts, err := obs.overlay.AllPieceCounts(ctx)
	if err != nil {
		obs.log.Error("error getting last piece counts", zap.Error(err))
		err = nil
	}
	if lastPieceCounts == nil {
		lastPieceCounts = make(map[storj.NodeID]int64)
	}

	obs.startTime = startTime
	obs.lastPieceCounts = lastPieceCounts
	obs.retainInfos = make(map[storj.NodeID]*RetainInfo, len(lastPieceCounts))
	obs.latestCreationTime = time.Time{}
	obs.seed = bloomfilter.GenerateSeed()
	return nil
}

// Fork creates a Partial to build bloom filters over a chunk of all the segments.
func (obs *Observer) Fork(ctx context.Context) (_ rangedloop.Partial, err error) {
	defer mon.Task()(&ctx)(&err)
	// TODO: refactor PieceTracker after the segmentloop has been removed to
	// more closely match the rangedloop observer needs.
	pieceTracker := NewPieceTrackerWithSeed(obs.log.Named("gc observer"), obs.config, obs.lastPieceCounts, obs.seed)
	if err := pieceTracker.LoopStarted(ctx, segmentloop.LoopInfo{
		Started: obs.startTime,
	}); err != nil {
		return nil, err
	}
	return pieceTracker, nil
}

// Join merges the bloom filters gathered by each Partial.
func (obs *Observer) Join(ctx context.Context, partial rangedloop.Partial) (err error) {
	defer mon.Task()(&ctx)(&err)
	pieceTracker, ok := partial.(*PieceTracker)
	if !ok {
		return errs.New("expected %T but got %T", pieceTracker, partial)
	}

	// Update the count and merge the bloom filters for each node.
	for nodeID, retainInfo := range pieceTracker.RetainInfos {
		if existing, ok := obs.retainInfos[nodeID]; ok {
			existing.Count += retainInfo.Count
			if err := existing.Filter.AddFilter(retainInfo.Filter); err != nil {
				return err
			}
		} else {
			obs.retainInfos[nodeID] = retainInfo
		}
	}

	// Replace the latestCreationTime if the partial observed a later time.
	if obs.latestCreationTime.IsZero() || obs.latestCreationTime.Before(pieceTracker.LatestCreationTime) {
		obs.latestCreationTime = pieceTracker.LatestCreationTime
	}

	return nil
}

// Finish uploads the bloom filters.
func (obs *Observer) Finish(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	if err := obs.uploadBloomFilters(ctx, obs.latestCreationTime, obs.retainInfos); err != nil {
		return err
	}
	obs.log.Debug("collecting bloom filters finished")
	return nil
}

// uploadBloomFilters stores a zipfile with multiple bloom filters in a bucket.
func (obs *Observer) uploadBloomFilters(ctx context.Context, latestCreationDate time.Time, retainInfos map[storj.NodeID]*RetainInfo) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(retainInfos) == 0 {
		return nil
	}

	prefix := time.Now().Format(time.RFC3339)

	expirationTime := time.Now().Add(obs.config.ExpireIn)

	accessGrant, err := uplink.ParseAccess(obs.config.AccessGrant)
	if err != nil {
		return err
	}

	project, err := uplink.OpenProject(ctx, accessGrant)
	if err != nil {
		return err
	}
	defer func() {
		// do cleanup in case of any error while uploading bloom filters
		if err != nil {
			// TODO should we drop whole bucket if cleanup will fail
			err = errs.Combine(err, obs.cleanup(ctx, project, prefix))
		}
		err = errs.Combine(err, project.Close())
	}()

	_, err = project.EnsureBucket(ctx, obs.config.Bucket)
	if err != nil {
		return err
	}

	// TODO move it before segment loop is started
	o := uplink.ListObjectsOptions{
		Prefix: prefix + "/",
	}
	iterator := project.ListObjects(ctx, obs.config.Bucket, &o)
	for iterator.Next() {
		if iterator.Item().IsPrefix {
			continue
		}

		obs.log.Warn("target bucket was not empty, stop operation and wait for next execution", zap.String("bucket", obs.config.Bucket))
		return nil
	}

	infos := make([]internalpb.RetainInfo, 0, obs.config.ZipBatchSize)
	batchNumber := 0
	for nodeID, info := range retainInfos {
		infos = append(infos, internalpb.RetainInfo{
			Filter: info.Filter.Bytes(),
			// because bloom filters should be created from immutable database
			// snapshot we are using latest segment creation date
			CreationDate:  latestCreationDate,
			PieceCount:    int64(info.Count),
			StorageNodeId: nodeID,
		})

		if len(infos) == obs.config.ZipBatchSize {
			err = obs.uploadPack(ctx, project, prefix, batchNumber, expirationTime, infos)
			if err != nil {
				return err
			}

			infos = infos[:0]
			batchNumber++
		}
	}

	// upload rest of infos if any
	if err := obs.uploadPack(ctx, project, prefix, batchNumber, expirationTime, infos); err != nil {
		return err
	}

	// update LATEST file
	upload, err := project.UploadObject(ctx, obs.config.Bucket, LATEST, nil)
	if err != nil {
		return err
	}
	_, err = upload.Write([]byte(prefix))
	if err != nil {
		return err
	}

	return upload.Commit()
}

// uploadPack uploads single zip pack with multiple bloom filters.
func (obs *Observer) uploadPack(ctx context.Context, project *uplink.Project, prefix string, batchNumber int, expirationTime time.Time, infos []internalpb.RetainInfo) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(infos) == 0 {
		return nil
	}

	upload, err := project.UploadObject(ctx, obs.config.Bucket, prefix+"/bloomfilters-"+strconv.Itoa(batchNumber)+".zip", &uplink.UploadOptions{
		Expires: expirationTime,
	})
	if err != nil {
		return err
	}

	zipWriter := zip.NewWriter(upload)
	defer func() {
		err = errs.Combine(err, zipWriter.Close())
		if err != nil {
			err = errs.Combine(err, upload.Abort())
		} else {
			err = upload.Commit()
		}
	}()

	for _, info := range infos {
		retainInfoBytes, err := pb.Marshal(&info)
		if err != nil {
			return err
		}

		writer, err := zipWriter.Create(info.StorageNodeId.String())
		if err != nil {
			return err
		}

		write, err := writer.Write(retainInfoBytes)
		if err != nil {
			return err
		}
		if len(retainInfoBytes) != write {
			return errs.New("not whole bloom filter was written")
		}
	}

	return nil
}

// cleanup moves all objects from root location to unique prefix. Objects will be deleted
// automatically when expires.
func (obs *Observer) cleanup(ctx context.Context, project *uplink.Project, prefix string) (err error) {
	defer mon.Task()(&ctx)(&err)

	errPrefix := "upload-error-" + time.Now().Format(time.RFC3339)
	o := uplink.ListObjectsOptions{
		Prefix: prefix + "/",
	}
	iterator := project.ListObjects(ctx, obs.config.Bucket, &o)

	for iterator.Next() {
		item := iterator.Item()
		if item.IsPrefix {
			continue
		}

		err := project.MoveObject(ctx, obs.config.Bucket, item.Key, obs.config.Bucket, prefix+"/"+errPrefix+"/"+item.Key, nil)
		if err != nil {
			return err
		}
	}

	return iterator.Err()
}
