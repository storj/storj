// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package sender

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"strings"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/storj/satellite/internalpb"
	"storj.io/uplink"
)

// IterateZipObjectKeys checks inside the top-level of a bucket and yields the keys which look like zip files.
func IterateZipObjectKeys(
	ctx context.Context,
	project uplink.Project,
	bucket string,
	prefix string,
	fn func(objectKey string) error,
) (err error) {
	defer mon.Task()(&ctx)(&err)

	objects := project.ListObjects(ctx, bucket, &uplink.ListObjectsOptions{
		System:    true,
		Recursive: false,
		Prefix:    prefix,
	})

	for objects.Next() {
		object := objects.Item()
		if object.IsPrefix {
			continue
		}

		if !strings.HasSuffix(object.Key, ".zip") {
			continue
		}

		if err := fn(object.Key); err != nil {
			return err
		}
	}

	return objects.Err()
}

// IterateZipContent opens a zip file at an object key and yields the files inside.
func IterateZipContent(
	ctx context.Context,
	project uplink.Project,
	bucket string,
	objectKey string,
	fn func(file *zip.File) error,
) (err error) {
	download, err := project.DownloadObject(ctx, bucket, objectKey, nil)
	if err != nil {
		return err
	}

	defer func() {
		err = errs.Combine(err, download.Close())
	}()

	zipContents, err := io.ReadAll(download)
	if err != nil {
		return err
	}

	reader, err := zip.NewReader(bytes.NewReader(zipContents), int64(len(zipContents)))
	if err != nil {
		return err
	}

	for _, file := range reader.File {
		err = fn(file)
		if err != nil {
			return err
		}
	}

	return nil
}

// UnpackZipEntry deserialized a retainInfo struct from a single file inside a zip file.
func UnpackZipEntry(
	file *zip.File,
) (retainInfo *internalpb.RetainInfo, err error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}

	defer func() {
		err = errs.Combine(err, reader.Close())
	}()

	uncompressedBytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	retainInfo = &internalpb.RetainInfo{}
	err = pb.Unmarshal(uncompressedBytes, retainInfo)
	if err != nil {
		return nil, err
	}

	err = validate(file.Name, *retainInfo)
	if err != nil {
		return nil, err
	}

	return retainInfo, nil
}

func validate(fileName string, retainInfo internalpb.RetainInfo) error {
	if retainInfo.StorageNodeId.IsZero() {
		return errs.New("Storage Node ID is missing from file %s", fileName)
	}

	if fileName != retainInfo.StorageNodeId.String() {
		return errs.New("Storage Node ID %s is not equal to file name %s", retainInfo.StorageNodeId.String(), fileName)
	}

	if retainInfo.PieceCount == 0 {
		return errs.New("Retain filter count is zero for storage node %s", retainInfo.StorageNodeId.String())
	}

	if len(retainInfo.Filter) == 0 {
		return errs.New("Retain filter is missing for storage node %s", retainInfo.StorageNodeId.String())
	}

	if len(bytes.Trim(retainInfo.Filter, "\x00")) == 0 {
		return errs.New("Retain filter is zeroes for storage node %s", retainInfo.StorageNodeId.String())
	}

	year2020 := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if retainInfo.CreationDate.Before(year2020) {
		return errs.New("Retain filter creation date is too long ago: %s", retainInfo.CreationDate.String())
	}

	return nil
}
