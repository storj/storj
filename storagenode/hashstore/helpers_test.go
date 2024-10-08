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

//
// hashtbl
//

type testHashTbl struct {
	t testing.TB
	*hashTbl
}

func newTestHashtbl(t testing.TB, lrec uint64) *testHashTbl {
	t.Helper()

	fh, err := os.CreateTemp(t.TempDir(), "hashtbl")
	assert.NoError(t, err)
	assert.NoError(t, fh.Truncate(1<<lrec*rSize))

	h, err := newHashTbl(fh, lrec, false)
	assert.NoError(t, err)

	return &testHashTbl{t: t, hashTbl: h}
}

func (th *testHashTbl) AssertReopen() {
	th.t.Helper()

	// we don't close the hold table because we reused the file handle.
	h, err := newHashTbl(th.fh, th.lrec, true)
	assert.NoError(th.t, err)
	th.hashTbl = h
}

func (th *testHashTbl) AssertInsertRecord(rec record) {
	th.t.Helper()

	ok, err := th.Insert(rec)
	assert.NoError(th.t, err)
	assert.True(th.t, ok)
}

func (th *testHashTbl) AssertInsert() record {
	th.t.Helper()

	rec := newRecord(newKey())
	th.AssertInsertRecord(rec)
	return rec
}

func (th *testHashTbl) AssertLookup(k Key) record {
	th.t.Helper()

	r, ok, err := th.Lookup(k)
	assert.NoError(th.t, err)
	assert.True(th.t, ok)
	return r
}

//
// store
//

type testStore struct {
	t testing.TB
	*store
	today uint32
}

func newTestStore(t testing.TB, nlogs int) *testStore {
	t.Helper()

	s, err := newStore(t.TempDir(), nlogs, nil)
	assert.NoError(t, err)

	ts := &testStore{t: t, store: s, today: s.today()}
	s.today = func() uint32 { return ts.today }

	return ts
}

func (ts *testStore) AssertReopen() {
	ts.t.Helper()

	ts.store.Close()
	s, err := newStore(ts.dir, ts.nlogs, ts.log)
	assert.NoError(ts.t, err)
	ts.store = s
}

func (ts *testStore) AssertCompact(
	shouldTrash func(context.Context, Key, time.Time) (bool, error),
	restore time.Time,
) {
	ts.t.Helper()

	assert.NoError(ts.t, ts.Compact(context.Background(), shouldTrash, restore))
}

func (ts *testStore) AssertCreateKey(key Key, expires time.Time) {
	ts.t.Helper()

	wr, err := ts.Create(context.Background(), key, expires)
	assert.NoError(ts.t, err)
	_, err = wr.Write(key[:])
	assert.NoError(ts.t, err)
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

func newTestDB(t testing.TB, nlogs int,
	dead func(context.Context, Key, time.Time) (bool, error),
	restore func(context.Context) (time.Time, error),
) *testDB {
	t.Helper()

	db, err := New(t.TempDir(), nlogs, nil, dead, restore)
	assert.NoError(t, err)

	td := &testDB{t: t, DB: db}

	return td
}

func (td *testDB) AssertReopen() {
	td.t.Helper()

	td.DB.Close()
	db, err := New(td.dir, td.nlogs, td.log, td.shouldTrash, td.lastRestore)
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

//
// other helpers
//

func newKey() (k Key) {
	_, _ = mwc.Rand().Read(k[:])
	return
}

func newRecord(k Key) record {
	n := binary.BigEndian.Uint32(k[28:32])
	rec := record{
		key:     k,
		offset:  uint64(n),
		log:     n,
		length:  n,
		created: n,
		expires: newExpiration(n, false),
	}
	rec.setChecksum()
	return rec
}

func alwaysTrash(ctx context.Context, key Key, created time.Time) (bool, error) {
	return true, nil
}

func blockOnContext(ctx context.Context, key Key, created time.Time) (bool, error) {
	<-ctx.Done()
	return false, ctx.Err()
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
