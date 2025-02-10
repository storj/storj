// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/zeebo/assert"
	"github.com/zeebo/mwc"
)

func TestClampTTL(t *testing.T) {
	assert.Equal(t, clampDate(0), 0)
	assert.Equal(t, clampDate(-1), 1)
	assert.Equal(t, clampDate(1<<23-1), 1<<23-1)
	assert.Equal(t, clampDate(1<<23), 1<<23-1)

	rng := mwc.Rand()
	for i := 0; i < 1000; i++ {
		// a negative timestamp still goes into the future
		ttl := time.Unix(-rng.Int63(), 0)
		assert.That(t, NormalizeTTL(ttl).After(ttl))
	}

	largestTTL := uint64(DateToTime(clampDate(1<<63 - 1)).Unix())
	for i := 0; i < 1000; i++ {
		// a positive timestamp smaller than largestTTL goes into the future
		ttl := time.Unix(int64(rng.Uint64n(largestTTL)), 0)
		assert.That(t, !NormalizeTTL(ttl).Before(ttl))
	}

	for i := 0; i < 1000; i++ {
		// anything larger than largestTTL goes into the past, but that's a problem for someone in
		// the year 24937.
		ttl := time.Unix(int64(largestTTL)+int64(i*i*i), 0)
		assert.That(t, !NormalizeTTL(ttl).After(ttl))
	}
}

func TestAllFiles(t *testing.T) {
	dir := t.TempDir()

	touch := func(name string) {
		assert.NoError(t, os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), 0755))
		assert.NoError(t, os.WriteFile(filepath.Join(dir, name), nil, 0644))
	}

	// backwards compatibility
	touch("log-0000000000000001-00000000")
	touch("log-0000000000000002-0000ffff")

	// new format
	touch("03/log-0000000000000003-00000000")
	touch("04/log-0000000000000004-00000000")
	touch("03/log-0000000000000103-00000000")

	entries, err := allFiles(dir)
	assert.NoError(t, err)
	assert.Equal(t, entries, []string{
		filepath.Join(dir, "03/log-0000000000000003-00000000"),
		filepath.Join(dir, "03/log-0000000000000103-00000000"),
		filepath.Join(dir, "04/log-0000000000000004-00000000"),
		filepath.Join(dir, "log-0000000000000001-00000000"),
		filepath.Join(dir, "log-0000000000000002-0000ffff"),
	})
}

func TestAtomicFile(t *testing.T) {
	dir := t.TempDir()
	f := func(name string) string { return filepath.Join(dir, name) }

	{ // successful path
		af, err := newAtomicFile(f("file0"))
		assert.NoError(t, err)
		defer af.Cancel()

		files, err := allFiles(dir)
		assert.NoError(t, err)
		assert.Equal(t, files, []string{f("file0.tmp")})

		_, err = af.Write([]byte("hello"))
		assert.NoError(t, err)
		assert.NoError(t, af.Commit())

		files, err = allFiles(dir)
		assert.NoError(t, err)
		assert.Equal(t, files, []string{f("file0")})

		data, err := os.ReadFile(f("file0"))
		assert.NoError(t, err)
		assert.Equal(t, string(data), "hello")
	}

	{ // cancel should clean up and commit after cancel should error
		af, err := newAtomicFile(f("file1"))
		assert.NoError(t, err)

		files, err := allFiles(dir)
		assert.NoError(t, err)
		assert.Equal(t, files, []string{f("file0"), f("file1.tmp")})

		af.Cancel()
		assert.Error(t, af.Commit())

		files, err = allFiles(dir)
		assert.NoError(t, err)
		assert.Equal(t, files, []string{f("file0")})
	}
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

	h, err := CreateHashtbl(context.Background(), fh, lrec, 0)
	assert.NoError(t, err)

	return &testHashTbl{t: t, HashTbl: h}
}

func (th *testHashTbl) Close() { th.HashTbl.Close() }

func (th *testHashTbl) AssertReopen() {
	th.t.Helper()

	th.HashTbl.Close()

	fh, err := os.Open(th.fh.Name())
	assert.NoError(th.t, err)

	h, err := OpenHashtbl(context.Background(), fh)
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

	s, err := NewStore(context.Background(), t.TempDir(), nil)
	assert.NoError(t, err)

	ts := &testStore{t: t, Store: s, today: s.today()}

	s.today = func() uint32 { return ts.today }

	return ts
}

