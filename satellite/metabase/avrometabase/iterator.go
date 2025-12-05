// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package avrometabase

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"

	"cloud.google.com/go/storage"
	"github.com/zeebo/errs"
	"google.golang.org/api/iterator"
)

// ReaderIterator is an iterator over Avro files.
type ReaderIterator interface {
	Next(ctx context.Context) (io.ReadCloser, error)
}

// FileIterator is an iterator over Avro files on disk.
type FileIterator struct {
	pattern string

	initOnce sync.Once

	mu           sync.Mutex
	files        []string
	currentIndex int
}

// NewFileIterator creates a new FileIterator.
func NewFileIterator(pattern string) ReaderIterator {
	return &FileIterator{
		pattern: pattern,
	}
}

// Next returns the next Avro file.
func (a *FileIterator) Next(ctx context.Context) (io.ReadCloser, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	var err error
	a.initOnce.Do(func() {
		a.files, err = filepath.Glob(a.pattern)
	})
	if err != nil {
		return nil, errs.New("failed to get files list: %v", err)
	}

	if a.currentIndex >= len(a.files) {
		return nil, nil
	}

	file, err := os.Open(a.files[a.currentIndex])
	if err != nil {
		return nil, err
	}

	a.currentIndex++

	return file, nil
}

// GCSIterator is an iterator over Avro files in GCS.
type GCSIterator struct {
	bucket  string
	pattern string

	initOnce sync.Once

	client   *storage.Client
	mu       sync.Mutex
	iterator *storage.ObjectIterator
}

// NewGCSIterator creates a new GCSIterator.
func NewGCSIterator(bucket, pattern string) ReaderIterator {
	return &GCSIterator{
		bucket:  bucket,
		pattern: pattern,
	}
}

// Next returns the next Avro file.
func (a *GCSIterator) Next(ctx context.Context) (rc io.ReadCloser, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	defer func() {
		if a.client != nil && !(rc != nil && err == nil) {
			err = errs.Combine(err, a.client.Close())
		}
	}()

	a.initOnce.Do(func() {
		a.client, err = storage.NewClient(ctx)
		if err != nil {
			return
		}

		a.iterator = a.client.Bucket(a.bucket).Objects(ctx, &storage.Query{
			MatchGlob: a.pattern,
		})
	})
	if err != nil {
		return nil, errs.New("failed to create GCS storage client: %v", err)
	}

	attr, err := a.iterator.Next()
	if err != nil {
		if errors.Is(err, iterator.Done) {
			return nil, nil
		}
		return nil, errs.New("failed to get next GCS object: %v", err)
	}

	reader, err := a.client.Bucket(a.bucket).Object(attr.Name).NewReader(ctx)
	if err != nil {
		return nil, errs.New("failed to create GCS object reader: %v", err)
	}
	return reader, nil
}
