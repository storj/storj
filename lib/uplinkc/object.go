// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"

import (
	"fmt"
	"io"
	"reflect"
	"time"
	"unsafe"

	"storj.io/common/errs2"
	"storj.io/common/storj"
	"storj.io/storj/lib/uplink"
)

// Object is a scoped uplink.Object
type Object struct {
	scope
	*uplink.Object
}

//export open_object
// open_object returns an Object handle, if authorized.
func open_object(bucketHandle C.BucketRef, objectPath *C.char, cerr **C.char) C.ObjectRef {
	bucket, ok := universe.Get(bucketHandle._handle).(*Bucket)
	if !ok {
		*cerr = C.CString("invalid bucket")
		return C.ObjectRef{}
	}

	scope := bucket.scope.child()

	object, err := bucket.OpenObject(scope.ctx, C.GoString(objectPath))
	if err != nil {
		*cerr = C.CString(fmt.Sprintf("%+v", err))
		return C.ObjectRef{}
	}

	return C.ObjectRef{universe.Add(&Object{scope, object})}
}

//export close_object
// close_object closes the object.
func close_object(objectHandle C.ObjectRef, cerr **C.char) {
	object, ok := universe.Get(objectHandle._handle).(*Object)
	if !ok {
		*cerr = C.CString("invalid object")
		return
	}

	universe.Del(objectHandle._handle)
	defer object.cancel()

	if err := object.Close(); err != nil {
		*cerr = C.CString(fmt.Sprintf("%+v", err))
		return
	}
}

//export get_object_meta
// get_object_meta returns the object meta which contains metadata about a specific Object.
func get_object_meta(cObject C.ObjectRef, cErr **C.char) C.ObjectMeta {
	object, ok := universe.Get(cObject._handle).(*Object)
	if !ok {
		*cErr = C.CString("invalid object")
		return C.ObjectMeta{}
	}

	checksumLen := len(object.Meta.Checksum)
	checksumPtr := C.malloc(C.size_t(checksumLen))

	checksum := *(*[]byte)(unsafe.Pointer(
		&reflect.SliceHeader{
			Data: uintptr(checksumPtr),
			Len:  checksumLen,
			Cap:  checksumLen,
		},
	))
	copy(checksum, object.Meta.Checksum)

	return C.ObjectMeta{
		bucket:          C.CString(object.Meta.Bucket),
		path:            C.CString(object.Meta.Path),
		is_prefix:       C.bool(object.Meta.IsPrefix),
		content_type:    C.CString(object.Meta.ContentType),
		created:         C.int64_t(object.Meta.Created.Unix()),
		modified:        C.int64_t(object.Meta.Modified.Unix()),
		expires:         C.int64_t(object.Meta.Expires.Unix()),
		size:            C.uint64_t(object.Meta.Size),
		checksum_bytes:  (*C.uint8_t)(checksumPtr),
		checksum_length: C.uint64_t(checksumLen),
	}
}

// Upload stores writecloser and context scope for uploading
type Upload struct {
	scope
	wc io.WriteCloser // 🚽
}

//export upload
// upload uploads a new object, if authorized.
func upload(cBucket C.BucketRef, path *C.char, cOpts *C.UploadOptions, cErr **C.char) C.UploaderRef {
	bucket, ok := universe.Get(cBucket._handle).(*Bucket)
	if !ok {
		*cErr = C.CString("invalid bucket")
		return C.UploaderRef{}
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
		*cErr = C.CString(fmt.Sprintf("%+v", err))
		return C.UploaderRef{}
	}

	return C.UploaderRef{universe.Add(&Upload{
		scope: scope,
		wc:    writeCloser,
	})}
}

//export upload_write
func upload_write(uploader C.UploaderRef, bytes *C.uint8_t, length C.size_t, cErr **C.char) C.size_t {
	upload, ok := universe.Get(uploader._handle).(*Upload)
	if !ok {
		*cErr = C.CString("invalid uploader")
		return C.size_t(0)
	}

	if err := upload.ctx.Err(); err != nil {
		if !errs2.IsCanceled(err) {
			*cErr = C.CString(fmt.Sprintf("%+v", err))
		}
		return C.size_t(0)
	}

	buf := *(*[]byte)(unsafe.Pointer(
		&reflect.SliceHeader{
			Data: uintptr(unsafe.Pointer(bytes)),
			Len:  int(length),
			Cap:  int(length),
		},
	))

	n, err := upload.wc.Write(buf)
	if err != nil {
		if !errs2.IsCanceled(err) {
			*cErr = C.CString(fmt.Sprintf("%+v", err))
		}
	}
	return C.size_t(n)
}

