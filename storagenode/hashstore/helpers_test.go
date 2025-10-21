// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/zeebo/assert"
	"github.com/zeebo/errs"
	"github.com/zeebo/mwc"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"storj.io/storj/storagenode/hashstore/platform"
)

func init() {
	// enable checking log file size and offset
	test_Log_CheckSizeAndOffset = true
}

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

func TestAtomicFile(t *testing.T) {
	dir := t.TempDir()
	f := func(name string) string { return filepath.Join(dir, name) }

	{ // successful path
		af, err := newAtomicFile(f("file0"))
		assert.NoError(t, err)
		defer af.Cancel()

		assert.Equal(t, allFiles(t, dir), []string{f("file0.tmp")})

		_, err = af.Write([]byte("hello"))
		assert.NoError(t, err)
		assert.NoError(t, af.Commit())

		assert.Equal(t, allFiles(t, dir), []string{f("file0")})

		data, err := os.ReadFile(f("file0"))
		assert.NoError(t, err)
		assert.Equal(t, string(data), "hello")
	}

	{ // cancel should clean up and commit after cancel should error
		af, err := newAtomicFile(f("file1"))
		assert.NoError(t, err)

		assert.Equal(t, allFiles(t, dir), []string{f("file0"), f("file1.tmp")})

		af.Cancel()
		assert.Error(t, af.Commit())

		assert.Equal(t, allFiles(t, dir), []string{f("file0")})
	}
}

func TestShortCollidingKeys(t *testing.T) {
	k0, k1 := newShortCollidingKeys()
	assert.Equal(t, shortKeyFrom(k0), shortKeyFrom(k1))
	assert.NotEqual(t, k0, k1)
}

func TestRewrittenIndex(t *testing.T) {
	var ri rewrittenIndex
	var recs []Record

	for i := 0; i < 1000; i++ {
		rec := newRecord(newKey())

		ri.add(rec)
		recs = append(recs, rec)
	}

	ri.sortByKey()

	for _, rec := range recs {
		i, ok := ri.findKey(rec.Key)
		assert.True(t, ok)
		assert.Equal(t, ri.records[i], rec)
	}

	for i := 0; i < 1000; i++ {
		i, ok := ri.findKey(newKey())
		assert.False(t, ok)
		assert.Equal(t, i, -1)
	}
}

func TestAtomicMap(t *testing.T) {
	var am atomicMap[int, string]

	val, ok := am.Lookup(1)
	assert.Equal(t, val, "")
	assert.False(t, ok)

	assert.True(t, am.Empty())
	am.Set(1, "one")
	assert.False(t, am.Empty())

	val, ok = am.Lookup(1)
	assert.Equal(t, val, "one")
	assert.True(t, ok)

	val, ok = am.LoadAndDelete(2)
	assert.Equal(t, val, "")
	assert.False(t, ok)

	val, ok = am.LoadAndDelete(1)
	assert.Equal(t, val, "one")
	assert.True(t, ok)

	val, ok = am.Lookup(1)
	assert.Equal(t, val, "")
	assert.False(t, ok)

	assert.True(t, am.Empty())
}

func TestMultiLRUCache_Basic(t *testing.T) {
	cache := newMultiLRUCache[string, *testCloser](3)
	defer cache.Clear()

	// Test Get with make function when cache is empty
	tc1 := &testCloser{}
	val, err := cache.Get("key1", returnConstant(tc1))
	assert.NoError(t, err)
	assert.Equal(t, val, tc1)
	assert.False(t, tc1.closed)

	// Test Put
	tc2 := &testCloser{}
	cache.Put("key1", tc2)

	// Test Get returns cached value (most recently added)
	val, err = cache.Get("key1", returnFailure(t))
	assert.NoError(t, err)
	assert.Equal(t, val, tc2)
	assert.False(t, tc2.closed)

	// Put another value for the same key
	tc3 := &testCloser{}
	cache.Put("key1", tc3)

	// Get should return the most recently added value
	val, err = cache.Get("key1", returnFailure(t))
	assert.NoError(t, err)
	assert.Equal(t, val, tc3)
	assert.False(t, tc3.closed)
}

