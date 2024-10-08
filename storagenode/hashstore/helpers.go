// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"encoding/binary"
	"os"
	"sync"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
)

// Key is the key space operated on by the store.
type Key = storj.PieceID

func keyIndex(k *Key) uint64 {
	return binary.LittleEndian.Uint64(k[0:8])
}

func timeToDateDown(t time.Time) uint32 { return uint32(t.Unix() / 86400) }
func timeToDateUp(t time.Time) uint32   { return uint32((t.Unix() + 86400 - 1) / 86400) }
func dateToTime(d uint32) time.Time     { return time.Unix(int64(d)*86400, 0) }

func fileSize(fh *os.File) (int64, error) {
	if fi, err := fh.Stat(); err != nil {
		return 0, errs.Wrap(err)
	} else {
		return fi.Size(), nil
	}
}

type flag bool

func (f flag) get() bool { return bool(f) }
func (f *flag) set() (old bool) {
	old, *f = f.get(), true
	return old
}

type atomicMap[K comparable, V any] struct {
	_ [0]func() (*K, *V) // prevent equality and unsound conversions
	m sync.Map
}

func (a *atomicMap[K, V]) Delete(k K)   { a.m.Delete(k) }
func (a *atomicMap[K, V]) Set(k K, v V) { a.m.Store(k, v) }
func (a *atomicMap[K, V]) Range(fn func(K, V) bool) {
	a.m.Range(func(k, v any) bool { return fn(k.(K), v.(V)) })
}

func (a *atomicMap[K, V]) LoadAndDelete(k K) (V, bool) {
	v, ok := a.m.LoadAndDelete(k)
	if !ok {
		return *new(V), false
	}
	return v.(V), true
}

func (a *atomicMap[K, V]) Lookup(k K) (V, bool) {
	v, ok := a.m.Load(k)
	if !ok {
		return *new(V), false
	}
	return v.(V), true
}

type semaphore struct {
	ch chan struct{}
}

func newSemaphore(cap int) *semaphore {
	return &semaphore{ch: make(chan struct{}, cap)}
}

func (s *semaphore) Cap() int            { return cap(s.ch) }
func (s *semaphore) Chan() chan struct{} { return s.ch }
func (s *semaphore) Lock()               { s.ch <- struct{}{} }
func (s *semaphore) Unlock()             { <-s.ch }
func (s *semaphore) TryLock() bool {
	select {
	case s.ch <- struct{}{}:
		return true
	default:
		return false
	}
}
