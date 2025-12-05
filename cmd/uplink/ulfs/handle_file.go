// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package ulfs

import (
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplink/ulloc"
)

//
// read handles
//

// osMultiReadHandle implements MultiReadHandle for *os.Files.
func newOSMultiReadHandle(fh LocalBackendFile) (MultiReadHandle, error) {
	fi, err := fh.Stat()
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return NewGenericMultiReadHandle(fh, ObjectInfo{
		Loc:           ulloc.NewLocal(fh.Name()),
		IsPrefix:      false,
		Created:       fi.ModTime(), // TODO: os specific crtime
		ContentLength: fi.Size(),
	}), nil
}

//
// write handles
//

type fileGenericWriter struct {
	fs  LocalBackend
	raw LocalBackendFile
}

func (f *fileGenericWriter) WriteAt(b []byte, off int64) (int, error) { return f.raw.WriteAt(b, off) }
func (f *fileGenericWriter) Commit() error                            { return f.raw.Close() }
func (f *fileGenericWriter) Abort() error {
	return errs.Combine(
		f.raw.Close(),
		f.fs.Remove(f.raw.Name()),
	)
}

func newOSMultiWriteHandle(fs LocalBackend, fh LocalBackendFile) MultiWriteHandle {
	return NewGenericMultiWriteHandle(&fileGenericWriter{
		fs:  fs,
		raw: fh,
	})
}