func TestMultiLRUCache_Capacity(t *testing.T) {
	cache := newMultiLRUCache[string, *testCloser](2)
	defer cache.Clear()

	// Add items up to capacity
	tc1 := &testCloser{}
	tc2 := &testCloser{}
	cache.Put("key1", tc1)
	cache.Put("key2", tc2)

	// Add a third item - should evict the oldest (tc1)
	tc3 := &testCloser{}
	cache.Put("key3", tc3)
	assert.True(t, tc1.closed) // tc1 should be closed due to eviction

	// key1 should no longer be cached
	tc1New := &testCloser{}
	val, err := cache.Get("key1", returnConstant(tc1New))
	assert.NoError(t, err)
	assert.Equal(t, val, tc1New)

	// key2 and key3 should still be available
	val, err = cache.Get("key2", returnFailure(t))
	assert.NoError(t, err)
	assert.Equal(t, val, tc2)

	val, err = cache.Get("key3", returnFailure(t))
	assert.NoError(t, err)
	assert.Equal(t, val, tc3)
}

func TestMultiLRUCache_MultipleValuesPerKey(t *testing.T) {
	cache := newMultiLRUCache[string, *testCloser](5)
	defer cache.Clear()

	// Add multiple values for the same key
	tc1 := &testCloser{}
	tc2 := &testCloser{}
	tc3 := &testCloser{}

	cache.Put("key1", tc1)
	cache.Put("key1", tc2)
	cache.Put("key1", tc3)

	// Get should return the most recently added (LIFO order)
	val, err := cache.Get("key1", returnFailure(t))
	assert.NoError(t, err)
	assert.Equal(t, val, tc3)

	// Get again should return the second most recent
	val, err = cache.Get("key1", returnFailure(t))
	assert.NoError(t, err)
	assert.Equal(t, val, tc2)

	// Get again should return the first
	val, err = cache.Get("key1", returnFailure(t))
	assert.NoError(t, err)
	assert.Equal(t, val, tc1)

	// Key should be removed from cache now
	tcNew := &testCloser{}
	val, err = cache.Get("key1", returnConstant(tcNew))
	assert.NoError(t, err)
	assert.Equal(t, val, tcNew)
}

func TestMultiLRUCache_EvictionWithMultipleValues(t *testing.T) {
	cache := newMultiLRUCache[string, *testCloser](3)
	defer cache.Clear()

	// Add multiple values for key1
	tc1a := &testCloser{}
	tc1b := &testCloser{}
	cache.Put("key1", tc1a)
	cache.Put("key1", tc1b)

	// Add one value for key2
	tc2 := &testCloser{}
	cache.Put("key2", tc2)

	// Add one more value for key3 - should trigger eviction of oldest from key1
	tc3 := &testCloser{}
	cache.Put("key3", tc3)
	assert.True(t, tc1a.closed) // oldest value from key1 should be evicted

	// key1 should still have one value left
	val, err := cache.Get("key1", returnFailure(t))
	assert.NoError(t, err)
	assert.Equal(t, val, tc1b)
	assert.False(t, tc1b.closed)
}

func TestMultiLRUCache_Close(t *testing.T) {
	cache := newMultiLRUCache[string, *testCloser](3)

	tc1 := &testCloser{}
	tc2 := &testCloser{}
	tc3 := &testCloser{}

	cache.Put("key1", tc1)
	cache.Put("key2", tc2)
	cache.Put("key1", tc3) // multiple values for key1

	assert.False(t, tc1.closed)
	assert.False(t, tc2.closed)
	assert.False(t, tc3.closed)

	cache.Clear()

	// All cached items should be closed
	assert.True(t, tc1.closed)
	assert.True(t, tc2.closed)
	assert.True(t, tc3.closed)
}

