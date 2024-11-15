// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/common/storj"
)

type pieceState int

const (
	pieceState_Normal pieceState = iota
	pieceState_Deleted
	pieceState_Corrupted
	pieceState_Mutated
)

type pieceIdentity struct {
	satellite storj.NodeID
	pieceID   storj.PieceID
}

type pieceContents struct {
	data   []byte
	header *pb.PieceHeader
}

// TestingBackend wraps a PieceBackend with Testing methods.
type TestingBackend struct {
	pb PieceBackend

	enabled  atomic.Bool
	delay    atomic.Int64
	error    atomic.Pointer[error]
	mu       sync.Mutex
	states   map[pieceIdentity]pieceState
	contents map[pieceIdentity]pieceContents
}

// NewTestingBackend constructs a TestingBackend wrapping a PieceBackend.
func NewTestingBackend(pb PieceBackend) *TestingBackend {
	return &TestingBackend{
		pb: pb,

		states:   make(map[pieceIdentity]pieceState),
		contents: make(map[pieceIdentity]pieceContents),
	}
}

// TestingEnableMethods enables the Testing methods and must be called from a stack that contains
// the testplanet package.
func (tb *TestingBackend) TestingEnableMethods() {
	var buf [4096]byte
	if !bytes.Contains(buf[:runtime.Stack(buf[:], false)], []byte("storj.io/storj/private/testplanet")) {
		panic("TestingEnableMethods can only be called from testplanet")
	}
	tb.enabled.Store(true)
}

// StartRestore implements PieceBackend.
func (tb *TestingBackend) StartRestore(ctx context.Context, satellite storj.NodeID) error {
	return tb.pb.StartRestore(ctx, satellite)
}

// Writer implements PieceBackend.
func (tb *TestingBackend) Writer(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID, hashAlgorithm pb.PieceHashAlgorithm, expiration time.Time) (PieceWriter, error) {
	if !tb.enabled.Load() { // fast path in production: just use the underying PieceBackend
		return tb.pb.Writer(ctx, satellite, pieceID, hashAlgorithm, expiration)
	}

	if err := tb.sleep(ctx); err != nil {
		return nil, err
	} else if errp := tb.error.Load(); errp != nil {
		return nil, *errp
	}

	wr, err := tb.pb.Writer(ctx, satellite, pieceID, hashAlgorithm, expiration)
	if err != nil {
		return nil, err
	}
	return &testingWriter{
		PieceWriter: wr,
		tb:          tb,
		satellite:   satellite,
		pieceID:     pieceID,
	}, nil
}

// Reader implements PieceBackend.
func (tb *TestingBackend) Reader(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) (PieceReader, error) {
	if !tb.enabled.Load() { // fast path in production: just use the underying PieceBackend
		return tb.pb.Reader(ctx, satellite, pieceID)
	}

	if err := tb.sleep(ctx); err != nil {
		return nil, err
	} else if errp := tb.error.Load(); errp != nil {
		return nil, *errp
	}

	tb.mu.Lock()
	defer tb.mu.Unlock()

	switch tb.states[pieceIdentity{satellite, pieceID}] {
	case pieceState_Normal:
		return tb.pb.Reader(ctx, satellite, pieceID)

	case pieceState_Deleted:
		return nil, errs.Wrap(fs.ErrNotExist)

	case pieceState_Corrupted:
		reader, err := tb.pb.Reader(ctx, satellite, pieceID)
		if err != nil {
			return nil, err
		}
		return &corruptedPieceReader{PieceReader: reader}, nil

	case pieceState_Mutated:
		contents := tb.contents[pieceIdentity{satellite, pieceID}]
		return &testingReader{
			Reader: bytes.NewReader(contents.data),
			header: contents.header,
		}, nil

	default:
		panic("invalid piece state")
	}
}

// TestingDeletePiece marks the piece as deleted if it exists.
func (tb *TestingBackend) TestingDeletePiece(satellite storj.NodeID, pieceID storj.PieceID) {
	if !tb.enabled.Load() {
		return
	}

	tb.mu.Lock()
	defer tb.mu.Unlock()

	key := pieceIdentity{satellite, pieceID}
	if _, ok := tb.states[key]; ok {
		tb.states[key] = pieceState_Deleted
	}
}

