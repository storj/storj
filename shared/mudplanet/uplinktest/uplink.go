// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package uplinktest

import (
	"bytes"
	"context"
	"io"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/uplink"
	"storj.io/uplink/private/metaclient"
	"storj.io/uplink/private/object"
)

var mon = monkit.Package()

// Uplink is a helper structure for tests, which wraps uplink.Access and uplink.Config
type Uplink struct {
	Config uplink.Config
	access *uplink.Access
}

// NewUplink creates new Uplink instance with predefined access grant and config.
func NewUplink(access *uplink.Access, config uplink.Config) (*Uplink, error) {
	return &Uplink{
		Config: config,
		access: access,
	}, nil
}

// OpenProject opens project with predefined access grant and gives access to pure uplink API.
func (client *Uplink) OpenProject(ctx context.Context) (_ *uplink.Project, err error) {
	return uplink.OpenProject(ctx, client.access)
}

// Upload data to specific satellite.
func (client *Uplink) Upload(ctx context.Context, bucket string, path storj.Path, data []byte) (err error) {
	defer mon.Task()(&ctx)(&err)
	return errs.Wrap(client.UploadWithExpiration(ctx, bucket, path, data, time.Time{}))
}

// UploadWithExpiration data to specific satellite and expiration time.
func (client *Uplink) UploadWithExpiration(ctx context.Context, bucketName string, key string, data []byte, expiration time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = client.UploadWithOptions(ctx, bucketName, key, data, &metaclient.UploadOptions{
		Expires: expiration,
	})
	return errs.Wrap(err)
}

// UploadWithOptions uploads data to specific satellite, with defined options.
func (client *Uplink) UploadWithOptions(ctx context.Context, bucketName, key string, data []byte, options *metaclient.UploadOptions) (obj *object.VersionedObject, err error) {
	defer mon.Task()(&ctx)(&err)

	project, err := client.GetProject(ctx)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, project.Close()) }()

	upload, err := object.UploadObject(ctx, project, bucketName, key, options)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	_, err = io.Copy(upload, bytes.NewReader(data))
	if err != nil {
		abortErr := upload.Abort()
		err = errs.Combine(err, abortErr)
		return nil, errs.Wrap(err)
	}

	err = upload.Commit()
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return upload.Info(), nil
}

// Download data from specific satellite.
func (client *Uplink) Download(ctx context.Context, bucketName string, path storj.Path) (_ []byte, err error) {
	defer mon.Task()(&ctx)(&err)

	project, err := client.GetProject(ctx)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, project.Close()) }()

	download, err := project.DownloadObject(ctx, bucketName, path, nil)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, download.Close()) }()

	data, err := io.ReadAll(download)
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

// Delete data from specific satellite.
func (client *Uplink) Delete(ctx context.Context, bucketName string, path storj.Path) (err error) {
	defer mon.Task()(&ctx)(&err)

	project, err := client.GetProject(ctx)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, project.Close()) }()

	_, err = project.DeleteObject(ctx, bucketName, path)
	if err != nil {
		return errs.Wrap(err)
	}

	return nil
}

// DeleteMany objects from specific satellite.
func (client *Uplink) DeleteMany(ctx context.Context, bucketName string, paths []storj.Path) (resultItems []object.DeleteObjectsResultItem, err error) {
	defer mon.Task()(&ctx)(&err)

	project, err := client.GetProject(ctx)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, project.Close()) }()

	items := make([]object.DeleteObjectsItem, len(paths))
	for i, path := range paths {
		items[i] = object.DeleteObjectsItem{ObjectKey: path}
	}

	resultItems, err = object.DeleteObjects(ctx, project, bucketName, items, nil)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return resultItems, nil
}

// Copy data between source and destination on specific satellite.
func (client *Uplink) Copy(ctx context.Context, srcBucket string, srcPath storj.Path, destBucket string, destPath storj.Path) (err error) {
	defer mon.Task()(&ctx)(&err)

	project, err := client.GetProject(ctx)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, project.Close()) }()

	_, err = project.CopyObject(ctx, srcBucket, srcPath, destBucket, destPath, nil)
	if err != nil {
		return errs.Wrap(err)
	}

	return nil
}

// Move data between source and destination on specific satellite.
func (client *Uplink) Move(ctx context.Context, srcBucket string, srcPath storj.Path, destBucket string, destPath storj.Path) (err error) {
	defer mon.Task()(&ctx)(&err)

	project, err := client.GetProject(ctx)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, project.Close()) }()

	err = project.MoveObject(ctx, srcBucket, srcPath, destBucket, destPath, nil)
	if err != nil {
		return errs.Wrap(err)
	}

	return nil
}

// GetProject returns a uplink.Project which allows interactions with a specific project.
func (client *Uplink) GetProject(ctx context.Context) (_ *uplink.Project, err error) {
	defer mon.Task()(&ctx)(&err)

	project, err := client.Config.OpenProject(ctx, client.access)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return project, nil
}
