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
	"sync"
	"testing"
	"testing/iotest"
	"time"

	"github.com/zeebo/assert"
	"github.com/zeebo/mwc"

	"storj.io/storj/storagenode/hashstore/platform"
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

func TestShortCollidingKeys(t *testing.T) {
	k0, k1 := newShortCollidingKeys()
	assert.Equal(t, shortKeyFrom(k0), shortKeyFrom(k1))
	assert.NotEqual(t, k0, k1)
}

//
// test helpers
//

var (
	// since temporarily is used primarily to set global variables during tests, it might be called
	// for the same variable multiple times. this is probably a bug, and so we keep track of which
	// variables are already set and panic if they overlap.
	temporarilyMutex    sync.Mutex
	temporarilyAcquired = make(map[any]bool)
)

func temporarily[T any](loc *T, val T) func() {
	temporarilyMutex.Lock()
	defer temporarilyMutex.Unlock()

	if temporarilyAcquired[loc] {
		panic("overlapped temporarily calls")
	}
	temporarilyAcquired[loc] = true

	old := *loc
	*loc = val
	return func() { *loc = old; delete(temporarilyAcquired, loc) }
}

func forAllTables[T interface{ Run(string, func(T)) bool }](t T, fn func(T)) {
	mmaps := map[TableKind]*bool{
		kind_HashTbl: &hashtbl_MMAP,
		kind_MemTbl:  &memtbl_MMAP,
	}

	run := func(t T, kind TableKind, mmap bool) {
		t.Run(fmt.Sprintf("tbl=%s/mmap=%v", kind, mmap), func(t T) {
			defer temporarily(&table_DefaultKind, kind)()
			defer temporarily(mmaps[kind], mmap)()
			fn(t)
		})
	}

	run(t, kind_HashTbl, false)
	run(t, kind_MemTbl, false)
	if platform.MmapSupported {
		run(t, kind_HashTbl, true)
		run(t, kind_MemTbl, true)
	}
}

func forEachBool[T interface{ Run(string, func(T)) bool }](t T, name string, ptr *bool, fn func(T)) {
	t.Run(name+"=false", func(t T) { defer temporarily(ptr, false)(); fn(t) })
	t.Run(name+"=true", func(t T) { defer temporarily(ptr, true)(); fn(t) })
}

func ifFailed(t testing.TB, fn func()) {
	if t.Failed() {
		fn()
	}
}

func withEntries(t *testing.T, entries int, keys *[]Key) WithConstructor {
	return WithConstructor(func(tc TblConstructor) {
		ctx := context.Background()
		for i := 0; i < entries; i++ {
			k := newKey()
			ok, err := tc.Append(ctx, newRecord(k))
			assert.NoError(t, err)
			assert.True(t, ok)
			if keys != nil {
				*keys = append(*keys, k)
			}
		}
	})
}

func withFilledTable(t *testing.T, keys *[]Key) WithConstructor {
	return WithConstructor(func(tc TblConstructor) {
		ctx := context.Background()
		for {
			k := newKey()
			ok, err := tc.Append(ctx, newRecord(k))
			assert.NoError(t, err)
			if !ok {
				break
			} else if keys != nil {
				*keys = append(*keys, k)
			}
		}
	})
}

//
// generic table
//

type testTbl struct {
	t testing.TB
	Tbl
}

func newTestTbl(t testing.TB, lrec uint64, opts ...any) *testTbl {
	fh, err := os.CreateTemp(t.TempDir(), "tbl")
	assert.NoError(t, err)
	defer ifFailed(t, func() { _ = fh.Close() })

	cons, err := CreateTable(context.Background(), fh, lrec, 0, table_DefaultKind)
	assert.NoError(t, err)
	defer cons.Close()
	checkOptions(opts, func(tc WithConstructor) { tc(cons) })
	tbl, err := cons.Done(context.Background())
	assert.NoError(t, err)

	return &testTbl{t: t, Tbl: tbl}
}

func (tbl *testTbl) Close() { tbl.Tbl.Close() }