//export upload_commit
func upload_commit(uploader C.UploaderRef, cErr **C.char) {
	upload, ok := universe.Get(uploader._handle).(*Upload)
	if !ok {
		*cErr = C.CString("invalid uploader")
		return
	}

	if err := upload.ctx.Err(); err != nil {
		if !errs2.IsCanceled(err) {
			*cErr = C.CString(fmt.Sprintf("%+v", err))
		}
		return
	}
	defer upload.cancel()

	err := upload.wc.Close()
	if err != nil {
		if !errs2.IsCanceled(err) {
			*cErr = C.CString(fmt.Sprintf("%+v", err))
		}
		return
	}
}

//export upload_cancel
func upload_cancel(uploader C.UploaderRef, cErr **C.char) {
	upload, ok := universe.Get(uploader._handle).(*Upload)
	if !ok {
		*cErr = C.CString("invalid uploader")
		return
	}

	if err := upload.ctx.Err(); err != nil {
		if !errs2.IsCanceled(err) {
			*cErr = C.CString(fmt.Sprintf("%+v", err))
		}
		return
	}

	upload.cancel()
}

//export list_objects
// list_objects lists objects a user is authorized to see.
func list_objects(bucketRef C.BucketRef, cListOpts *C.ListOptions, cErr **C.char) (cObjList C.ObjectList) {
	bucket, ok := universe.Get(bucketRef._handle).(*Bucket)
	if !ok {
		*cErr = C.CString("invalid bucket")
		return cObjList
	}

	scope := bucket.scope.child()

	var opts *uplink.ListOptions
	if unsafe.Pointer(cListOpts) != nil {
		opts = &uplink.ListOptions{
			Prefix:    C.GoString(cListOpts.prefix),
			Cursor:    C.GoString(cListOpts.cursor),
			Delimiter: rune(cListOpts.delimiter),
			Recursive: bool(cListOpts.recursive),
			Direction: storj.ListDirection(cListOpts.direction),
			Limit:     int(cListOpts.limit),
		}
	}

	objectList, err := bucket.ListObjects(scope.ctx, opts)
	if err != nil {
		*cErr = C.CString(fmt.Sprintf("%+v", err))
		return cObjList
	}
	objListLen := len(objectList.Items)

	objectSize := int(C.sizeof_ObjectInfo)
	ptr := C.malloc(C.size_t(objectSize * objListLen))
	cObjectsPtr := *(*[]C.ObjectInfo)(unsafe.Pointer(
		&reflect.SliceHeader{
			Data: uintptr(ptr),
			Len:  objListLen,
			Cap:  objListLen,
		},
	))

	for i, object := range objectList.Items {
		object := object
		cObjectsPtr[i] = newObjectInfo(&object)
	}

	return C.ObjectList{
		bucket: C.CString(objectList.Bucket),
		prefix: C.CString(objectList.Prefix),
		more:   C.bool(objectList.More),
		items:  (*C.ObjectInfo)(ptr),
		length: C.int32_t(objListLen),
	}
}

// Download stores readcloser and context scope for downloading
type Download struct {
	scope
	rc io.ReadCloser
}

//export download
// download returns an Object's data. A length of -1 will mean
// (Object.Size - offset).
func download(bucketRef C.BucketRef, path *C.char, cErr **C.char) C.DownloaderRef {
	bucket, ok := universe.Get(bucketRef._handle).(*Bucket)
	if !ok {
		*cErr = C.CString("invalid bucket")
		return C.DownloaderRef{}
	}

	scope := bucket.scope.child()

	rc, err := bucket.Download(scope.ctx, C.GoString(path))
	if err != nil {
		if !errs2.IsCanceled(err) {
			*cErr = C.CString(fmt.Sprintf("%+v", err))
		}
		return C.DownloaderRef{}
	}

	return C.DownloaderRef{universe.Add(&Download{
		scope: scope,
		rc:    rc,
	})}
}

//export download_range
// download_range returns an Object's data from specified range
func download_range(bucketRef C.BucketRef, path *C.char, start, limit int64, cErr **C.char) C.DownloaderRef {
	bucket, ok := universe.Get(bucketRef._handle).(*Bucket)
	if !ok {
		*cErr = C.CString("invalid bucket")
		return C.DownloaderRef{}
	}

	scope := bucket.scope.child()

	rc, err := bucket.DownloadRange(scope.ctx, C.GoString(path), start, limit)
	if err != nil {
		if !errs2.IsCanceled(err) {
			*cErr = C.CString(fmt.Sprintf("%+v", err))
		}
		return C.DownloaderRef{}
	}

	return C.DownloaderRef{universe.Add(&Download{
		scope: scope,
		rc:    rc,
	})}
}

