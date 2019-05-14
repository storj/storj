// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package mobile

import (
	"fmt"
	"time"

	"storj.io/storj/pkg/storj"
)

type ObjectInfo struct {
	object storj.Object
}

func (bl *ObjectInfo) GetVersion() int32 {
	return int32(bl.object.Version)
}

func (bl *ObjectInfo) GetBucket() *BucketInfo {
	return newBucketInfo(bl.object.Bucket)
}

func (bl *ObjectInfo) GetPath() string {
	return bl.object.Path
}

func (bl *ObjectInfo) IsPrefix() bool {
	return bl.object.IsPrefix
}

func (bl *ObjectInfo) GetSize() int64 {
	return bl.object.Size
}

func (bl *ObjectInfo) GetMetadata(key string) string {
	return bl.object.Metadata[key]
}

func (bl *ObjectInfo) GetContentType() string {
	return bl.object.ContentType
}

func (bl *ObjectInfo) GetCreated() int64 {
	return bl.object.Created.UTC().UnixNano() / int64(time.Millisecond)
}

func (bl *ObjectInfo) GetModified() int64 {
	return bl.object.Modified.UTC().UnixNano() / int64(time.Millisecond)
}

func (bl *ObjectInfo) GetExpires() int64 {
	return bl.object.Expires.UTC().UnixNano() / int64(time.Millisecond)
}

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
	return &ObjectInfo{bl.list.Items[index]}, nil
}