func (ts *testStore) Close() { ts.Store.Close() }

func (ts *testStore) AssertReopen() {
	ts.t.Helper()

	ts.Store.Close()

	s, err := NewStore(context.Background(), ts.dir, ts.log)
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

func (ts *testStore) AssertCreate(opts ...any) Key {
	ts.t.Helper()

	var expires time.Time
	checkOptions(opts, func(t WithTTL) { expires = time.Time(t) })

	key := newKey()
	checkOptions(opts, func(t WithKey) { key = Key(t) })

	data := key[:]
	checkOptions(opts, func(t WithData) { data = []byte(t) })

	wr, err := ts.Create(context.Background(), key, expires)
	assert.NoError(ts.t, err)
	assert.Equal(ts.t, wr.Size(), int64(0))

	_, err = wr.Write(data)
	assert.NoError(ts.t, err)

	assert.Equal(ts.t, wr.Size(), int64(len(data)))
	assert.NoError(ts.t, wr.Close())

	return key
}

func (ts *testStore) AssertRead(key Key, opts ...any) {
	ts.t.Helper()

	r, err := ts.Read(context.Background(), key)
	assert.NoError(ts.t, err)
	assert.NotNil(ts.t, r)

	checkOptions(opts, func(rt AssertTrash) {
		assert.Equal(ts.t, rt, r.Trash())
	})

	assert.Equal(ts.t, r.Key(), key)

	data := key[:]
	checkOptions(opts, func(t WithData) { data = []byte(t) })

	assert.Equal(ts.t, r.Size(), len(data))
	assert.NoError(ts.t, iotest.TestReader(r, data))

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

	db, err := New(context.Background(), t.TempDir(), nil, dead, restore)
	assert.NoError(t, err)

	td := &testDB{t: t, DB: db}

	return td
}

func (td *testDB) Close() { td.DB.Close() }

func (td *testDB) AssertReopen() {
	td.t.Helper()

	td.DB.Close()

	db, err := New(context.Background(), td.dir, td.log, td.shouldTrash, td.lastRestore)
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

func (td *testDB) AssertCreate(opts ...any) Key {
	td.t.Helper()

	var expires time.Time
	checkOptions(opts, func(t WithTTL) { expires = time.Time(t) })

	key := newKey()
	td.AssertCreateKey(key, expires)
	return key
}

func (td *testDB) AssertRead(key Key) {
	td.t.Helper()

	r, err := td.Read(context.Background(), key)
	assert.NoError(td.t, err)
	assert.NotNil(td.t, r)

	assert.NoError(td.t, iotest.TestReader(r, key[:]))
	assert.NoError(td.t, r.Close())
}

func (td *testDB) AssertCompact() {
	td.t.Helper()

	assert.NoError(td.t, td.Compact(context.Background()))
}

//
// other helpers
//

type (
	AssertTrash bool
	WithTTL     time.Time
	WithData    []byte
	WithKey     Key
)

func checkOptions[T any](opts []any, cb func(T)) {
	for _, opt := range opts {
		if v, ok := opt.(T); ok {
			cb(v)
		}
	}
}

func newKey() (k Key) {
	_, _ = mwc.Rand().Read(k[:])
	return
}

func newKeyAt(h *HashTbl, pi pageIdxT, ri uint64, n uint8) (k Key) {
	rng := mwc.Rand()
	for {
		binary.BigEndian.PutUint64(k[0:8], rng.Uint64())
		k[31] = n
		gpi, gri := h.slotForKey(&k).PageIndexes()
		if pi == gpi && ri == gri {
			return k
		}
	}
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

func waitForGoroutine(frames ...string) { waitForGoroutines(1, frames...) }

func waitForGoroutines(count int, frames ...string) {
	var buf [1 << 20]byte

	for {
		matches := 0
		stacks := string(buf[:runtime.Stack(buf[:], true)])
	goroutine:
		for _, g := range strings.Split(stacks, "\n\n") {
			for _, frame := range frames {
				if !strings.Contains(g, frame) {
					continue goroutine
				}
			}
			matches++
			if matches >= count {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
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
		for lrec := uint64(14); lrec < 20; lrec++ {
			b.Run(fmt.Sprintf("lrec=%d", lrec), func(b *testing.B) { run(b, lrec) })
		}
	})
}