// TestingCorruptPiece marks the piece as corrupted (returns invalid data) if it exists.
func (tb *TestingBackend) TestingCorruptPiece(satellite storj.NodeID, pieceID storj.PieceID) {
	if !tb.enabled.Load() {
		return
	}

	tb.mu.Lock()
	defer tb.mu.Unlock()

	key := pieceIdentity{satellite, pieceID}
	if _, ok := tb.states[key]; ok {
		tb.states[key] = pieceState_Corrupted
	}
}

// TestingMutatePiece mutates the piece using the provided callback if it exists.
func (tb *TestingBackend) TestingMutatePiece(satellite storj.NodeID, pieceID storj.PieceID, mutator func(contents []byte, header *pb.PieceHeader)) {
	if !tb.enabled.Load() {
		return
	}

	tb.mu.Lock()
	defer tb.mu.Unlock()

	key := pieceIdentity{satellite, pieceID}
	if _, ok := tb.states[key]; ok {
		r, err := tb.pb.Reader(context.Background(), satellite, pieceID)
		if err != nil {
			panic(err)
		}
		defer func() { _ = r.Close() }()

		header, err := r.GetPieceHeader()
		if err != nil {
			panic(err)
		}

		data, err := io.ReadAll(r)
		if err != nil {
			panic(err)
		}

		mutator(data, header)

		tb.contents[key] = pieceContents{data, header}
		tb.states[key] = pieceState_Mutated
	}
}

// TestingDeleteAllPiecesForSatellite marks every piece for the satellite as deleted.
func (tb *TestingBackend) TestingDeleteAllPiecesForSatellite(satellite storj.NodeID) {
	if !tb.enabled.Load() {
		return
	}

	tb.mu.Lock()
	defer tb.mu.Unlock()

	for key := range tb.states {
		if key.satellite == satellite {
			tb.states[key] = pieceState_Deleted
		}
	}
}

// TestingSetLatency sets a latency that is added to every PieceBackend method.
func (tb *TestingBackend) TestingSetLatency(delay time.Duration) { tb.delay.Store(int64(delay)) }

// TestingSetError sets an error to be returned by every PieceBackend method.
func (tb *TestingBackend) TestingSetError(err error) { tb.error.Store(&err) }

func (tb *TestingBackend) sleep(ctx context.Context) error {
	delay := time.Duration(tb.delay.Load())
	if delay == 0 {
		return nil
	}
	select {
	case <-time.After(delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

//
// testingReader is an in-memory implementation of PieceReader
//

type testingReader struct {
	*bytes.Reader
	header *pb.PieceHeader
}

func (t *testingReader) Close() error                             { return nil }
func (t *testingReader) Trash() bool                              { return false }
func (t *testingReader) GetPieceHeader() (*pb.PieceHeader, error) { return t.header, nil }

//
// testingWriter records in the TestingBackend when a piece is committed
//

type testingWriter struct {
	PieceWriter
	tb        *TestingBackend
	satellite storj.NodeID
	pieceID   storj.PieceID
	once      sync.Once
}

func (w *testingWriter) Commit(ctx context.Context, header *pb.PieceHeader) (err error) {
	w.once.Do(func() {
		if err = w.PieceWriter.Commit(ctx, header); err == nil {
			w.tb.mu.Lock()
			defer w.tb.mu.Unlock()

			w.tb.states[pieceIdentity{w.satellite, w.pieceID}] = pieceState_Normal
		}
	})
	return err
}

func (w *testingWriter) Cancel(ctx context.Context) (err error) {
	w.once.Do(func() { err = w.PieceWriter.Cancel(ctx) })
	return err
}

//
// corruptedPieceReader returns corrupted data when reading a piece
//

type corruptedPieceReader struct{ PieceReader }

func (c *corruptedPieceReader) Read(p []byte) (n int, err error) {
	if n, err = c.PieceReader.Read(p); n > 0 {
		p[0]++
	}
	return n, err
}
