// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/zeebo/assert"
	"github.com/zeebo/mwc"
)

func TestSaturatingUint32(t *testing.T) {
	assert.Equal(t, saturatingUint23(0), 0)
	assert.Equal(t, saturatingUint23(-1), 1<<23-1)
	assert.Equal(t, saturatingUint23(1<<23-1), 1<<23-1)
	assert.Equal(t, saturatingUint23(1<<23), 1<<23-1)
}

//
// hashtbl
//

type testHashTbl struct {
	t testing.TB
	*HashTbl
}

func newTestHashtbl(t testing.TB, lrec uint64) *testHashTbl {
	t.Helper()

	fh, err := os.CreateTemp(t.TempDir(), "hashtbl")
	assert.NoError(t, err)

	h, err := CreateHashtbl(fh, lrec, 0)
	assert.NoError(t, err)

	return &testHashTbl{t: t, HashTbl: h}
}

func (th *testHashTbl) AssertReopen() {
	th.t.Helper()

	th.HashTbl.Close()

	fh, err := os.Open(th.fh.Name())
	assert.NoError(th.t, err)

	h, err := OpenHashtbl(fh)
	assert.NoError(th.t, err)

	th.HashTbl = h
}

func (th *testHashTbl) AssertInsertRecord(rec Record) {
	th.t.Helper()

	ok, err := th.Insert(context.Background(), rec)
	assert.NoError(th.t, err)
	assert.True(th.t, ok)
}

func (th *testHashTbl) AssertInsert() Record {
	th.t.Helper()

	rec := newRecord(newKey())
	th.AssertInsertRecord(rec)
	return rec
}

func (th *testHashTbl) AssertLookup(k Key) Record {
	th.t.Helper()

	r, ok, err := th.Lookup(context.Background(), k)
	assert.NoError(th.t, err)
	assert.True(th.t, ok)
	return r
}

//
// store
//

type testStore struct {
	t testing.TB
	*Store
	today uint32
}

func newTestStore(t testing.TB) *testStore {
	t.Helper()

	s, err := NewStore(t.TempDir(), nil)
	assert.NoError(t, err)

	ts := &testStore{t: t, Store: s, today: s.today()}

	s.today = func() uint32 { return ts.today }

	return ts
}

func (ts *testStore) AssertReopen() {
	ts.t.Helper()

	ts.Store.Close()

	s, err := NewStore(ts.dir, ts.log)
	assert.NoError(ts.t, err)

	s.today = func() uint32 { return ts.today }

	ts.Store = s
}

func (ts *testStore) AssertCompact(
	shouldTrash func(context.Context, Key, time.Time) bool,
	restore time.Time,
) {
	ts.t.Helper()

	assert.NoError(ts.t, ts.Compact(context.Background(), shouldTrash, restore))
}

func (ts *testStore) AssertCreateKey(key Key, expires time.Time) {
	ts.t.Helper()

	wr, err := ts.Create(context.Background(), key, expires)
	assert.NoError(ts.t, err)
	assert.Equal(ts.t, wr.Size(), int64(0))
	_, err = wr.Write(key[:])
	assert.NoError(ts.t, err)
	assert.Equal(ts.t, wr.Size(), int64(len(key)))
	assert.NoError(ts.t, wr.Close())
}

func (ts *testStore) AssertCreate(expires time.Time) Key {
	ts.t.Helper()

	key := newKey()
	ts.AssertCreateKey(key, expires)
	return key
}

func (ts *testStore) AssertRead(key Key) {
	ts.t.Helper()

	r, err := ts.Read(context.Background(), key)
	assert.NoError(ts.t, err)
	assert.NotNil(ts.t, r)

	assert.Equal(ts.t, r.Key(), key)
	assert.Equal(ts.t, r.Size(), len(key))

	data, err := io.ReadAll(r)
	assert.NoError(ts.t, err)
	assert.Equal(ts.t, data, key[:])
	assert.NoError(ts.t, r.Close())
}

func (ts *testStore) AssertNotExist(key Key) {
	ts.t.Helper()

	r, err := ts.Read(context.Background(), key)
	assert.NoError(ts.t, err)
	assert.Nil(ts.t, r)
}

