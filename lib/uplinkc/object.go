// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"

import (
	"io"
	"time"
	"unsafe"

	"storj.io/storj/lib/uplink"
)

type Upload struct {
	scope
	wc io.WriteCloser // 🤔
}

// upload uploads a new object, if authorized.
//export upload
func upload(cBucket C.BucketRef, path *C.char, cOpts *C.UploadOptions, cErr **C.char) (downloader C.UploaderRef) {
	bucket, ok := universe.Get(cBucket._handle).(*Bucket)
	if !ok {
		*cErr = C.CString("invalid bucket")
		return
	}

	scope := bucket.scope.child()

	var opts *uplink.UploadOptions
	if cOpts != nil {
		var metadata map[string]string

		opts = &uplink.UploadOptions{
			ContentType: C.GoString(cOpts.content_type),
			Metadata:    metadata,
			Expires:     time.Unix(int64(cOpts.expires), 0),
		}
	}

	writeCloser, err := bucket.NewWriter(scope.ctx, C.GoString(path), opts)
	if err != nil {
		*cErr = C.CString(err.Error())
		return
	}

	return C.UploaderRef{universe.Add(&Upload{
		scope: scope,
		wc:    writeCloser,
	})}
}

//export upload_write
func upload_write(uploader C.UploaderRef, bytes *C.uint8_t, length C.int, cErr **C.char) (writeLength C.int) {
	upload, ok := universe.Get(uploader._handle).(*Upload)
	if !ok {
		*cErr = C.CString("invalid uploader")
		return C.int(0)
	}

	buf := (*[1 << 30]byte)(unsafe.Pointer(bytes))[:length]

	n, err := upload.wc.Write(buf)
	if err == io.EOF {
		return C.EOF
	}

	return C.int(n)
}

//export upload_commit
func upload_commit(uploader C.UploaderRef, cErr **C.char) {
	upload, ok := universe.Get(uploader._handle).(*Upload)
	if !ok {
		*cErr = C.CString("invalid uploader")
	}

	universe.Del(uploader._handle)
	defer upload.cancel()

	err := upload.wc.Close()
	if err != nil {
		*cErr = C.CString(err.Error())
		return
	}
}

type Download struct {
	scope
	rc interface {
		io.Reader
		io.Seeker
		io.Closer
	}
}

// download returns an Object's data. A length of -1 will mean
// (Object.Size - offset).
//export download
func download(bucketRef C.BucketRef, path *C.char, cErr **C.char) (downloader C.DownloaderRef) {
	bucket, ok := universe.Get(bucketRef._handle).(*Bucket)
	if !ok {
		*cErr = C.CString("invalid bucket")
		return
	}

	scope := bucket.scope.child()

	rc, err := bucket.NewReader(scope.ctx, C.GoString(path))
	if err != nil {
		*cErr = C.CString(err.Error())
		return
	}

	return C.DownloaderRef{universe.Add(&Download{
		scope: scope,
		rc:    rc,
	})}
}

//export download_read
func download_read(downloader C.DownloaderRef, bytes *C.uint8_t, length C.int, cErr **C.char) (readLength C.int) {
	download, ok := universe.Get(downloader._handle).(*Download)
	if !ok {
		*cErr = C.CString("invalid downloader")
		return C.int(0)
	}

	buf := (*[1 << 30]byte)(unsafe.Pointer(bytes))[:length]

	n, err := download.rc.Read(buf)
	if err == io.EOF {
		return C.EOF
	}

	return C.int(n)
}

//export download_close
func download_close(downloader C.DownloaderRef, cErr **C.char) {
	download, ok := universe.Get(downloader._handle).(*Download)
	if !ok {
		*cErr = C.CString("invalid downloader")
	}

	universe.Del(downloader._handle)
	defer download.cancel()

	err := download.rc.Close()
	if err != nil {
		*cErr = C.CString(err.Error())
		return
	}
}

//export free_upload_opts
func free_upload_opts(uploadOpts *C.UploadOptions) {
	C.free(unsafe.Pointer(uploadOpts.content_type))
	uploadOpts.content_type = nil
}
