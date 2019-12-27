// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package mobile

import (
	"fmt"
	"time"

	"storj.io/common/storj"
	libuplink "storj.io/storj/lib/uplink"
)

// ObjectInfo object metadata
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
}

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

// GetMetadata gets objects custom metadata
func (bl *ObjectInfo) GetMetadata(key string) string {
	return bl.metadata[key]
}

// ObjectList represents list of objects
type ObjectList struct {
	list storj.ObjectList
}

// More returns true if list request was not able to return all results
func (bl *ObjectList) More() bool {
	return bl.list.More
}

// Prefix prefix for objects from list
func (bl *ObjectList) Prefix() string {
	return bl.list.Prefix
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
