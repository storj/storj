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

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/shared/nodeidmap"
	"storj.io/uplink"
)

// LATEST is the name of the file that contains the most recently completed bloomfilter generation prefix.
const LATEST = "LATEST"

// Upload is used to upload bloom filters to specified bucket.
type Upload struct {
	log    *zap.Logger
	config Config
}

// NewUpload creates new upload for bloom filters.
func NewUpload(log *zap.Logger, config Config) *Upload {
	return &Upload{
		log:    log,
		config: config,
	}
}

// CheckConfig check configuration values.
func (bfu *Upload) CheckConfig() error {
	switch {
	case bfu.config.AccessGrant == "":
		return errs.New("Access Grant is not set")
	case bfu.config.Bucket == "":
		return errs.New("Bucket is not set")
	}
	return nil
}

// UploadBloomFilters stores a zipfile with multiple bloom filters in a bucket.
func (bfu *Upload) UploadBloomFilters(ctx context.Context, creationDate time.Time, retainInfos nodeidmap.Map[*RetainInfo]) (err error) {
	defer mon.Task()(&ctx)(&err)

	if retainInfos.IsEmpty() {
		return nil
	}

	prefix := time.Now().Format(time.RFC3339Nano)

	expirationTime := time.Now().Add(bfu.config.ExpireIn)

	accessGrant, err := uplink.ParseAccess(bfu.config.AccessGrant)
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
			err = errs.Combine(err, bfu.cleanup(ctx, project, prefix))
		}
		err = errs.Combine(err, project.Close())
	}()

	_, err = project.EnsureBucket(ctx, bfu.config.Bucket)
	if err != nil {
		return err
	}

	// TODO move it before segment loop is started
	o := uplink.ListObjectsOptions{
		Prefix: prefix + "/",
	}
	iterator := project.ListObjects(ctx, bfu.config.Bucket, &o)
	for iterator.Next() {
		if iterator.Item().IsPrefix {
			continue
		}

		bfu.log.Warn("target bucket was not empty, stop operation and wait for next execution", zap.String("bucket", bfu.config.Bucket))
		return nil
	}

	infos := make([]internalpb.RetainInfo, 0, bfu.config.ZipBatchSize)
	batchNumber := 0
	retainInfos.Range(func(nodeID storj.NodeID, info *RetainInfo) bool {
		infos = append(infos, internalpb.RetainInfo{
			Filter: info.Filter.Bytes(),
			// because bloom filters should be created from immutable database
			// snapshot we are using latest segment creation date
			CreationDate:  creationDate,
			PieceCount:    int64(info.Count),
			StorageNodeId: nodeID,
		})

		if len(infos) == bfu.config.ZipBatchSize {
			err = bfu.uploadPack(ctx, project, prefix, batchNumber, expirationTime, infos)
			if err != nil {
				return false
			}

			infos = infos[:0]
			batchNumber++
		}

		return true
	})
	if err != nil {
		return err
	}

	// upload rest of infos if any
	if err := bfu.uploadPack(ctx, project, prefix, batchNumber, expirationTime, infos); err != nil {
		return err
	}

	// update LATEST file
	upload, err := project.UploadObject(ctx, bfu.config.Bucket, LATEST, nil)
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
func (bfu *Upload) uploadPack(ctx context.Context, project *uplink.Project, prefix string, batchNumber int, expirationTime time.Time, infos []internalpb.RetainInfo) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(infos) == 0 {
		return nil
	}

	upload, err := project.UploadObject(ctx, bfu.config.Bucket, prefix+"/bloomfilters-"+strconv.Itoa(batchNumber)+".zip", &uplink.UploadOptions{
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
func (bfu *Upload) cleanup(ctx context.Context, project *uplink.Project, prefix string) (err error) {
	defer mon.Task()(&ctx)(&err)

	errPrefix := "upload-error-" + time.Now().Format(time.RFC3339)
	o := uplink.ListObjectsOptions{
		Prefix: prefix + "/",
	}
	iterator := project.ListObjects(ctx, bfu.config.Bucket, &o)

	for iterator.Next() {
		item := iterator.Item()
		if item.IsPrefix {
			continue
		}

		err := project.MoveObject(ctx, bfu.config.Bucket, item.Key, bfu.config.Bucket, prefix+"/"+errPrefix+"/"+item.Key, nil)
		if err != nil {
			return err
		}
	}

	return iterator.Err()
}
