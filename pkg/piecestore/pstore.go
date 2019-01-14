// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package pstore

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/ranger"
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

// IDLength -- Minimum ID length
const IDLength = 20

// Errors
var (
	Error = errs.Class("piecestore error")
)

// PiecePath creates piece storage path from id and dir
func (storage *Storage) PiecePath(pieceID string) (string, error) {
	if len(pieceID) < IDLength {
		return "", Error.New("invalid id length")
	}

	folder1, folder2, filename := pieceID[0:2], pieceID[2:4], pieceID[4:]
	return filepath.Join(storage.dir, folder1, folder2, filename), nil
}

// Writer returns a writer that can be used to store piece.
func (storage *Storage) Writer(pieceID string) (io.WriteCloser, error) {
	path, err := storage.PiecePath(pieceID)
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
	path, err := storage.PiecePath(pieceID)
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
		return nil, err
	}

	return rr.Range(ctx, offset, length)
}

// Delete deletes piece from storage
func (storage *Storage) Delete(pieceID string) error {
	path, err := storage.PiecePath(pieceID)
	if err != nil {
		return err
	}

	err = os.Remove(path)
	if os.IsNotExist(err) {
		err = nil
	}
	return err
}
