// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package ranger

import (
	"context"
	"io"
	"os"

	"storj.io/storj/pkg/utils"
)

type fileRanger struct {
	path string
	size int64
}

// FileRanger returns a Ranger from a path.
func FileRanger(path string) (Ranger, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return &fileRanger{path: path, size: info.Size()}, nil
}

func (rr *fileRanger) Size() int64 {
	return rr.size
}

func (rr *fileRanger) Range(ctx context.Context, offset, length int64) (io.ReadCloser, error) {
	if offset < 0 {
		return nil, Error.New("negative offset")
	}
	if length < 0 {
		return nil, Error.New("negative length")
	}
	if offset+length > rr.size {
		return nil, Error.New("range beyond end")
	}

	fh, err := os.Open(rr.path)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	_, err = fh.Seek(offset, io.SeekStart)
	if err != nil {
		err = utils.CombineErrors(err, fh.Close())
		return nil, Error.Wrap(err)
	}
	return struct {
		io.Reader
		io.Closer
	}{
		Reader: io.LimitReader(fh, length),
		Closer: fh,
	}, nil
}
