// +build ignore

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import "C"
import (
	"context"
	"io"
	"storj.io/storj/internal/readcloser"
	"storj.io/storj/lib/uplink"
)

//export CloseObject
func CloseObject(cObject CObjectRef, cErr *CCharPtr) {
	object, ok := universe.Get(Token(cObject)).(*uplink.Object)
	if !ok {
		*cErr = CCString("invalid object")
		return
	}

	if err := object.Close(); err != nil {
		*cErr = CCString(err.Error())
		return
	}

	universe.Del(Token(cObject))
}

//export DownloadRange
func DownloadRange(cObject CObjectRef, offset CInt64, length CInt64, cErr *CCharPtr) (downloader CDownloadReaderRef) {
	ctx := context.Background()

	object, ok := universe.Get(Token(cObject)).(*uplink.Object)
	if !ok {
		*cErr = CCString("invalid object")
		return downloader
	}

	rc, err := object.DownloadRange(ctx, int64(offset), int64(length))
	if err != nil {
		*cErr = CCString(err.Error())
		return downloader
	}

	return CDownloadReaderRef(universe.Add(rc))
}

//export Download
func Download(downloader CDownloadReaderRef, bytes *CBytes, cErr *CCharPtr) (readLength CInt) {
	readCloser, ok := universe.Get(Token(downloader)).(*readcloser.LimitedReadCloser)
	if !ok {
		*cErr = CCString("invalid reader")
		return CInt(0)
	}

	// TODO: This size could be optimized
	buf := make([]byte, 1024)

	n, err := readCloser.Read(buf)
	if err == io.EOF {
		readCloser.Close()
		return CEOF
	}

	bytesToCbytes(buf, n, bytes)

	return CInt(n)
}

//export ObjectMeta
func ObjectMeta(cObject CObjectRef, cErr *CCharPtr) (objectMeta CObjectMeta) {
	object, ok := universe.Get(Token(cObject)).(*uplink.Object)
	if !ok {
		*cErr = CCString("invalid object")
		return objectMeta
	}

	bytes := new(CBytes)
	bytesToCbytes(object.Meta.Checksum, len(object.Meta.Checksum), bytes)

	return CObjectMeta{
		Bucket:      CCString(object.Meta.Bucket),
		Path:        CCString(object.Meta.Path),
		IsPrefix:    CBool(object.Meta.IsPrefix),
		ContentType: CCString(object.Meta.ContentType),
		MetaData:    NewMapRef(),
		Created:     CUint64(object.Meta.Created.Unix()),
		Modified:    CUint64(object.Meta.Modified.Unix()),
		Expires:     CUint64(object.Meta.Expires.Unix()),
		Size:        CUint64(object.Meta.Size),
		Checksum:    *bytes,
	}
}
