// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.
package mobile

import (
	"fmt"

	"storj.io/storj/pkg/storj"
)

type ObjectInfo struct {
	object storj.Object
	// Stream
}

func (bl *ObjectInfo) Version() int32 {
	return int32(bl.object.Version)
}

func (bl *ObjectInfo) Bucket() *BucketInfo {
	return newBucketInfo(bl.object.Bucket)
}

func (bl *ObjectInfo) Path() string {
	return bl.object.Path
}

func (bl *ObjectInfo) IsPrefix() bool {
	return bl.object.IsPrefix
}

func (bl *ObjectInfo) Metadata(key string) string {
	return bl.object.Metadata[key]
}

func (bl *ObjectInfo) ContentType() string {
	return bl.object.ContentType
}

func (bl *ObjectInfo) Created() int {
	return int(bl.object.Created.UTC().Unix())
}

func (bl *ObjectInfo) Modified() int {
	return int(bl.object.Modified.UTC().Unix())
}

func (bl *ObjectInfo) Expires() int {
	return int(bl.object.Expires.UTC().Unix())
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
