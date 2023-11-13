// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"context"
	"hash"
	"time"

	"github.com/spacemonkeygo/monkit/v3"

	"storj.io/storj/storagenode/blobstore"
)

var sizes = sizeCategory()

var canceledTag = monkit.SeriesTag{
	Key: "cancelled",
	Val: "true",
}

var committedTag = monkit.SeriesTag{
	Key: "cancelled",
	Val: "false",
}

// MonitoredBlobWriter is a blobstore.BlobWriter wrapper with additional Monkit metrics.
type MonitoredBlobWriter struct {
	name string
	blobstore.BlobWriter
	writeTime    time.Duration
	failed       bool
	writtenBytes int
}

// MonitorBlobWriter wraps the original BlobWriter and measures writing time.
func MonitorBlobWriter(name string, writer blobstore.BlobWriter) blobstore.BlobWriter {
	return &MonitoredBlobWriter{
		name:       name,
		BlobWriter: writer,
	}
}

// Write implements io.Write.
func (m *MonitoredBlobWriter) Write(p []byte) (n int, err error) {
	start := time.Now()

	n, err = m.BlobWriter.Write(p)
	m.writtenBytes += n

	m.writeTime += time.Since(start)
	if err != nil {
		m.failed = true
	}
	return n, err
}

// Cancel implements io.Write.
func (m *MonitoredBlobWriter) Cancel(ctx context.Context) error {
	err := m.BlobWriter.Cancel(ctx)
	mon.DurationVal(m.name, canceledTag, sizes(m.writtenBytes)).Observe(m.writeTime)
	return err

}

// Commit implements io.Commit.
func (m *MonitoredBlobWriter) Commit(ctx context.Context) error {
	err := m.BlobWriter.Commit(ctx)
	mon.DurationVal(m.name, committedTag, sizes(m.writtenBytes)).Observe(m.writeTime)
	return err
}

// MonitoredHash is a hash.Hash wrapper with additional Monkit metrics.
type MonitoredHash struct {
	name string
	hash.Hash
	writeTime    time.Duration
	writtenBytes int
}

// MonitorHash wraps the original Hash with an instance which also measures the hashing time.
func MonitorHash(name string, hash hash.Hash) hash.Hash {
	return &MonitoredHash{
		name: name,
		Hash: hash,
	}
}

// Sum implements hash.Hash.
func (m *MonitoredHash) Sum(b []byte) []byte {
	start := time.Now()
	sum := m.Hash.Sum(b)
	m.writeTime += time.Since(start)
	if m.writtenBytes > 0 {
		mon.DurationVal(m.name, sizes(m.writtenBytes)).Observe(m.writeTime)
	}
	m.writeTime = 0
	m.writtenBytes = 0
	return sum
}

// Reset implements hash.Hash.
func (m *MonitoredHash) Reset() {
	m.Hash.Reset()
	m.writeTime = 0
	m.writtenBytes = 0
}

// Write implements io.Writer.
func (m *MonitoredHash) Write(p []byte) (n int, err error) {
	start := time.Now()
	n, err = m.Hash.Write(p)
	m.writtenBytes += n
	m.writeTime += time.Since(start)
	return n, err
}

func sizeCategory() func(int) monkit.SeriesTag {
	var s2m = monkit.NewSeriesTag("size", "2m")
	var s1m = monkit.NewSeriesTag("size", "1m")
	var s500k = monkit.NewSeriesTag("size", "500k")
	var s50k = monkit.NewSeriesTag("size", "50k")
	var small = monkit.NewSeriesTag("size", "small")

	return func(byteNo int) monkit.SeriesTag {
		if byteNo > 2000000 {
			return s2m
		}
		if byteNo > 1000000 {
			return s1m
		}
		if byteNo > 500000 {
			return s500k
		}
		if byteNo > 50000 {
			return s50k
		}
		return small
	}

}