func TestMultiLRUCache_LRUOrdering(t *testing.T) {
	cache := newMultiLRUCache[string, *testCloser](4)
	defer cache.Clear()

	tc1 := &testCloser{}
	tc2 := &testCloser{}
	tc3 := &testCloser{}
	tc4 := &testCloser{}
	tc5 := &testCloser{}

	// Fill cache
	cache.Put("key1", tc1)
	cache.Put("key2", tc2)
	cache.Put("key3", tc3)
	cache.Put("key4", tc4)

	// Access key1 to remove it and cause key2 to be the oldest
	val, err := cache.Get("key1", returnFailure(t))
	assert.NoError(t, err)
	assert.Equal(t, val, tc1)

	// Put it back so that the cache is full again
	cache.Put("key1", tc1)

	// Add a new item - should evict key2 (oldest accessed)
	cache.Put("key5", tc5)
	assert.True(t, tc2.closed)
	assert.False(t, tc1.closed) // key1 was accessed recently, so not evicted
}

func TestMultiLRUCache_MakeError(t *testing.T) {
	cache := newMultiLRUCache[string, *testCloser](3)
	defer cache.Clear()

	// Test Get with make function that returns an error
	expectedErr := errs.New("sentinel")
	val, err := cache.Get("key1", returnError(expectedErr))
	assert.Error(t, err)
	assert.Equal(t, err, expectedErr)
	assert.Nil(t, val)
}

func TestMultiLRUCache_ZeroCapacity(t *testing.T) {
	cache := newMultiLRUCache[string, *testCloser](0)
	defer cache.Clear()

	tc1 := &testCloser{}
	cache.Put("key1", tc1)

	// With zero capacity, item should be immediately closed
	assert.True(t, tc1.closed)

	// Get should always call make function
	tc2 := &testCloser{}
	val, err := cache.Get("key1", returnConstant(tc2))
	assert.NoError(t, err)
	assert.Equal(t, val, tc2)
}

func TestMultiLRUCache_SingleCapacity(t *testing.T) {
	cache := newMultiLRUCache[string, *testCloser](1)
	defer cache.Clear()

	tc1 := &testCloser{}
	tc2 := &testCloser{}

	cache.Put("key1", tc1)
	assert.False(t, tc1.closed)

	// Adding second item should evict first
	cache.Put("key2", tc2)
	assert.True(t, tc1.closed)
	assert.False(t, tc2.closed)

	// Verify key2 is cached
	val, err := cache.Get("key2", returnFailure(t))
	assert.NoError(t, err)
	assert.Equal(t, val, tc2)
}

//
// test helpers
//

func touch(t *testing.T, name ...string) {
	path := filepath.Join(name...)
	dir := filepath.Dir(path)
	assert.NoError(t, os.MkdirAll(dir, 0755))
	assert.NoError(t, os.WriteFile(path, nil, 0644))
}

// allFiles recursively collects all files in the given directory and returns
// their full path.
func allFiles(t *testing.T, dir string) (paths []string) {
	all := func(name string) (struct{}, bool) { return struct{}{}, true }
	for parsed, err := range parseFiles(all, dir) {
		assert.NoError(t, err)
		paths = append(paths, parsed.path)
	}
	return paths
}

func assertClose(t testing.TB, cl io.Closer) { assert.NoError(t, cl.Close()) }

func forAllTables[T interface {
	Run(string, func(T)) bool
}](t T, fn func(T, Config)) {
	run := func(t T, kind TableKind, mmap bool) {
		t.Run(fmt.Sprintf("tbl=%s/mmap=%v", kind, mmap), func(t T) {
			fn(t, CreateDefaultConfig(kind, mmap))
		})
	}

	run(t, TableKind_HashTbl, false)
	run(t, TableKind_MemTbl, false)
	if platform.MmapSupported {
		run(t, TableKind_HashTbl, true)
		run(t, TableKind_MemTbl, true)
	}
}

func ifFailed(t testing.TB, fn func()) {
	if t.Failed() {
		fn()
	}
}

