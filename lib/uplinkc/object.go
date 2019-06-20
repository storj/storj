// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"

import (
	"io"
	"storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
	"time"
	"unsafe"
)

// Object is a scoped uplink.Object
type Object struct {
	scope
	*uplink.Object
}

// open_object returns an Object handle, if authorized.
//export open_object
func open_object(bucketHandle C.BucketRef, objectPath *C.char, cerr **C.char) C.ObjectRef {
	bucket, ok := universe.Get(bucketHandle._handle).(*Bucket)
	if !ok {
		*cerr = C.CString("invalid bucket")
		return C.ObjectRef{}
	}

	scope := bucket.scope.child()

	object, err := bucket.OpenObject(scope.ctx, C.GoString(objectPath))
	if err != nil {
		*cerr = C.CString(err.Error())
		return C.ObjectRef{}
	}

	return C.ObjectRef{universe.Add(&Object{scope, object})}
}

// close_object closes the object.
//export close_object
func close_object(objectHandle C.ObjectRef, cerr **C.char) {
	object, ok := universe.Get(objectHandle._handle).(*Object)
	if !ok {
		*cerr = C.CString("invalid object")
		return
	}

	universe.Del(objectHandle._handle)
	defer object.cancel()

	if err := object.Close(); err != nil {
		*cerr = C.CString(err.Error())
		return
	}
}

// get_object_meta returns the object meta which contains metadata about a specific Object.
//export get_object_meta
func get_object_meta(cObject C.ObjectRef, cErr **C.char) C.ObjectMeta {
	object, ok := universe.Get(cObject._handle).(*Object)
	if !ok {
		*cErr = C.CString("invalid object")
		return C.ObjectMeta{}
	}

	checksum, checksumLen := bytes_to_cbytes(object.Meta.Checksum)

	mapRef := new_map_ref()
	for k, v := range object.Meta.Metadata {
		map_ref_set(mapRef, C.CString(k), C.CString(v), cErr)
		if C.GoString(*cErr) != "" {
			return C.ObjectMeta{}
		}
	}

	return C.ObjectMeta {
		bucket: C.CString(object.Meta.Bucket),
		path:  C.CString(object.Meta.Path),
		is_prefix: C.bool(object.Meta.IsPrefix),
		content_type:  C.CString(object.Meta.ContentType),
		meta_data: mapRef,
		created: C.int64_t(object.Meta.Created.Unix()),
		modified: C.int64_t(object.Meta.Modified.Unix()),
		expires: C.int64_t(object.Meta.Expires.Unix()),
		size: C.uint64_t(object.Meta.Size),
		checksum_bytes: checksum,
		checksum_length: checksumLen,
	}
}

// free_object_meta frees the object meta
//export free_object_meta
func free_object_meta(objectMeta *C.ObjectMeta) {
	C.free(unsafe.Pointer(objectMeta.bucket))
	objectMeta.bucket = nil

	C.free(unsafe.Pointer(objectMeta.path))
	objectMeta.path = nil

	C.free(unsafe.Pointer(objectMeta.content_type))
	objectMeta.content_type = nil

	C.free(unsafe.Pointer(objectMeta.checksum_bytes))
	objectMeta.checksum_bytes = nil

	universe.Del(objectMeta.meta_data._handle)
}

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
		var metadata *MapRef
		if cOpts.metadata._handle != 0 {
			metadata, ok = universe.Get(cOpts.metadata._handle).(*MapRef)
			if !ok {
				*cErr = C.CString("invalid metadata in upload options")
				return
			}
		}

		opts = &uplink.UploadOptions{
			ContentType: C.GoString(cOpts.content_type),
			Metadata:    metadata.m,
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
		wc: writeCloser,
	})}
}

//export free_upload_opts
func free_upload_opts(uploadOpts *C.UploadOptions) {
	C.free(unsafe.Pointer(uploadOpts.content_type))
	uploadOpts.content_type = nil

	universe.Del(uploadOpts.metadata._handle)
}

//export upload_write
func upload_write(uploader C.UploaderRef, bytes *C.uint8_t, length C.int, cErr **C.char) (writeLength C.int) {
	upload, ok := universe.Get(uploader._handle).(*Upload)
	if !ok {
		*cErr = C.CString("invalid uploader")
		return C.int(0)
	}

	buf := (*[1<<30]byte)(unsafe.Pointer(bytes))[:length]

	n, err := upload.wc.Write(buf)
	if err == io.EOF {
		return C.EOF
	}

	return C.int(n)
}

//export upload_close
func upload_close(uploader C.UploaderRef, cErr **C.char) {
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


// list_objects lists objects a user is authorized to see.
//export list_objects
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
			Prefix: C.GoString(cListOpts.cursor),
			Cursor: C.GoString(cListOpts.cursor),
			Delimiter: rune(cListOpts.delimiter),
			Recursive: bool(cListOpts.recursive),
			Direction: storj.ListDirection(cListOpts.direction),
			Limit: int(cListOpts.limit),
		}
	}

	objectList, err := bucket.ListObjects(scope.ctx, opts)
	if err != nil {
		*cErr = C.CString(err.Error())
		return cObjList
	}
	objListLen := len(objectList.Items)

	objectSize := int(C.sizeof_ObjectInfo)
	ptr := C.malloc(C.size_t(objListLen * objectSize))
	cObjectsPtr := (*[1 << 30]C.ObjectInfo)(ptr)

	for i, object := range objectList.Items {
		(*cObjectsPtr)[i] = newObjectInfo(&object)
	}

	return C.ObjectList{
		bucket: C.CString(objectList.Bucket),
		prefix: C.CString(objectList.Prefix),
		more: C.bool(objectList.More),
		items:  (*C.ObjectInfo)(unsafe.Pointer(cObjectsPtr)),
		length: C.int32_t(objListLen),
	}
}

// free_object_info frees the object info
//export free_object_info
func free_object_info(objectInfo *C.ObjectInfo) {
	bucketInfo := objectInfo.bucket
	free_bucket_info(&bucketInfo)

	C.free(unsafe.Pointer(objectInfo.path))
	objectInfo.path = nil

	C.free(unsafe.Pointer(objectInfo.content_type))
	objectInfo.content_type = nil

	delete_map_ref(objectInfo.metadata)
}

// free_list_objects frees the list of objects
//export free_list_objects
func free_list_objects(objectList *C.ObjectList) {
	C.free(unsafe.Pointer(objectList.bucket))
	objectList.bucket = nil

	C.free(unsafe.Pointer(objectList.prefix))
	objectList.prefix = nil

	for i := 0; i < int(objectList.length); i++ {
		item := int(uintptr(unsafe.Pointer(objectList.items))) + i * int(unsafe.Sizeof(C.ObjectInfo{}))
		free_object_info((*C.ObjectInfo)(unsafe.Pointer(uintptr(item))))
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
		rc: rc,
	})}
}

//export download_read
func download_read(downloader C.DownloaderRef, bytes *C.uint8_t, length C.int, cErr **C.char) (readLength C.int) {
	download, ok := universe.Get(downloader._handle).(*Download)
	if !ok {
		*cErr = C.CString("invalid downloader")
		return C.int(0)
	}

	var result [1024]byte
	buf := (*[1<<30]byte)(unsafe.Pointer(bytes))[:length]

	n, err := download.rc.Read(result[:])
	copy(buf, result[:n])
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
