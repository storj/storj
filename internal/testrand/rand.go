// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package testrand implements generating random base types for testing.
package testrand

import (
	"io"
	"math/rand"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/storj"
)

// Intn returns, as an int, a non-negative pseudo-random number in [0,n)
// from the default Source.
// It panics if n <= 0.
func Intn(n int) int { return rand.Intn(n) }

// Int63n returns, as an int64, a non-negative pseudo-random number in [0,n)
// from the default Source.
// It panics if n <= 0.
func Int63n(n int64) int64 {
	return rand.Int63n(n)
}

// Read reads pseudo-random data into data.
func Read(data []byte) {
	const newSourceThreshold = 64
	if len(data) < newSourceThreshold {
		_, _ = rand.Read(data)
		return
	}

	src := rand.NewSource(rand.Int63())
	r := rand.New(src)
	_, _ = r.Read(data)
}

// Bytes generates size amount of random data.
func Bytes(size memory.Size) []byte {
	data := make([]byte, size.Int())
	Read(data)
	return data
}

// BytesInt generates size amount of random data.
func BytesInt(size int) []byte {
	return Bytes(memory.Size(size))
}

// Reader creates a new random data reader.
func Reader() io.Reader {
	return rand.New(rand.NewSource(rand.Int63()))
}

// NodeID creates a random node id.
func NodeID() storj.NodeID {
	var id storj.NodeID
	Read(id[:])
	// set version to 0
	id[len(id)-1] = 0
	return id
}

// PieceID creates a random piece id.
func PieceID() storj.PieceID {
	var id storj.PieceID
	Read(id[:])
	return id
}

// Key creates a random test key.
func Key() storj.Key {
	var key storj.Key
	Read(key[:])
	return key
}

// Nonce creates a random test nonce.
func Nonce() storj.Nonce {
	var nonce storj.Nonce
	Read(nonce[:])
	return nonce
}

// SerialNumber creates a random serial number.
func SerialNumber() storj.SerialNumber {
	var serial storj.SerialNumber
	Read(serial[:])
	return serial
}

// UUID creates a random uuid.
func UUID() uuid.UUID {
	var uuid uuid.UUID
	Read(uuid[:])
	return uuid
}
