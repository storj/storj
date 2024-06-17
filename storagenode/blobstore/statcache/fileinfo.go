// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package statcache

import (
	"encoding/binary"
	"time"

	"storj.io/storj/storagenode/blobstore"
)

// FileInfo is a simple implementation of blobstore.FileInfo.
type FileInfo struct {
	modTime time.Time
	size    int64
}

// ModTime implements blobstore.FileInfo.
func (f FileInfo) ModTime() time.Time {
	return f.modTime
}

// Size implements blobstore.FileInfo.
func (f FileInfo) Size() int64 {
	return f.size
}

func deserialize(item []byte) blobstore.FileInfo {
	modtime := binary.BigEndian.Uint64(item[:8])
	size := binary.BigEndian.Uint64(item[8:])
	return FileInfo{
		modTime: time.Unix(0, int64(modtime)),
		size:    int64(size),
	}
}

func serialize(value blobstore.FileInfo) []byte {
	res := make([]byte, 16)
	binary.BigEndian.PutUint64(res, uint64(value.ModTime().UnixNano()))
	binary.BigEndian.PutUint64(res[8:], uint64(value.Size()))
	return res
}
