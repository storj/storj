// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package mobile

import (
	"fmt"
	"time"

	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
)

type ObjectInfo struct {
	Version     int32
	Bucket      string
	Path        string
	IsPrefix    bool
	Size        int64
	ContentType string
	Created     int64
	Modified    int64
	Expires     int64

	metadata map[string]string

	// object storj.Object
}

// func (bl *ObjectInfo) GetVersion() int32 {
// 	return int32(bl.object.Version)
// }

// func (bl *ObjectInfo) GetBucket() *BucketInfo {
// 	return &BucketInfo{bl.object.Bucket}
// }

// func (bl *ObjectInfo) GetPath() string {
// 	return bl.object.Path
// }

// func (bl *ObjectInfo) IsPrefix() bool {
// 	return bl.object.IsPrefix
// }

// func (bl *ObjectInfo) GetSize() int64 {
// 	return bl.object.Size
// }

func newObjectInfoFromObject(object storj.Object) *ObjectInfo {
	return &ObjectInfo{
		Version:     int32(object.Version),
		Bucket:      object.Bucket.Name,
		Path:        object.Path,
		IsPrefix:    object.IsPrefix,
		Size:        object.Size,
		ContentType: object.ContentType,
		Created:     object.Created.UTC().UnixNano() / int64(time.Millisecond),
		Modified:    object.Modified.UTC().UnixNano() / int64(time.Millisecond),
		Expires:     object.Expires.UTC().UnixNano() / int64(time.Millisecond),
		metadata:    object.Metadata,
	}
}

func newObjectInfoFromObjectMeta(objectMeta libuplink.ObjectMeta) *ObjectInfo {
	return &ObjectInfo{
		// TODO ObjectMeta doesn't have Version but storj.Object has
		// Version:     int32(objectMeta.Version),
		Bucket:      objectMeta.Bucket,
		Path:        objectMeta.Path,
		IsPrefix:    objectMeta.IsPrefix,
		Size:        objectMeta.Size,
		ContentType: objectMeta.ContentType,
		Created:     objectMeta.Created.UTC().UnixNano() / int64(time.Millisecond),
		Modified:    objectMeta.Modified.UTC().UnixNano() / int64(time.Millisecond),
		Expires:     objectMeta.Expires.UTC().UnixNano() / int64(time.Millisecond),
		metadata:    objectMeta.Metadata,
	}
}

func (bl *ObjectInfo) GetMetadata(key string) string {
	return bl.metadata[key]
}

// func (bl *ObjectInfo) GetContentType() string {
// 	return bl.object.ContentType
// }

// func (bl *ObjectInfo) GetCreated() int64 {
// 	return bl.object.Created.UTC().UnixNano() / int64(time.Millisecond)
// }

// func (bl *ObjectInfo) GetModified() int64 {
// 	return bl.object.Modified.UTC().UnixNano() / int64(time.Millisecond)
// }

// func (bl *ObjectInfo) GetExpires() int64 {
// 	return bl.object.Expires.UTC().UnixNano() / int64(time.Millisecond)
// }

type ObjectList struct {
	list storj.ObjectList
}

// More returns true if list request was not able to return all results
func (bl *ObjectList) More() bool {
	return bl.list.More
}

// Prefix
func (bl *ObjectList) Prefix() string {
	return string(bl.list.Prefix)
}

// Bucket returns bucket name
func (bl *ObjectList) Bucket() string {
	return bl.list.Bucket
}

// Length returns number of returned items
func (bl *ObjectList) Length() int {
	return len(bl.list.Items)
}

// Item gets item from specific index
func (bl *ObjectList) Item(index int) (*ObjectInfo, error) {
	if index < 0 && index >= len(bl.list.Items) {
		return nil, fmt.Errorf("index out of range")
	}
	return newObjectInfoFromObject(bl.list.Items[index]), nil
}
