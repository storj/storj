// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomfilter

import (
	"archive/zip"
	"context"
	"strconv"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/metabase/segmentloop"
	"storj.io/storj/satellite/overlay"
	"storj.io/uplink"
)

var mon = monkit.Package()

// Config contains configurable values for garbage collection.
type Config struct {
	Interval time.Duration `help:"the time between each garbage collection executions" releaseDefault:"120h" devDefault:"10m" testDefault:"$TESTINTERVAL"`
	// TODO service is not enabled by default for testing until will be finished
	Enabled bool `help:"set if garbage collection bloom filters is enabled or not" default:"true" testDefault:"false"`

	RunOnce bool `help:"set if garbage collection bloom filter process should only run once then exit" default:"false"`

	UseRangedLoop bool `help:"whether to use ranged loop instead of segment loop" default:"false"`

	// value for InitialPieces currently based on average pieces per node
	InitialPieces     int64   `help:"the initial number of pieces expected for a storage node to have, used for creating a filter" releaseDefault:"400000" devDefault:"10"`
	FalsePositiveRate float64 `help:"the false positive rate used for creating a garbage collection bloom filter" releaseDefault:"0.1" devDefault:"0.1"`

	AccessGrant  string        `help:"Access Grant which will be used to upload bloom filters to the bucket" default:""`
	Bucket       string        `help:"Bucket which will be used to upload bloom filters" default:"" testDefault:"gc-queue"` // TODO do we need full location?
	ZipBatchSize int           `help:"how many bloom filters will be packed in a single zip" default:"500" testDefault:"2"`
	ExpireIn     time.Duration `help:"how long bloom filters will remain in the bucket for gc/sender to consume before being automatically deleted" default:"336h"`
}

// Service implements service to collect bloom filters for the garbage collection.
//
// architecture: Chore
type Service struct {
	log    *zap.Logger
	config Config
	Loop   *sync2.Cycle

	overlay     overlay.DB
	segmentLoop *segmentloop.Service
}

// NewService creates a new instance of the gc service.
func NewService(log *zap.Logger, config Config, overlay overlay.DB, loop *segmentloop.Service) *Service {
	return &Service{
		log:         log,
		config:      config,
		Loop:        sync2.NewCycle(config.Interval),
		overlay:     overlay,
		segmentLoop: loop,
	}
}

// Run starts the gc loop service.
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if !service.config.Enabled {
		return nil
	}

	switch {
	case service.config.AccessGrant == "":
		return errs.New("Access Grant is not set")
	case service.config.Bucket == "":
		return errs.New("Bucket is not set")
	}

	return service.Loop.Run(ctx, service.RunOnce)
}

// RunOnce runs service only once.
func (service *Service) RunOnce(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	service.log.Debug("collecting bloom filters started")

	// load last piece counts from overlay db
	lastPieceCounts, err := service.overlay.AllPieceCounts(ctx)
	if err != nil {
		service.log.Error("error getting last piece counts", zap.Error(err))
		err = nil
	}
	if lastPieceCounts == nil {
		lastPieceCounts = make(map[storj.NodeID]int64)
	}

	pieceTracker := NewPieceTracker(service.log.Named("gc observer"), service.config, lastPieceCounts)

	// collect things to retain
	err = service.segmentLoop.Join(ctx, pieceTracker)
	if err != nil {
		service.log.Error("error joining metainfoloop", zap.Error(err))
		return nil
	}

	err = service.uploadBloomFilters(ctx, pieceTracker.LatestCreationTime, pieceTracker.RetainInfos)
	if err != nil {
		return err
	}

	service.log.Debug("collecting bloom filters finished")

	return nil
}

// uploadBloomFilters stores a zipfile with multiple bloom filters in a bucket.
func (service *Service) uploadBloomFilters(ctx context.Context, latestCreationDate time.Time, retainInfos map[storj.NodeID]*RetainInfo) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(retainInfos) == 0 {
		return nil
	}

	prefix := time.Now().Format(time.RFC3339)

	expirationTime := time.Now().Add(service.config.ExpireIn)

	accessGrant, err := uplink.ParseAccess(service.config.AccessGrant)
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
			err = errs.Combine(err, service.cleanup(ctx, project, prefix))
		}
		err = errs.Combine(err, project.Close())
	}()

	_, err = project.EnsureBucket(ctx, service.config.Bucket)
	if err != nil {
		return err
	}

	// TODO move it before segment loop is started
	o := uplink.ListObjectsOptions{
		Prefix: prefix + "/",
	}
	iterator := project.ListObjects(ctx, service.config.Bucket, &o)
	for iterator.Next() {
		if iterator.Item().IsPrefix {
			continue
		}

		service.log.Warn("target bucket was not empty, stop operation and wait for next execution", zap.String("bucket", service.config.Bucket))
		return nil
	}

	infos := make([]internalpb.RetainInfo, 0, service.config.ZipBatchSize)
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

		if len(infos) == service.config.ZipBatchSize {
			err = service.uploadPack(ctx, project, prefix, batchNumber, expirationTime, infos)
			if err != nil {
				return err
			}

			infos = infos[:0]
			batchNumber++
		}
	}

	// upload rest of infos if any
	if err := service.uploadPack(ctx, project, prefix, batchNumber, expirationTime, infos); err != nil {
		return err
	}

	// update LATEST file
	upload, err := project.UploadObject(ctx, service.config.Bucket, LATEST, nil)
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
func (service *Service) uploadPack(ctx context.Context, project *uplink.Project, prefix string, batchNumber int, expirationTime time.Time, infos []internalpb.RetainInfo) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(infos) == 0 {
		return nil
	}

	upload, err := project.UploadObject(ctx, service.config.Bucket, prefix+"/bloomfilters-"+strconv.Itoa(batchNumber)+".zip", &uplink.UploadOptions{
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
func (service *Service) cleanup(ctx context.Context, project *uplink.Project, prefix string) (err error) {
	defer mon.Task()(&ctx)(&err)

	errPrefix := "upload-error-" + time.Now().Format(time.RFC3339)
	o := uplink.ListObjectsOptions{
		Prefix: prefix + "/",
	}
	iterator := project.ListObjects(ctx, service.config.Bucket, &o)

	for iterator.Next() {
		item := iterator.Item()
		if item.IsPrefix {
			continue
		}

		err := project.MoveObject(ctx, service.config.Bucket, item.Key, service.config.Bucket, prefix+"/"+errPrefix+"/"+item.Key, nil)
		if err != nil {
			return err
		}
	}

	return iterator.Err()
}