func (tbl *testTbl) AssertReopen() {
	tbl.Tbl.Close()

	fh, err := os.OpenFile(tbl.Handle().Name(), os.O_RDWR, 0)
	assert.NoError(tbl.t, err)

	h, err := OpenTable(context.Background(), fh)
	assert.NoError(tbl.t, err)

	tbl.Tbl = h
}

func (tbl *testTbl) AssertInsertRecord(rec Record) {
	ok, err := tbl.Insert(context.Background(), rec)
	assert.NoError(tbl.t, err)
	assert.True(tbl.t, ok)
}

func (tbl *testTbl) AssertInsert(opts ...any) Record {
	key := newKey()
	checkOptions(opts, func(t WithKey) { key = Key(t) })

	rec := newRecord(key)
	checkOptions(opts, func(t WithRecord) { rec = Record(t) })

	tbl.AssertInsertRecord(rec)
	return rec
}

func (tbl *testTbl) AssertLookup(k Key) Record {
	r, ok, err := tbl.Lookup(context.Background(), k)
	assert.NoError(tbl.t, err)
	assert.True(tbl.t, ok)
	return r
}

func (tbl *testTbl) AssertLookupMiss(k Key) {
	_, ok, err := tbl.Lookup(context.Background(), k)
	assert.NoError(tbl.t, err)
	assert.False(tbl.t, ok)
}

//
// hashtbl
//

type testHashTbl struct {
	t testing.TB
	*HashTbl
}

func newTestHashTbl(t testing.TB, lrec uint64, opts ...any) *testHashTbl {
	fh, err := os.CreateTemp(t.TempDir(), "hashtbl")
	assert.NoError(t, err)
	defer ifFailed(t, func() { _ = fh.Close() })

	cons, err := CreateHashTbl(context.Background(), fh, lrec, 0)
	assert.NoError(t, err)
	defer cons.Close()
	checkOptions(opts, func(tc WithConstructor) { tc(cons) })
	h, err := cons.Done(context.Background())
	assert.NoError(t, err)

	return &testHashTbl{t: t, HashTbl: h.(*HashTbl)}
}

func (th *testHashTbl) Close() { th.HashTbl.Close() }

func (th *testHashTbl) AssertReopen() {
	th.HashTbl.Close()

	fh, err := os.OpenFile(th.fh.Name(), os.O_RDWR, 0)
	assert.NoError(th.t, err)

	h, err := OpenHashTbl(context.Background(), fh)
	assert.NoError(th.t, err)

	th.HashTbl = h
}

func (th *testHashTbl) AssertInsertRecord(rec Record) {
	ok, err := th.Insert(context.Background(), rec)
	assert.NoError(th.t, err)
	assert.True(th.t, ok)
}

func (th *testHashTbl) AssertInsert(opts ...any) Record {
	key := newKey()
	checkOptions(opts, func(t WithKey) { key = Key(t) })

	rec := newRecord(key)
	checkOptions(opts, func(t WithRecord) { rec = Record(t) })

	th.AssertInsertRecord(rec)
	return rec
}

func (th *testHashTbl) AssertLookup(k Key) Record {
	r, ok, err := th.Lookup(context.Background(), k)
	assert.NoError(th.t, err)
	assert.True(th.t, ok)
	return r
}

func (th *testHashTbl) AssertLookupMiss(k Key) {
	_, ok, err := th.Lookup(context.Background(), k)
	assert.NoError(th.t, err)
	assert.False(th.t, ok)
}

//
// memtbl
//

type testMemTbl struct {
	t testing.TB
	*MemTbl
}

func newTestMemTbl(t testing.TB, lrec uint64, opts ...any) *testMemTbl {
	fh, err := os.CreateTemp(t.TempDir(), "memtbl")
	assert.NoError(t, err)
	defer ifFailed(t, func() { _ = fh.Close() })

	cons, err := CreateMemTbl(context.Background(), fh, lrec, 0)
	assert.NoError(t, err)
	defer cons.Close()
	checkOptions(opts, func(tc WithConstructor) { tc(cons) })
	m, err := cons.Done(context.Background())
	assert.NoError(t, err)

	return &testMemTbl{t: t, MemTbl: m.(*MemTbl)}
}

