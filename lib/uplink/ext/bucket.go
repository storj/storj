// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import "C"
import (
	"context"
	"time"
	"unsafe"

	"storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
)

//export OpenObject
func OpenObject(cBucket CBucketRef, cpath CCharPtr, cErr *CCharPtr) (objectRef CObjectRef) {
	ctx := context.Background()

	bucket, ok := structRefMap.Get(Token(cBucket)).(*uplink.Bucket)
	if !ok {
		*cErr = CCString("invalid bucket")
		return objectRef
	}

	path := storj.JoinPaths(CGoString(cpath))
	object, err := bucket.OpenObject(ctx, path)
	if err != nil {
		*cErr = CCString(err.Error())
		return objectRef
	}

	return CObjectRef(structRefMap.Add(object))
}

//export UploadObject
func UploadObject(cBucket CBucketRef, path CCharPtr, reader *CFile, cOpts *CUploadOptions, cErr *CCharPtr) {
	ctx := context.Background()

	bucket, ok := structRefMap.Get(Token(cBucket)).(*uplink.Bucket)
	if !ok {
		*cErr = CCString("invalid bucket")
		return
	}

	// TODO: should `unsafe.Pointer(cOpts) == nil` be an error?
	// TODO: fix ^
	var metadata map[string]string
	if uintptr(cOpts.metadata) != 0 {
		metadata, ok = structRefMap.Get(Token(cOpts.metadata)).(map[string]string)
		if !ok {
			*cErr = CCString("invalid metadata in upload options")
			return
		}
	}

	var opts *uplink.UploadOptions
	if cOpts != nil {
		opts = &uplink.UploadOptions{
			ContentType: CGoString(cOpts.content_type),
			Metadata:    metadata,
			Expires:     time.Unix(int64(cOpts.expires), 0),
		}
	}

	if err := bucket.UploadObject(ctx, CGoString(path), reader, opts); err != nil {
		*cErr = CCString(err.Error())
		return
	}
}

//export ListObjects
func ListObjects(bucketRef CBucketRef, cListOpts *CObjectListOptions, cErr *CCharPtr) (cObjList CObjectList) {
	ctx := context.Background()

	bucket, ok := structRefMap.Get(Token(bucketRef)).(*uplink.Bucket)
	if !ok {
		*cErr = CCString("invalid bucket")
		return cObjList
	}

	var opts *uplink.ObjectListOptions
	if unsafe.Pointer(cListOpts) != nil {
		opts = &uplink.ObjectListOptions{
			Prefix:    CGoString(cListOpts.cursor),
			Cursor:    CGoString(cListOpts.cursor),
			Delimiter: rune(cListOpts.delimiter),
			Recursive: bool(cListOpts.recursive),
			Direction: storj.ListDirection(cListOpts.direction),
			Limit:     int(cListOpts.limit),
		}
	}

	objectList, err := bucket.ListObjects(ctx, opts)
	if err != nil {
		*cErr = CCString(err.Error())
		return cObjList
	}
	objListLen := len(objectList.Items)

	objectSize := int(unsafe.Sizeof(CObject{}))
	// TODO: use `calloc` instead?
	cObjectsPtr := GoCMalloc(uintptr(objListLen * objectSize))

	for i, object := range objectList.Items {
		cObject := (*CObject)(unsafe.Pointer(uintptr(int(cObjectsPtr) + (i * objectSize))))
		*cObject = NewCObject(&object)
	}

	return CObjectList{
		bucket: CCString(objectList.Bucket),
		prefix: CCString(objectList.Prefix),
		more:   CBool(objectList.More),
		items:  (*CObject)(unsafe.Pointer(cObjectsPtr)),
		length: CInt32(objListLen),
	}
}

//export CloseBucket
func CloseBucket(bucketRef CBucketRef, cErr *CCharPtr) {
	bucket, ok := structRefMap.Get(Token(bucketRef)).(*uplink.Bucket)
	if !ok {
		*cErr = CCString("invalid bucket")
		return
	}

	if err := bucket.Close(); err != nil {
		*cErr = CCString(err.Error())
		return
	}

	structRefMap.Del(Token(bucketRef))
}

func NewCObject(object *storj.Object) CObject {
	return CObject{
		version:      CUint32(object.Version),
		bucket:       NewCBucket(&object.Bucket),
		path:         CCString(object.Path),
		is_prefix:    CBool(object.IsPrefix),
		metadata:     CMapRef(structRefMap.Add(object.Metadata)),
		content_type: CCString(object.ContentType),
		// TODO: use `UnixNano()`?
		created:  CTime(object.Created.Unix()),
		modified: CTime(object.Modified.Unix()),
		expires:  CTime(object.Expires.Unix()),
	}
}
