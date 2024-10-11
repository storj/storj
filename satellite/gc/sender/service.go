// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package sender

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/satellite/gc/bloomfilter"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/overlay"
	"storj.io/uplink"
	"storj.io/uplink/private/piecestore"
)

var (
	// Error defines the gc service errors class.
	Error = errs.Class("gc")
	mon   = monkit.Package()
)

// Config contains configurable values for garbage collection.
type Config struct {
	Interval time.Duration `help:"the time between each attempt to download and send garbage collection retain filters to storage nodes" default:"1h" devDefault:"5m" testDefault:"$TESTINTERVAL"`
	Enabled  bool          `help:"set if loop to send garbage collection retain filters is enabled" default:"true" devDefault:"false"`

	// We suspect this currently not to be the critical path w.r.t. garbage collection, so no paralellism is implemented.
	ConcurrentSends   int           `help:"the number of nodes to concurrently send garbage collection retain filters to" default:"100" devDefault:"1"`
	RetainSendTimeout time.Duration `help:"the amount of time to allow a node to handle a retain request" default:"1m"`

	AccessGrant string        `help:"Access to download the bloom filters. Needs read and write permission."`
	Bucket      string        `help:"bucket where retain info is stored" default:"" testDefault:"gc-queue"`
	ExpireIn    time.Duration `help:"Expiration of newly created objects in the bucket. These objects are under the prefix error-[timestamp] and store error messages." default:"336h"`
}

// NewService creates a new instance of the gc sender service.
func NewService(log *zap.Logger, config Config, dialer rpc.Dialer, overlay overlay.DB) *Service {
	return &Service{
		log:    log,
		Config: config,
		Loop:   sync2.NewCycle(config.Interval),

		dialer:  dialer,
		overlay: overlay,
	}
}

// Service reads bloom filters of piece IDs to retain from a Storj bucket
// and sends them out to the storage nodes. This is intended to run on a live satellite,
// not on a backup database.
//
// The split between creating retain info and sending it out to storagenodes
// is made so that the bloom filter can be created from a backup database.
//
// architecture: Service
type Service struct {
	log    *zap.Logger
	Config Config
	Loop   *sync2.Cycle

	dialer  rpc.Dialer
	overlay overlay.DB
}

// Run continuously polls for new retain filters and sends them out.
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if !service.Config.Enabled {
		return nil
	}

	return service.Loop.Run(ctx, service.RunOnce)
}

// RunOnce opens the bucket and sends out all the retain filters located in it to the storage nodes.
func (service *Service) RunOnce(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	switch {
	case service.Config.AccessGrant == "":
		return errs.New("Access Grant is not set")
	case service.Config.Bucket == "":
		return errs.New("Bucket is not set")
	}

	access, err := uplink.ParseAccess(service.Config.AccessGrant)
	if err != nil {
		return err
	}

	project, err := uplink.OpenProject(ctx, access)
	if err != nil {
		return err
	}
	defer func() {
		err = errs.Combine(err, project.Close())
	}()

	download, err := project.DownloadObject(ctx, service.Config.Bucket, bloomfilter.LATEST, nil)
	if err != nil {
		if errors.Is(err, uplink.ErrObjectNotFound) {
			service.log.Info("LATEST file does not exist in bucket", zap.String("bucket", service.Config.Bucket))
			return nil
		}
		return err
	}

	defer func() {
		err = errs.Combine(err, download.Close())
	}()

	value, err := io.ReadAll(download)
	if err != nil {
		return err
	}
	prefix := string(value) + "/"

	return IterateZipObjectKeys(ctx, *project, service.Config.Bucket, prefix, func(objectKey string) error {
		limiter := sync2.NewLimiter(service.Config.ConcurrentSends)
		err := IterateZipContent(ctx, *project, service.Config.Bucket, objectKey, func(zipEntry *zip.File) error {
			retainInfo, err := UnpackZipEntry(zipEntry)
			if err != nil {
				service.log.Warn("Skipping retain filter entry", zap.Error(err))
				return nil
			}
			limiter.Go(ctx, func() {
				err := service.sendRetainRequest(ctx, retainInfo)
				if err != nil {
					service.log.Warn("Error sending retain filter", zap.Stringer("NodeID", retainInfo.StorageNodeId), zap.Error(err))
				}
			})
			return nil
		})
		limiter.Wait()

		if err != nil {
			// We store the error in the bucket and then continue with the next zip file.
			return service.moveToErrorPrefix(ctx, project, objectKey, err)
		}

		return service.moveToSentPrefix(ctx, project, objectKey)
	})
}

func (service *Service) sendRetainRequest(ctx context.Context, retainInfo *internalpb.RetainInfo) (err error) {
	defer mon.Task()(&ctx)(&err)

	dossier, err := service.overlay.Get(ctx, retainInfo.StorageNodeId)
	if err != nil {
		return Error.Wrap(err)
	}

	// avoid sending bloom filters to disqualified and exited nodes
	if dossier.Disqualified != nil || dossier.ExitStatus.ExitSuccess {
		return nil
	}

	if service.Config.RetainSendTimeout > 0 {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, service.Config.RetainSendTimeout)
		defer cancel()
	}

	nodeurl := storj.NodeURL{
		ID:      retainInfo.StorageNodeId,
		Address: dossier.Address.Address,
	}

	client, err := piecestore.Dial(ctx, service.dialer, nodeurl, piecestore.DefaultConfig)
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, Error.Wrap(client.Close()))
	}()

	err = client.Retain(ctx, &pb.RetainRequest{
		CreationDate: retainInfo.CreationDate,
		Filter:       retainInfo.Filter,
	})
	return Error.Wrap(err)
}

// moveToErrorPrefix moves an object to prefix "error" and attaches the error to the metadata.
func (service *Service) moveToErrorPrefix(
	ctx context.Context, project *uplink.Project, objectKey string, previousErr error,
) error {
	newObjectKey := "error-" + objectKey

	err := project.MoveObject(ctx, service.Config.Bucket, objectKey, service.Config.Bucket, newObjectKey, nil)
	if err != nil {
		return err
	}

	return service.uploadError(ctx, project, newObjectKey+".error.txt", previousErr)
}

// uploadError saves an error under an object key.
func (service *Service) uploadError(
	ctx context.Context, project *uplink.Project, destinationObjectKey string, previousErr error,
) (err error) {
	upload, err := project.UploadObject(ctx, service.Config.Bucket, destinationObjectKey, &uplink.UploadOptions{
		Expires: time.Now().Add(service.Config.ExpireIn),
	})
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			err = errs.Combine(err, upload.Abort())
		}
	}()

	_, err = upload.Write([]byte(previousErr.Error()))
	if err != nil {
		return err
	}

	return upload.Commit()
}

// moveToSentPrefix moves an object to prefix "sent".
func (service *Service) moveToSentPrefix(
	ctx context.Context, project *uplink.Project, objectKey string,
) error {
	newObjectKey := "sent-" + objectKey

	return project.MoveObject(ctx, service.Config.Bucket, objectKey, service.Config.Bucket, newObjectKey, nil)
}