func (tm *testMemTbl) Close() { tm.MemTbl.Close() }

func (tm *testMemTbl) AssertReopen() {
	tm.MemTbl.Close()

	fh, err := os.OpenFile(tm.fh.Name(), os.O_RDWR, 0)
	assert.NoError(tm.t, err)

	m, err := OpenMemTbl(context.Background(), fh)
	assert.NoError(tm.t, err)

	tm.MemTbl = m
}

func (tm *testMemTbl) AssertInsertRecord(rec Record) {
	ok, err := tm.Insert(context.Background(), rec)
	assert.NoError(tm.t, err)
	assert.True(tm.t, ok)
}

func (tm *testMemTbl) AssertInsert(opts ...any) Record {
	key := newKey()
	checkOptions(opts, func(t WithKey) { key = Key(t) })

	rec := newRecord(key)
	checkOptions(opts, func(t WithRecord) { rec = Record(t) })

	tm.AssertInsertRecord(rec)
	return rec
}

func (tm *testMemTbl) AssertLookup(k Key) Record {
	r, ok, err := tm.Lookup(context.Background(), k)
	assert.NoError(tm.t, err)
	assert.True(tm.t, ok)
	return r
}

func (tm *testMemTbl) AssertLookupMiss(k Key) {
	_, ok, err := tm.Lookup(context.Background(), k)
	assert.NoError(tm.t, err)
	assert.False(tm.t, ok)
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
	s, err := NewStore(context.Background(), t.TempDir(), "", nil)
	assert.NoError(t, err)

	ts := &testStore{t: t, Store: s, today: s.today()}

	s.today = func() uint32 { return ts.today }

	return ts
}

func (ts *testStore) Close() { ts.Store.Close() }

func (ts *testStore) AssertReopen() {
	ts.Store.Close()

	s, err := NewStore(context.Background(), ts.logsPath, ts.tablePath, ts.log)
	assert.NoError(ts.t, err)

	s.today = func() uint32 { return ts.today }

	ts.Store = s
}

func (ts *testStore) AssertCompact(
	shouldTrash func(context.Context, Key, time.Time) bool,
	restore time.Time,
) {
	assert.NoError(ts.t, ts.Compact(context.Background(), shouldTrash, restore))
}

func (ts *testStore) AssertCreate(opts ...any) Key {
	var expires time.Time
	checkOptions(opts, func(t WithTTL) { expires = time.Time(t) })

	key := newKey()
	checkOptions(opts, func(t WithKey) { key = Key(t) })

	data := key[:]
	checkOptions(opts, func(t WithDataSize) { data = make([]byte, t) })
	checkOptions(opts, func(t WithData) { data = []byte(t) })

	wr, err := ts.Create(context.Background(), key, expires)
	assert.NoError(ts.t, err)
	assert.Equal(ts.t, wr.Size(), 0)

	_, err = wr.Write(data)
	assert.NoError(ts.t, err)

	assert.Equal(ts.t, wr.Size(), len(data))
	assert.NoError(ts.t, wr.Close())

	return key
}

func (ts *testStore) AssertRead(key Key, opts ...any) {
	r, err := ts.Read(context.Background(), key)
	assert.NoError(ts.t, err)
	assert.NotNil(ts.t, r)

	checkOptions(opts, func(rt AssertTrash) {
		assert.Equal(ts.t, rt, r.Trash())
	})

	assert.Equal(ts.t, r.Key(), key)

	data := key[:]
	checkOptions(opts, func(t WithDataSize) { data = make([]byte, t) })
	checkOptions(opts, func(t WithData) { data = []byte(t) })

	assert.Equal(ts.t, r.Size(), len(data))
	assert.NoError(ts.t, iotest.TestReader(r, data))

	checkOptions(opts, func(wr WithRevive) {
		if wr {
			assert.NoError(ts.t, r.Revive(context.Background()))
		}
	})

	assert.NoError(ts.t, r.Close())
}

