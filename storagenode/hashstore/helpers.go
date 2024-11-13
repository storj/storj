// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/spacemonkeygo/monkit/v3"

	"storj.io/common/storj"
	"storj.io/drpc/drpcsignal"
)

var mon = monkit.Package()

// Key is the key space operated on by the store.
type Key = storj.PieceID

func safeDivide(x, y float64) float64 {
	if y == 0 {
		return 0
	}
	return x / y
}

//
// date/time helpers
//

// saturatingUint23 returns the uint32 value of x, saturating to the maximum if the conversion
// would overflow or underflow. This is used to put a maximum date on expiration times and so that
// if someone passes in an expiration way in the future it doesn't end up in the past.
func saturatingUint23(x int64) uint32 {
	if uint64(x) >= 1<<23-1 {
		return 1<<23 - 1
	}
	return uint32(x)
}

func timeToDateDown(t time.Time) uint32 { return saturatingUint23(t.Unix() / 86400) }
func timeToDateUp(t time.Time) uint32   { return saturatingUint23((t.Unix() + 86400 - 1) / 86400) }
func dateToTime(d uint32) time.Time     { return time.Unix(int64(d)*86400, 0).UTC() }

//
// simple boolean flag that can be used to set once
//

type flag bool

func (f flag) get() bool { return bool(f) }
func (f *flag) set() (old bool) {
	old, *f = f.get(), true
	return old
}

//
// generic wrapper around sync.Map
//

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

//
// context/signal aware mutex
//

type mutex struct {
	ch chan struct{}
}

func newMutex() *mutex {
	return &mutex{ch: make(chan struct{}, 1)}
}

func (s *mutex) WaitLock() { s.ch <- struct{}{} }

func (s *mutex) TryLock() bool {
	select {
	case s.ch <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s *mutex) Lock(ctx context.Context, closed *drpcsignal.Signal) error {
	if err := ctx.Err(); err != nil {
		return err
	} else if err := closed.Err(); err != nil {
		return err
	}
	select {
	case s.ch <- struct{}{}:
		return nil
	case <-closed.Signal():
		return closed.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *mutex) Unlock() { <-s.ch }

//
// context/signal aware rw-mutex
//

type rwMutex struct {
	wmu *mutex
	rmu *mutex
	rw  sync.RWMutex
	rs  atomic.Int64
}

func newRWMutex() *rwMutex {
	return &rwMutex{
		wmu: newMutex(),
		rmu: newMutex(),
	}
}

func (m *rwMutex) RLock(ctx context.Context, closed *drpcsignal.Signal) error {
	if err := ctx.Err(); err != nil {
		return err
	} else if err := closed.Err(); err != nil {
		return err
	}
	for {
		if m.rw.TryRLock() {
			if m.rs.Add(1) == 1 {
				m.rmu.WaitLock() // should not block because rs ensures we're the first reader.
			}
			return nil
		}
		if err := m.wmu.Lock(ctx, closed); err != nil {
			return err
		}
		m.wmu.Unlock()
	}
}

func (m *rwMutex) RUnlock() {
	if m.rs.Add(-1) == 0 {
		m.rmu.Unlock()
	}
	m.rw.RUnlock()
}

func (m *rwMutex) WaitLock() {
	m.rw.Lock()
	m.wmu.WaitLock()
}

func (m *rwMutex) Lock(ctx context.Context, closed *drpcsignal.Signal) error {
	if err := ctx.Err(); err != nil {
		return err
	} else if err := closed.Err(); err != nil {
		return err
	}
	for {
		if m.rw.TryLock() {
			m.wmu.WaitLock() // should not block because rwmutex is locked.
			return nil
		}
		if err := m.rmu.Lock(ctx, closed); err != nil {
			return err
		}
		m.rmu.Unlock()
	}
}

func (m *rwMutex) Unlock() {
	m.wmu.Unlock()
	m.rw.Unlock()
}

//
// filesystem helpers
//

func fileSize(fh *os.File) (int64, error) {
	if fi, err := fh.Stat(); err != nil {
		return 0, Error.Wrap(err)
	} else {
		return fi.Size(), nil
	}
}

func syncDirectory(dir string) {
	if fh, err := os.Open(dir); err == nil {
		_ = fh.Sync()
		_ = fh.Close()
	}
}

//
// atomic file creation helper
//

type atomicFile struct {
	*os.File

	dir  string
	name string

	mu        sync.Mutex // protects the following fields
	canceled  flag
	committed flag
}

func newAtomicFile(dir string, name string) (*atomicFile, error) {
	fh, err := os.CreateTemp(dir, name+"-*.tmp")
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return &atomicFile{
		File: fh,

		dir:  dir,
		name: name,
	}, nil
}

func (a *atomicFile) Commit() (err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.committed.set() || a.canceled.get() {
		return nil
	}

	// attempt to unlink the temporary file if there are any commit errors.
	defer func() {
		if err != nil {
			_ = a.Close()
			_ = os.Remove(a.Name())
		}
	}()

	if err := a.Sync(); err != nil {
		return Error.Wrap(err)
	}
	if err := os.Rename(a.Name(), filepath.Join(a.dir, a.name)); err != nil {
		return Error.Wrap(err)
	}

	return nil
}

func (a *atomicFile) Cancel() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.canceled.set() || a.committed.get() {
		return
	}

	_ = a.Close()
	_ = os.Remove(a.Name())
}