//export download_read
// download_read reads data upto `length` bytes into `bytes` buffer and returns
// the count of bytes read. The exact number of bytes returned depends on different
// buffers and what is currently available.
// When there is no more data available function returns 0.
// On an error cErr is set, however some data may still be returned.
func download_read(downloader C.DownloaderRef, bytes *C.uint8_t, length C.size_t, cErr **C.char) C.size_t {
	download, ok := universe.Get(downloader._handle).(*Download)
	if !ok {
		*cErr = C.CString("invalid downloader")
		return C.size_t(0)
	}

	if err := download.ctx.Err(); err != nil {
		if !errs2.IsCanceled(err) {
			*cErr = C.CString(fmt.Sprintf("%+v", err))
		}
		return C.size_t(0)
	}

	buf := *(*[]byte)(unsafe.Pointer(
		&reflect.SliceHeader{
			Data: uintptr(unsafe.Pointer(bytes)),
			Len:  int(length),
			Cap:  int(length),
		},
	))

	n, err := download.rc.Read(buf)
	if err != nil && err != io.EOF && !errs2.IsCanceled(err) {
		*cErr = C.CString(fmt.Sprintf("%+v", err))
	}
	return C.size_t(n)
}

//export download_close
func download_close(downloader C.DownloaderRef, cErr **C.char) {
	download, ok := universe.Get(downloader._handle).(*Download)
	if !ok {
		*cErr = C.CString("invalid downloader")
	}

	if err := download.ctx.Err(); err != nil {
		if !errs2.IsCanceled(err) {
			*cErr = C.CString(fmt.Sprintf("%+v", err))
		}
		return
	}

	defer download.cancel()

	err := download.rc.Close()
	if err != nil {
		if !errs2.IsCanceled(err) {
			*cErr = C.CString(fmt.Sprintf("%+v", err))
		}
		return
	}
}

//export download_cancel
func download_cancel(downloader C.DownloaderRef, cErr **C.char) {
	download, ok := universe.Get(downloader._handle).(*Download)
	if !ok {
		*cErr = C.CString("invalid downloader")
		return
	}

	if err := download.ctx.Err(); err != nil {
		if !errs2.IsCanceled(err) {
			*cErr = C.CString(fmt.Sprintf("%+v", err))
		}
		return
	}

	download.cancel()
}

//export delete_object
func delete_object(bucketRef C.BucketRef, path *C.char, cerr **C.char) {
	bucket, ok := universe.Get(bucketRef._handle).(*Bucket)
	if !ok {
		*cerr = C.CString("invalid downloader")
		return
	}

	if err := bucket.DeleteObject(bucket.ctx, C.GoString(path)); err != nil {
		*cerr = C.CString(fmt.Sprintf("%+v", err))
		return
	}
}

//export free_uploader
// free_uploader deletes the uploader reference from the universe
func free_uploader(uploader C.UploaderRef) {
	universe.Del(uploader._handle)
}

//export free_downloader
// free_downloader deletes the downloader reference from the universe
func free_downloader(downloader C.DownloaderRef) {
	universe.Del(downloader._handle)
}

//export free_upload_opts
func free_upload_opts(uploadOpts *C.UploadOptions) {
	C.free(unsafe.Pointer(uploadOpts.content_type))
	uploadOpts.content_type = nil
}

//export free_object_meta
// free_object_meta frees the object meta
func free_object_meta(objectMeta *C.ObjectMeta) {
	C.free(unsafe.Pointer(objectMeta.bucket))
	objectMeta.bucket = nil

	C.free(unsafe.Pointer(objectMeta.path))
	objectMeta.path = nil

	C.free(unsafe.Pointer(objectMeta.content_type))
	objectMeta.content_type = nil

	C.free(unsafe.Pointer(objectMeta.checksum_bytes))
	objectMeta.checksum_bytes = nil
	objectMeta.checksum_length = 0
}

//export free_object_info
// free_object_info frees the object info
func free_object_info(objectInfo *C.ObjectInfo) {
	bucketInfo := objectInfo.bucket
	free_bucket_info(&bucketInfo)

	C.free(unsafe.Pointer(objectInfo.path))
	objectInfo.path = nil

	C.free(unsafe.Pointer(objectInfo.content_type))
	objectInfo.content_type = nil
}

//export free_list_objects
// free_list_objects frees the list of objects
func free_list_objects(objectList *C.ObjectList) {
	C.free(unsafe.Pointer(objectList.bucket))
	objectList.bucket = nil

	C.free(unsafe.Pointer(objectList.prefix))
	objectList.prefix = nil

	items := *(*[]C.ObjectInfo)(unsafe.Pointer(
		&reflect.SliceHeader{
			Data: uintptr(unsafe.Pointer(objectList.items)),
			Len:  int(objectList.length),
			Cap:  int(objectList.length),
		},
	))
	for i := range items {
		free_object_info(&items[i])
	}
	C.free(unsafe.Pointer(objectList.items))
	objectList.items = nil
	objectList.length = 0
}