func (ts *testStore) AssertNotExist(key Key) {
	r, err := ts.Read(context.Background(), key)
	assert.NoError(ts.t, err)
	assert.Nil(ts.t, r)
}

func (ts *testStore) AssertExist(key Key) {
	_, ok, err := ts.tbl.Lookup(context.Background(), key)
	assert.NoError(ts.t, err)
	assert.True(ts.t, ok)
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
	db, err := New(context.Background(), t.TempDir(), "", nil, dead, restore)
	assert.NoError(t, err)

	td := &testDB{t: t, DB: db}

	return td
}

func (td *testDB) Close() { td.DB.Close() }

func (td *testDB) AssertReopen() {
	td.DB.Close()

	db, err := New(context.Background(), td.logsPath, td.tablePath, td.log, td.shouldTrash, td.lastRestore)
	assert.NoError(td.t, err)

	td.DB = db
}

func (td *testDB) AssertCreate(opts ...any) Key {
	var expires time.Time
	checkOptions(opts, func(t WithTTL) { expires = time.Time(t) })

	key := newKey()
	checkOptions(opts, func(t WithKey) { key = Key(t) })

	data := key[:]
	checkOptions(opts, func(t WithDataSize) { data = make([]byte, t) })
	checkOptions(opts, func(t WithData) { data = []byte(t) })

	wr, err := td.Create(context.Background(), key, expires)
	assert.NoError(td.t, err)
	assert.Equal(td.t, wr.Size(), 0)

	_, err = wr.Write(data)
	assert.NoError(td.t, err)

	assert.Equal(td.t, wr.Size(), len(data))
	assert.NoError(td.t, wr.Close())

	return key
}

func (td *testDB) AssertRead(key Key, opts ...any) {
	r, err := td.Read(context.Background(), key)
	assert.NoError(td.t, err)
	assert.NotNil(td.t, r)

	checkOptions(opts, func(rt AssertTrash) {
		assert.Equal(td.t, rt, r.Trash())
	})

	assert.NoError(td.t, iotest.TestReader(r, key[:]))
	assert.NoError(td.t, r.Close())
}

func (td *testDB) AssertCompact() {
	assert.NoError(td.t, td.Compact(context.Background()))
}

//
// other helpers
//

type (
	AssertTrash     bool
	WithTTL         time.Time
	WithData        []byte
	WithDataSize    int
	WithKey         Key
	WithRecord      Record
	WithRevive      bool
	WithConstructor func(TblConstructor)
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

func newShortCollidingKeys() (k0, k1 Key) {
	_, _ = mwc.Rand().Read(k0[:])
	k1 = k0
	k1[0]++
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
		b.Run("size=0B", func(b *testing.B) { run(b, 0) })
		b.Run("size=256B", func(b *testing.B) { run(b, 256) })
		b.Run("size=1KB", func(b *testing.B) { run(b, 1*1024) })
		b.Run("size=4KB", func(b *testing.B) { run(b, 4*1024) })
		b.Run("size=16KB", func(b *testing.B) { run(b, 16*1024) })
		if !testing.Short() {
			b.Run("size=64KB", func(b *testing.B) { run(b, 64*1024) })
			b.Run("size=256KB", func(b *testing.B) { run(b, 256*1024) })
			b.Run("size=1MB", func(b *testing.B) { run(b, 1*1024*1024) })
			b.Run("size=2MB", func(b *testing.B) { run(b, 2*1024*1024) })
		}
	})
}

func benchmarkLRecs(b *testing.B, name string, run func(*testing.B, uint64)) {
	b.Run(name, func(b *testing.B) {
		nrecs := uint64(20)
		if testing.Short() {
			nrecs = 16
		}
		for lrec := uint64(14); lrec < nrecs; lrec++ {
			b.Run(fmt.Sprintf("lrec=%d", lrec), func(b *testing.B) { run(b, lrec) })
		}
	})
}