func withEntries(t *testing.T, entries int, keys *[]Key) WithConstructor {
	return WithConstructor(func(tc TblConstructor) {
		ctx := t.Context()
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
		ctx := t.Context()
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

func newMemoryLogger() *zap.Logger {
	return zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.AddSync(new(bytes.Buffer)),
		zapcore.DebugLevel,
	))
}

type testCloser struct {
	closed bool
}

func (tc *testCloser) Close() error {
	tc.closed = true
	return nil
}

func returnConstant(val *testCloser) func(string) (*testCloser, error) {
	return func(string) (*testCloser, error) { return val, nil }
}

func returnError(err error) func(string) (*testCloser, error) {
	return func(string) (*testCloser, error) { return nil, err }
}

func returnFailure(t testing.TB) func(string) (*testCloser, error) {
	return func(string) (*testCloser, error) {
		t.Helper()
		t.Fatal("should not be called")
		return nil, nil
	}
}

//
// generic table
//

type testTbl struct {
	t testing.TB
	Tbl

	cfg Config
}

func newTestTbl(t testing.TB, cfg Config, lrec uint64, opts ...any) *testTbl {
	fh, err := os.CreateTemp(t.TempDir(), "tbl")
	assert.NoError(t, err)
	defer ifFailed(t, func() { _ = fh.Close() })

	cons, err := CreateTable(t.Context(), fh, lrec, 0, cfg.TableDefaultKind.Kind, cfg)
	assert.NoError(t, err)
	defer cons.Cancel()
	checkOptions(opts, func(tc WithConstructor) { tc(cons) })
	tbl, err := cons.Done(t.Context())
	assert.NoError(t, err)

	return &testTbl{t: t, Tbl: tbl, cfg: cfg}
}

func (tbl *testTbl) Close() { assert.NoError(tbl.t, tbl.Tbl.Close()) }

func (tbl *testTbl) AssertReopen() {
	assert.NoError(tbl.t, tbl.Tbl.Close())

	fh, err := os.OpenFile(tbl.Handle().Name(), os.O_RDWR, 0)
	assert.NoError(tbl.t, err)

	h, _, err := OpenTable(tbl.t.Context(), fh, tbl.cfg)
	assert.NoError(tbl.t, err)

	tbl.Tbl = h
}

func (tbl *testTbl) AssertInsertRecord(rec Record) {
	ok, err := tbl.Insert(tbl.t.Context(), rec)
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
	r, ok, err := tbl.Lookup(tbl.t.Context(), k)
	assert.NoError(tbl.t, err)
	assert.True(tbl.t, ok)
	return r
}

func (tbl *testTbl) AssertLookupMiss(k Key) {
	_, ok, err := tbl.Lookup(tbl.t.Context(), k)
	assert.NoError(tbl.t, err)
	assert.False(tbl.t, ok)
}

//
// hashtbl
//

type testHashTbl struct {
	t testing.TB
	*HashTbl

	cfg MmapCfg
}

func newTestHashTbl(t testing.TB, cfg MmapCfg, lrec uint64, opts ...any) *testHashTbl {
	fh, err := os.CreateTemp(t.TempDir(), "hashtbl")
	assert.NoError(t, err)
	defer ifFailed(t, func() { _ = fh.Close() })

	cons, err := CreateHashTbl(t.Context(), fh, lrec, 0, cfg)
	assert.NoError(t, err)
	defer cons.Cancel()
	checkOptions(opts, func(tc WithConstructor) { tc(cons) })
	h, err := cons.Done(t.Context())
	assert.NoError(t, err)

	return &testHashTbl{t: t, HashTbl: h.(*HashTbl), cfg: cfg}
}

func (th *testHashTbl) Close() { assert.NoError(th.t, th.HashTbl.Close()) }

func (th *testHashTbl) AssertReopen() {
	assert.NoError(th.t, th.HashTbl.Close())

	fh, err := os.OpenFile(th.fh.Name(), os.O_RDWR, 0)
	assert.NoError(th.t, err)

	h, _, err := OpenHashTbl(th.t.Context(), fh, th.cfg)
	assert.NoError(th.t, err)

	th.HashTbl = h
}

func (th *testHashTbl) AssertInsertRecord(rec Record) {
	ok, err := th.Insert(th.t.Context(), rec)
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
	r, ok, err := th.Lookup(th.t.Context(), k)
	assert.NoError(th.t, err)
	assert.True(th.t, ok)
	return r
}

func (th *testHashTbl) AssertLookupMiss(k Key) {
	_, ok, err := th.Lookup(th.t.Context(), k)
	assert.NoError(th.t, err)
	assert.False(th.t, ok)
}

//
// memtbl
//

type testMemTbl struct {
	t testing.TB
	*MemTbl

	cfg MmapCfg
}

func newTestMemTbl(t testing.TB, cfg MmapCfg, lrec uint64, opts ...any) *testMemTbl {
	fh, err := os.CreateTemp(t.TempDir(), "memtbl")
	assert.NoError(t, err)
	defer ifFailed(t, func() { _ = fh.Close() })

	cons, err := CreateMemTbl(t.Context(), fh, lrec, 0, cfg)
	assert.NoError(t, err)
	defer cons.Cancel()
	checkOptions(opts, func(tc WithConstructor) { tc(cons) })
	m, err := cons.Done(t.Context())
	assert.NoError(t, err)

	return &testMemTbl{t: t, MemTbl: m.(*MemTbl), cfg: cfg}
}

func (tm *testMemTbl) Close() { assert.NoError(tm.t, tm.MemTbl.Close()) }

func (tm *testMemTbl) AssertReopen() {
	assert.NoError(tm.t, tm.MemTbl.Close())

	fh, err := os.OpenFile(tm.fh.Name(), os.O_RDWR, 0)
	assert.NoError(tm.t, err)

	m, _, err := OpenMemTbl(tm.t.Context(), fh, tm.cfg)
	assert.NoError(tm.t, err)

	tm.MemTbl = m
}

func (tm *testMemTbl) AssertInsertRecord(rec Record) {
	ok, err := tm.Insert(tm.t.Context(), rec)
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
	r, ok, err := tm.Lookup(tm.t.Context(), k)
	assert.NoError(tm.t, err)
	assert.True(tm.t, ok)
	return r
}

func (tm *testMemTbl) AssertLookupMiss(k Key) {
	_, ok, err := tm.Lookup(tm.t.Context(), k)
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

func newTestStore(t testing.TB, cfg Config) *testStore {
	s, err := NewStore(t.Context(), cfg, t.TempDir(), "", newMemoryLogger())
	assert.NoError(t, err)

	ts := &testStore{t: t, Store: s, today: s.today()}

	s.today = func() uint32 { return ts.today }

	return ts
}

func (ts *testStore) Close() { assert.NoError(ts.t, ts.Store.Close()) }

func (ts *testStore) AssertReopen() {
	assert.NoError(ts.t, ts.Store.Close())

	s, err := NewStore(ts.t.Context(), ts.cfg, ts.logsPath, ts.tablePath, ts.log)
	assert.NoError(ts.t, err)

	s.today = func() uint32 { return ts.today }

	ts.Store = s
}

func (ts *testStore) AssertCompact(
	shouldTrash func(context.Context, Key, time.Time) bool,
	restore time.Time,
) {
	assert.NoError(ts.t, ts.Compact(ts.t.Context(), shouldTrash, restore))
}

func (ts *testStore) AssertCreate(opts ...any) Key {
	var expires time.Time
	checkOptions(opts, func(t WithTTL) { expires = time.Time(t) })

	key := newKey()
	checkOptions(opts, func(t WithKey) { key = Key(t) })

	data := key[:]
	checkOptions(opts, func(t WithDataSize) { data = dataSizedFromKey(key, int(t)) })
	checkOptions(opts, func(t WithData) { data = []byte(t) })

	wr, err := ts.Create(ts.t.Context(), key, expires)
	assert.NoError(ts.t, err)
	assert.Equal(ts.t, wr.Size(), 0)

	_, err = wr.Write(data)
	assert.NoError(ts.t, err)

	assert.Equal(ts.t, wr.Size(), len(data))
	assert.NoError(ts.t, wr.Close())

	return key
}

func (ts *testStore) AssertRead(key Key, opts ...any) {
	r, err := ts.Read(ts.t.Context(), key)
	assert.NoError(ts.t, err)
	assert.NotNil(ts.t, r)

	checkOptions(opts, func(rt AssertTrash) {
		assert.Equal(ts.t, rt, r.Trash())
	})

	assert.Equal(ts.t, r.Key(), key)

	data := key[:]
	checkOptions(opts, func(t WithDataSize) { data = dataSizedFromKey(key, int(t)) })
	checkOptions(opts, func(t WithData) { data = []byte(t) })

	assert.Equal(ts.t, r.Size(), len(data))
	assert.NoError(ts.t, iotest.TestReader(r, data))

	checkOptions(opts, func(wr WithRevive) {
		if wr {
			assert.NoError(ts.t, r.Revive(ts.t.Context()))
		}
	})

	assert.NoError(ts.t, r.Close())
}

func (ts *testStore) AssertNotExist(key Key) {
	r, err := ts.Read(ts.t.Context(), key)
	assert.NoError(ts.t, err)
	assert.Nil(ts.t, r)
}

func (ts *testStore) AssertExist(key Key) {
	_, ok, err := ts.tbl.Lookup(ts.t.Context(), key)
	assert.NoError(ts.t, err)
	assert.True(ts.t, ok)
}

func (ts *testStore) LogFile(key Key) uint64 {
	rec, ok, err := ts.tbl.Lookup(ts.t.Context(), key)
	assert.NoError(ts.t, err)
	assert.True(ts.t, ok)
	return rec.Log
}

//
// db
//

type testDB struct {
	t testing.TB
	*DB

	cfg Config
}

func newTestDB(
	t testing.TB,
	cfg Config,
	dead func(context.Context, Key, time.Time) bool,
	restore func(context.Context) time.Time,
) *testDB {
	db, err := New(t.Context(), cfg, t.TempDir(), "", newMemoryLogger(), dead, restore)
	assert.NoError(t, err)

	td := &testDB{t: t, DB: db, cfg: cfg}

	return td
}

func (td *testDB) Close() { assert.NoError(td.t, td.DB.Close()) }

func (td *testDB) AssertReopen() {
	assert.NoError(td.t, td.DB.Close())

	db, err := New(td.t.Context(), td.cfg, td.logsPath, td.tablePath, td.log, td.shouldTrash, td.lastRestore)
	assert.NoError(td.t, err)

	td.DB = db
}

func (td *testDB) AssertCreate(opts ...any) Key {
	var expires time.Time
	checkOptions(opts, func(t WithTTL) { expires = time.Time(t) })

	key := newKey()
	checkOptions(opts, func(t WithKey) { key = Key(t) })

	data := key[:]
	checkOptions(opts, func(t WithDataSize) { data = dataSizedFromKey(key, int(t)) })
	checkOptions(opts, func(t WithData) { data = []byte(t) })

	wr, err := td.Create(td.t.Context(), key, expires)
	assert.NoError(td.t, err)
	assert.Equal(td.t, wr.Size(), 0)

	_, err = wr.Write(data)
	assert.NoError(td.t, err)

	assert.Equal(td.t, wr.Size(), len(data))
	assert.NoError(td.t, wr.Close())

	return key
}

func (td *testDB) AssertRead(key Key, opts ...any) {
	r, err := td.Read(td.t.Context(), key)
	assert.NoError(td.t, err)
	assert.NotNil(td.t, r)

	checkOptions(opts, func(rt AssertTrash) {
		assert.Equal(td.t, rt, r.Trash())
	})

	assert.NoError(td.t, iotest.TestReader(r, key[:]))
	assert.NoError(td.t, r.Close())
}

func (td *testDB) AssertCompact() {
	assert.NoError(td.t, td.Compact(td.t.Context()))
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

func rngFromKey(key Key) *mwc.T {
	return mwc.New(
		binary.LittleEndian.Uint64(key[0:8]),
		binary.LittleEndian.Uint64(key[8:16]),
	)
}

func dataFromKey(key Key) []byte {
	return dataSizedFromKey(key, rngFromKey(key).Intn(1024))
}

func dataSizedFromKey(key Key, size int) []byte {
	buf := make([]byte, size)
	_, _ = rngFromKey(key).Read(buf)
	return buf
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