//
// db
//

type testDB struct {
	t testing.TB
	*DB
}

func newTestDB(t testing.TB,
	dead func(context.Context, Key, time.Time) bool,
	restore func(context.Context) time.Time,
) *testDB {
	t.Helper()

	db, err := New(t.TempDir(), nil, dead, restore)
	assert.NoError(t, err)

	td := &testDB{t: t, DB: db}

	return td
}

func (td *testDB) AssertReopen() {
	td.t.Helper()

	td.DB.Close()

	db, err := New(td.dir, td.log, td.shouldTrash, td.lastRestore)
	assert.NoError(td.t, err)

	td.DB = db
}

func (td *testDB) AssertCreateKey(key Key, expires time.Time) {
	td.t.Helper()

	wr, err := td.Create(context.Background(), key, expires)
	assert.NoError(td.t, err)
	_, err = wr.Write(key[:])
	assert.NoError(td.t, err)
	assert.NoError(td.t, wr.Close())
}

func (td *testDB) AssertCreate(expires time.Time) Key {
	td.t.Helper()

	key := newKey()
	td.AssertCreateKey(key, expires)
	return key
}

func (td *testDB) AssertRead(key Key) {
	td.t.Helper()

	r, err := td.Read(context.Background(), key)
	assert.NoError(td.t, err)
	assert.NotNil(td.t, r)

	data, err := io.ReadAll(r)
	assert.NoError(td.t, err)
	assert.Equal(td.t, data, key[:])
	assert.NoError(td.t, r.Close())
}

func (td *testDB) AssertCompact() {
	td.t.Helper()

	assert.NoError(td.t, td.Compact(context.Background()))
}

//
// other helpers
//

func newKey() (k Key) {
	_, _ = mwc.Rand().Read(k[:])
	return
}

func newRecord(k Key) Record {
	n := binary.BigEndian.Uint32(k[28:32])
	return Record{
		Key:     k,
		Offset:  uint64(n),
		Log:     uint64(n),
		Length:  n,
		Created: n & 0x7fffff,
		Expires: NewExpiration(n&0x7fffff, n%2 == 0),
	}
}

func alwaysTrash(ctx context.Context, key Key, created time.Time) bool {
	return true
}

func blockOnContext(ctx context.Context, key Key, created time.Time) bool {
	<-ctx.Done()
	return false
}

func waitForGoroutine(frames ...string) {
	var buf [1 << 20]byte

	for {
		stacks := string(buf[:runtime.Stack(buf[:], true)])
	goroutine:
		for _, g := range strings.Split(stacks, "\n\n") {
			for _, frame := range frames {
				if !strings.Contains(g, frame) {
					time.Sleep(time.Millisecond)
					continue goroutine
				}
			}
			return
		}
		time.Sleep(1 * time.Millisecond)
	}
}

func benchmarkSizes(b *testing.B, name string, run func(*testing.B, uint64)) {
	b.Run(name, func(b *testing.B) {
		b.Run("0B", func(b *testing.B) { run(b, 0) })
		b.Run("256B", func(b *testing.B) { run(b, 256) })
		b.Run("1KB", func(b *testing.B) { run(b, 1*1024) })
		b.Run("4KB", func(b *testing.B) { run(b, 4*1024) })
		b.Run("16KB", func(b *testing.B) { run(b, 16*1024) })
		b.Run("64KB", func(b *testing.B) { run(b, 64*1024) })
		b.Run("256KB", func(b *testing.B) { run(b, 256*1024) })
		b.Run("1MB", func(b *testing.B) { run(b, 1*1024*1024) })
		b.Run("2MB", func(b *testing.B) { run(b, 2*1024*1024) })
	})
}

func benchmarkLRecs(b *testing.B, name string, run func(*testing.B, uint64)) {
	b.Run(name, func(b *testing.B) {
		for lrec := uint64(6); lrec < 15; lrec++ {
			b.Run(fmt.Sprintf("lrec=%d", lrec), func(b *testing.B) { run(b, lrec) })
		}
	})
}
