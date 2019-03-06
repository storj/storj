// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pstore

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/shirou/gopsutil/disk"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/ranger"
)

// IDLength -- Minimum ID length
const IDLength = 20

// Errors
var (
	Error = errs.Class("piecestore error")
)

// Storage stores piecestore pieces
type Storage struct {
	dir string
}

// NewStorage creates database for storing pieces
func NewStorage(dir string) *Storage {
	return &Storage{dir}
}

// Close closes resources
func (storage *Storage) Close() error { return nil }

// DiskInfo contains statistics about the disk
type DiskInfo struct {
	AvailableSpace int64 // TODO: use memory.Size
}

// Info returns information about the current state of the dir
func (storage *Storage) Info() (DiskInfo, error) {
	rootPath := filepath.Dir(filepath.Clean(storage.dir))
	diskSpace, err := disk.Usage(rootPath)
	if err != nil {
		return DiskInfo{}, Error.Wrap(err)
	}
	return DiskInfo{
		AvailableSpace: int64(diskSpace.Free),
	}, nil
}

// piecePath creates piece storage path from id and dir
func (storage *Storage) piecePath(pieceID string) (string, error) {
	if len(pieceID) < IDLength {
		return "", Error.New("invalid id length")
	}
	folder1, folder2, filename := pieceID[0:2], pieceID[2:4], pieceID[4:]
	return filepath.Join(storage.dir, folder1, folder2, filename), nil
}

// Size returns piece size.
func (storage *Storage) Size(pieceID string) (int64, error) {
	path, err := storage.piecePath(pieceID)
	if err != nil {
		return 0, err
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		return 0, Error.Wrap(err)
	}

	return fileInfo.Size(), nil
}

// Writer returns a writer that can be used to store piece.
func (storage *Storage) Writer(pieceID string) (io.WriteCloser, error) {
	path, err := storage.piecePath(pieceID)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, Error.Wrap(err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return file, nil
}

// Reader returns a reader for the specified piece at the location
func (storage *Storage) Reader(ctx context.Context, pieceID string, offset int64, length int64) (io.ReadCloser, error) {
	path, err := storage.piecePath(pieceID)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if offset >= info.Size() || offset < 0 {
		return nil, Error.New("invalid offset: %v", offset)
	}
	if length <= -1 {
		length = info.Size()
	}
	// If trying to read past the end of the file, just read to the end
	if info.Size() < offset+length {
		length = info.Size() - offset
	}

	rr, err := ranger.FileRanger(path)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	r, err := rr.Range(ctx, offset, length)
	return r, Error.Wrap(err)
}

// Delete deletes piece from storage
func (storage *Storage) Delete(pieceID string) error {
	path, err := storage.piecePath(pieceID)
	if err != nil {
		return Error.Wrap(err)
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		err = nil
	}
	return Error.Wrap(err)
}
