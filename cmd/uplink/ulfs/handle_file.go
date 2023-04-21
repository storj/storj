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
	fs   LocalBackend
	raw  LocalBackendFile
	done bool
}

func (f *fileGenericWriter) Write(b []byte) (int, error) { return f.raw.Write(b) }

func (f *fileGenericWriter) Commit() error {
	if f.done {
		return errs.New("already commit/aborted")
	}
	f.done = true

	return f.raw.Close()
}

func (f *fileGenericWriter) Abort() error {
	if f.done {
		return errs.New("already commit/aborted")
	}
	f.done = true

	return errs.Combine(
		f.raw.Close(),
		f.fs.Remove(f.raw.Name()),
	)
}

func newOSWriteHandle(fs LocalBackend, fh LocalBackendFile) WriteHandle {
	return &fileGenericWriter{
		fs:  fs,
		raw: fh,
	}
}
