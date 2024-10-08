// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"container/heap"
	"context"
	"fmt"
	"io"
	"math/bits"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/context2"
	"storj.io/drpc/drpcsignal"
)

const (
	store_minTableSize = 10 // log_2 of number of records for smallest hash table

	compaction_MaxLogSize    = 10 << 30 // max size of a log file
	compaction_AliveFraction = 0.75     // if the log file is not this alive, compact it
	compaction_ExpiresDays   = 7        // number of days to keep trash records around
)

// store is a hash table based key-value store with compaction.
type store struct {
	// immutable data
	dir   string        // directory containing store files
	nlogs int           // number of log files for active writes
	log   *zap.Logger   // logger for unhandleable errors
	today func() uint32 // hook for getting the current timestamp

	closed drpcsignal.Signal // closed state
	cloMu  sync.Mutex        // synchronizes closing

	activeSem *semaphore // semaphore of active writes to log files
	compactMu *semaphore // held during compaction to ensure only 1 compaction at a time
	reviveMu  *semaphore // held during revival to ensure only 1 object is revived from trash at a time

	maxid atomic.Uint32 // maximum log file id

	hmu sync.Mutex // enforces atomic access to the heap
	lfh logHeap    // heap of log files sorted by size ready to be written into

	rmu sync.RWMutex                // protects consistency of lfs and tbl
	lfs atomicMap[uint32, *logFile] // all log files
	tbl *hashTbl                    // hash table of records
}

func newStore(dir string, nlogs int, log *zap.Logger) (_ *store, err error) {
	s := &store{
		dir:   dir,
		nlogs: nlogs,
		log:   log,
		today: func() uint32 { return timeToDateDown(time.Now()) },

		activeSem: newSemaphore(nlogs),
		compactMu: newSemaphore(1),
		reviveMu:  newSemaphore(1),
	}

	// clean up any old temp files from compaction.
	if files, err := filepath.Glob(filepath.Join(dir, "hashtbl-*.tmp")); err != nil {
		return nil, errs.Wrap(err)
	} else {
		for _, file := range files {
			_ = os.Remove(file)
		}
	}

	// if we have any errors, close all the log files.
	defer func() {
		if err != nil {
			s.lfs.Range(func(_ uint32, lf *logFile) bool {
				lf.Close()
				return true
			})
		}
	}()

	// open all the log files in the directory and sort them by size.
	logs, err := filepath.Glob(filepath.Join(dir, "log-*"))
	if err != nil {
		return nil, errs.Wrap(err)
	}

	for _, log := range logs {
		if len(log) < 12 {
			continue
		}

		fh, err := os.OpenFile(log, os.O_RDWR, 0)
		if err != nil {
			return nil, errs.Wrap(err)
		}

		size, err := fh.Seek(0, io.SeekEnd)
		if err != nil {
			_ = fh.Close()
			return nil, errs.New("unable to seek name=%q: %w", log, err)
		}

		id64, err := strconv.ParseUint(log[len(log)-8:], 16, 32)
		if err != nil {
			_ = fh.Close()
			return nil, errs.New("unable to parse name=%q: %w", log, err)
		}
		id := uint32(id64)

		if maxid := s.maxid.Load(); id > maxid {
			s.maxid.Store(id)
		}
		s.lfs.Set(id, newLogFile(fh, id, uint64(size)))
	}

	// now that lfs is populated, initialize the heap of log files for writing into.
	s.initializeHeap()

	// try to open the existing hash table, creating a new one if necessary.
	fh, err := os.OpenFile(filepath.Join(dir, "hashtbl"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, errs.New("unable to open/create initial hashtbl: %w", err)
	}
	defer func() {
		if err != nil {
			_ = fh.Close()
		}
	}()

	if err := flock(fh); err != nil {
		return nil, errs.New("unable to exclusively lock hashtbl: %w", err)
	}

	// compute the number of records from the file size of the hash table.
	size, err := fileSize(fh)
	if err != nil {
		return nil, errs.New("unable to determine hashtbl size: %w", err)
	}

	// if the size of the hashtbl is zero, assume it is empty and allocate a new one.
	if size == 0 {
		size = 1 << store_minTableSize * rSize
		if err := fh.Truncate(size); err != nil {
			return nil, errs.New("unable to allocate initial hashtbl: %w", err)
		}
	}

	// compute the lrec from the size.
	lrec := uint64(bits.Len64(uint64(size)/rSize) - 1)

	// sanity check that our lrec is correct.
	if 1<<lrec*rSize != size {
		return nil, errs.New("lrec calculation mismatch: size=%d lrec=%d", size, lrec)
	}

	// set up the hash table to use the handle.
	s.tbl, err = newHashTbl(fh, lrec, true)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return s, nil
}

func (s *store) acquireSemaphore(ctx context.Context, sem *semaphore) error {
	// check if we're already closed so we don't have to worry about select nondeterminism: a closed
	// store or already canceled context will definitely error.
	if err := s.closed.Err(); err != nil {
		return err
	} else if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-s.closed.Signal():
		return s.closed.Err()
	case <-ctx.Done():
		return ctx.Err()
	case sem.Chan() <- struct{}{}:
		return nil
	}
}

func (s *store) createLogFile() (*logFile, error) {
	id := s.maxid.Add(1)
	path := filepath.Join(s.dir, fmt.Sprintf("log-%08x", id))
	fh, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	lf := newLogFile(fh, id, 0)
	s.lfs.Set(id, lf)
	return lf, nil
}

func (s *store) initializeHeap() {
	s.hmu.Lock()
	defer s.hmu.Unlock()

	// collect all of the writable log files into the heap.
	s.lfh = s.lfh[:0]
	s.lfs.Range(func(_ uint32, lf *logFile) bool {
		if lf.size < compaction_MaxLogSize {
			s.lfh.Push(lf)
		}
		return true
	})
	heap.Init(&s.lfh)
}

func (s *store) replaceLogFile(lf *logFile) {
	s.hmu.Lock()
	defer s.hmu.Unlock()

	// only put the file back into the heap if it's not too big.
	if lf.size < compaction_MaxLogSize {
		heap.Push(&s.lfh, lf)
	}

	s.activeSem.Unlock()
}

func (s *store) tryPopLogFile() *logFile {
	s.hmu.Lock()
	defer s.hmu.Unlock()

	if s.lfh.Len() == 0 {
		return nil
	}
	return heap.Pop(&s.lfh).(*logFile)
}

func (s *store) acquireLogFile() (*logFile, error) {
	if lf := s.tryPopLogFile(); lf != nil {
		return lf, nil
	}
	return s.createLogFile()
}

func (s *store) addRecord(rec record) error {
	ok, err := s.tbl.Insert(rec)
	if err != nil {
		return errs.Wrap(err)
	} else if !ok {
		return errs.New("hash table is full")
	} else {
		return nil
	}
}

// Load returns the estimated load factor of the hash table. If it's too large, a Compact call is
// indicated.
func (s *store) Load() float64 {
	s.rmu.RLock()
	defer s.rmu.RUnlock()
	return s.tbl.Load()
}

// NumSet returns the estimated number of keys in the hash table.
func (s *store) NumSet() uint64 {
	s.rmu.RLock()
	defer s.rmu.RUnlock()
	return s.tbl.NumSet()
}

// Close interrupts any compactions and closes the store.
func (s *store) Close() {
	s.cloMu.Lock()
	defer s.cloMu.Unlock()

	if !s.closed.Set(errs.New("store closed")) {
		return
	}

	// acquire the compaction lock to ensure no compactions are in progress. setting s.closed should
	// ensure that any ongoing compaction exits promptly.
	s.compactMu.Lock()
	defer s.compactMu.Unlock()

	// consume all of the active write slots. this ensures no writes are active and none can start.
	for i := 0; i < s.activeSem.Cap(); i++ {
		s.activeSem.Lock()
	}

	// we can now close all of the resources. this may interrupt reads, but that's ok.
	s.lfs.Range(func(id uint32, lf *logFile) bool {
		s.lfs.Delete(id)
		lf.Close()
		return true
	})
	s.tbl.Close()
	s.lfh = nil
}

// Create returns a Handle that writes data to the store. The error on Close must be checked.
// Expires is when the data expires, or zero if it never expires.
func (s *store) Create(ctx context.Context, key Key, expires time.Time) (*Writer, error) {
	if err := s.acquireSemaphore(ctx, s.activeSem); err != nil {
		return nil, err
	}

	w, err := s.writerForKey(key, expires)
	if err != nil {
		s.activeSem.Unlock()
		return nil, errs.Wrap(err)
	}

	return w, nil
}

func (s *store) writerForKey(key Key, expires time.Time) (*Writer, error) {
	// try to acquire the log file
	lf, err := s.acquireLogFile()
	if err != nil {
		return nil, errs.Wrap(err)
	}

	// don't trust lf.size to be the actual offset of the cursor into the file. there could be
	// bugs around error paths in replacing files and it would be unfortunate if we thought the
	// position the data started at was not the actual position we wrote to.
	offset, err := lf.fh.Seek(0, io.SeekCurrent)
	if err != nil {
		// don't replace the log file into the heap if seek failed. it will get put back into
		// the heap once compaction happens if this is just a transient error.
		_ = lf
		return nil, errs.Wrap(err)
	}

	var exp expiration
	if !expires.IsZero() {
		exp = newExpiration(timeToDateUp(expires), false)
	}

	return &Writer{
		store: s,
		lf:    lf,
		rec: record{
			key:     key,
			offset:  uint64(offset),
			log:     lf.id,
			created: s.today(),
			expires: exp,
		},
	}, nil
}

// Read returns a Reader that reads data from the store. The Reader will be nil if the key does not
// exist.
func (s *store) Read(ctx context.Context, key Key) (*Reader, error) {
	// check if we're already closed so we don't have to worry about select nondeterminism: a
	// closed store will definitely error.
	if err := s.closed.Err(); err != nil {
		return nil, err
	} else if err := ctx.Err(); err != nil {
		return nil, err
	}

	// ensure that tbl and lfs are consistent
	s.rmu.RLock()
	defer s.rmu.RUnlock()

	if rec, ok, err := s.tbl.Lookup(key); err != nil {
		return nil, errs.Wrap(err)
	} else if !ok {
		return nil, nil
	} else {
		return s.readerForRecord(ctx, rec)
	}
}

func (s *store) readerForRecord(ctx context.Context, rec record) (*Reader, error) {
	lf, ok := s.lfs.Lookup(rec.log)
	if !ok {
		return nil, errs.New("record points to unknown log file rec=%v", rec)
	}
	if !lf.Acquire() {
		return nil, errs.New("unable to acquire log file for reading rec=%v", rec)
	}

	if rec.expires.trash() {
		if s.log != nil {
			s.log.Warn("trashed record was read",
				zap.String("record", rec.String()),
			)
		}

		if err := s.reviveRecord(ctx, lf, rec); err != nil {
			if s.log != nil {
				s.log.Error("unable to revive record",
					zap.String("record", rec.String()),
					zap.Error(err),
				)
			}
		}
	}

	return newLogReader(lf, rec), nil
}

func (s *store) reviveRecord(ctx context.Context, lf *logFile, rec record) (err error) {
	// we don't want to respect cancelling if the reader for the trashed piece goes away. we know
	// it was trashed so we should revive it no matter what.
	ctx = context2.WithoutCancellation(ctx)

	if err := s.acquireSemaphore(ctx, s.reviveMu); err != nil {
		return err
	}
	defer s.reviveMu.Unlock()

	// easy case: if we can acquire the compaction mutex, then we can be sure that no compaction is
	// ongoing so we can just write the record with a cleared expires field. we're already holding
	// the rmu so the tbl will not be mutated and the record points at a valid log file.
	if s.compactMu.TryLock() {
		defer s.compactMu.Unlock()

		rec.expires = 0
		return s.addRecord(rec)
	}

	// uh oh, a compaction is potentially ongoing (it may have just finished right after the TryLock
	// failed). we're still holding rmu, so we can't wait on the compaction because it could be
	// trying to acquire rmu. fortunately, because we have a handle to the log file, we can always
	// read the piece even if it gets fully deleted by compaction. the last tricky bit is to decide
	// if we need to re-write the piece or if we can get away with updating the record like above.
	// once we have acquired a write slot, we can be sure that no compaction is ongoing so we can
	// check the table to see if the record matches as it usually will.

	// 0. drop the mutex so compaction can proceed. this may invalidate the log file pointed at by
	// rec but we have a handle to it so we'll still be able to read it.
	s.rmu.RUnlock()
	defer s.rmu.RLock()

	// 1. acquire a write slot, ensuring that no compaction is ongoing and we can write to a log
	// if necessary. once we have a writer, we know the state of the hash table and logs can only
	// be added to.
	w, err := s.Create(ctx, rec.key, time.Time{})
	if err != nil {
		return errs.Wrap(err)
	}
	defer w.Cancel()

	// 2. find the current state of the record. if found, we can just update the expiration and be
	// happy. as noted in 1, we're safe to do a lookup into s.tbl here even without the rmu held
	// because we know no compaction is ongoing due to having a writer acquired, and compaction is
	// the only thing that does more than just add to the hash table.
	if tmp, ok, err := s.tbl.Lookup(rec.key); err == nil && ok {
		if tmp.expires == 0 {
			return nil
		}

		tmp.expires = 0
		return s.addRecord(tmp)
	}

	// 3. otherwise, we either had an error looking up the current record, or the entry got fully
	// deleted, and the open file handle is the last remaining evidence that it exists, so we have
	// to rewrite it. note that we purposefully do not close the log reader because after this
	// function exits, a log reader will be created and returned to the user.
	_, err = io.Copy(w, newLogReader(lf, rec))
	if err != nil {
		return errs.Wrap(err)
	}
	if err := w.Close(); err != nil {
		return errs.Wrap(err)
	}

	return nil
}

func (s *store) stopAndWaitForWriters(ctx context.Context) (_ func(), err error) {
	// keep track of how many write tokens we have acquired in case of partial exit due to close or
	// context cancel.
	acquired := 0

	// resume is a function that releases all of the write tokens we have acquired.
	resume := func() {
		for i := 0; i < acquired; i++ {
			s.activeSem.Unlock()
		}
	}

	// acquire all of the write tokens to ensure no writers are active and none can start.
	for i := 0; i < s.activeSem.Cap(); i++ {
		if err := s.acquireSemaphore(ctx, s.activeSem); err != nil {
			resume()
			return nil, err
		}
		acquired++
	}

	return resume, nil
}

// Compact removes keys and files that are definitely expired, and marks keys that are determined
// trash by the callback to expire in the future. It also rewrites any log files that have too much
// dead data.
func (s *store) Compact(
	ctx context.Context,
	shouldTrash func(ctx context.Context, key Key, created time.Time) (bool, error),
	lastRestore time.Time,
) error {
	// ensure only one compaction at a time.
	if err := s.acquireSemaphore(ctx, s.compactMu); err != nil {
		return err
	}
	defer s.compactMu.Unlock()

	// 0. stop all writers from starting and wait for all current writers to finish.
	resume, err := s.stopAndWaitForWriters(ctx)
	if err != nil {
		return err
	}
	defer resume()

	// 1. collect statistics about the hash table and how live each of the log files are.
	today := s.today()
	var restore uint32
	if !lastRestore.IsZero() {
		restore = timeToDateUp(lastRestore)
	}

	restored := func(e expiration) bool {
		// if the expiration is trash and it is before the restore time, it is restored.
		return e.trash() && e.time() <= restore+compaction_ExpiresDays
	}

	expired := func(e expiration) bool {
		// if the record does not have an expiration, it is not expired.
		if e == 0 {
			return false
		}
		// if it is not currently after the expiration time, it is not expired.
		if today <= e.time() {
			return false
		}
		// if it has been restored, it is not expired.
		if restored(e) {
			return false
		}
		return true
	}

	nset := uint64(0)
	used := make(map[uint32]uint64)
	rerr := error(nil)
	s.tbl.Range(func(rec record, err error) bool {
		rerr = func() error {
			if err != nil {
				return errs.Wrap(err)
			} else if err := ctx.Err(); err != nil {
				return err
			} else if err := s.closed.Err(); err != nil {
				return err
			}

			if expired(rec.expires) {
				return nil
			}

			nset++
			used[rec.log] += uint64(rec.length)
			return nil
		}()
		return rerr == nil
	})
	if rerr != nil {
		return rerr
	}

	// 2. using the information, determine which logs need compaction.
	compact := make(map[uint32]bool)
	s.lfs.Range(func(id uint32, lf *logFile) bool {
		rerr = func() error {
			if err := ctx.Err(); err != nil {
				return err
			} else if err := s.closed.Err(); err != nil {
				return err
			}

			size, err := fileSize(lf.fh)
			if err != nil {
				return errs.Wrap(err)
			} else if size == 0 { // if the log is empty, just leave it alone
				return nil
			}

			if float64(used[id])/float64(size) < compaction_AliveFraction {
				compact[id] = true
			}
			return nil
		}()
		return rerr == nil
	})
	if rerr != nil {
		return rerr
	}

	// 3. create a new hash table and size it so that it targets just under a 0.25 load factor.
	lrec := uint64(bits.Len64(nset)) + 1
	if lrec < store_minTableSize {
		lrec = store_minTableSize
	}

	fh, err := os.CreateTemp(s.dir, "hashtbl-*.tmp")
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() {
		if fh != nil {
			_ = fh.Close()
			_ = os.Remove(fh.Name())
		}
	}()

	if err := fh.Truncate(1 << lrec * rSize); err != nil {
		return errs.New("allocating new hashtbl lrec=%d size=%d: %w", lrec, 1<<lrec*rSize, err)
	}

	ntbl, err := newHashTbl(fh, lrec, false)
	if err != nil {
		return errs.Wrap(err)
	}

	// 4. copy all of the entries from the hash table to the new table, skipping expired entries,
	// and rewriting any entries that are in logs marked for compaction.

	// create a context that inherits from the existing context that is canceled when the store is
	// closed. this ensures that the shouldTrash callback exits when the store is closed.
	trashCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-trashCtx.Done():
		case <-s.closed.Signal():
			cancel()
		}
	}()

	// the current log file compacting into and how many bytes have been written into it.
	var into *logFile
	s.tbl.Range(func(rec record, err error) bool {
		rerr = func() error {
			if err != nil {
				return errs.Wrap(err)
			} else if err := ctx.Err(); err != nil {
				return err
			} else if err := s.closed.Err(); err != nil {
				return err
			}

			// if the record is restored, clear the expiration.
			if restored(rec.expires) {
				rec.expires = 0

				// we bump created so that the shouldTrash callback likely ignores it in case the
				// bloom filter was bad or something. this will probably change once the hashstore
				// is more integrated with the system and it has more details about the bloom
				// filter.
				rec.created = today
			}

			// totally ignore any expired records.
			if expired(rec.expires) {
				return nil
			}

			// trash records are flagged as expired some number of days from now with a bit set to
			// signal if they are read that there was a problem. we only check records that are not
			// already flagged as trashed and keep the minimum time for the record to live.
			if !rec.expires.trash() && shouldTrash != nil {
				trash, err := shouldTrash(trashCtx, rec.key, dateToTime(rec.created))
				if err != nil {
					return errs.Wrap(err)
				}
				if trash {
					expiresTime := today + compaction_ExpiresDays
					// if we have an existing ttl time and it's smaller, use that instead.
					if existingTime := rec.expires.time(); existingTime > 0 && existingTime < expiresTime {
						expiresTime = existingTime
					}
					rec.expires = newExpiration(expiresTime, true)
				}
			}

			if compact[rec.log] {
				r, err := s.readerForRecord(ctx, rec)
				if err != nil {
					return errs.Wrap(err)
				}
				defer r.Release() // same as r.Close() but no error to worry about.

				// if we don't have a log file to compact into or it is too big, make a new one.
				if into == nil || into.size >= compaction_MaxLogSize {
					lf, err := s.createLogFile()
					if err != nil {
						return errs.Wrap(err)
					}
					into = lf
				}

				// record the new offset and log id for the record.
				rec.offset = into.size
				rec.log = into.id

				n, err := io.Copy(into.fh, r)
				if err != nil {
					return errs.New("writing into compacted log: %w", err)
				}
				into.size += uint64(n)
			}

			if ok, err := ntbl.Insert(rec); err != nil {
				return errs.Wrap(err)
			} else if !ok {
				return errs.New("compaction hash table is full")
			}
			return nil
		}()
		return rerr == nil
	})
	if rerr != nil {
		return rerr
	}

	// 5. sync and rename the new hash table to the final name.
	if err := fh.Sync(); err != nil {
		return errs.New("unable to sync newly compacted hashtbl: %w", err)
	}
	if err := os.Rename(fh.Name(), filepath.Join(s.dir, "hashtbl")); err != nil {
		return errs.New("unable to rename newly compacted hashtbl: %w", err)
	}

	// now that it has been renamed, we should not try to remove it, and the rest of the function
	// should not have any error paths. this is because all new writes must go into the new file
	// because if the store reopens, it will be using the new file.
	fh = nil

	// try to sync the dir. we have to proceed even if this fails because we renamed successfully.
	if dir, err := os.Open(s.dir); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}

	// 6. swap the new hash table in and collect the set of log files to remove.
	s.rmu.Lock()

	otbl := s.tbl
	s.tbl = ntbl

	var toRemove []*logFile
	for id := range compact {
		if lf, ok := s.lfs.LoadAndDelete(id); ok {
			toRemove = append(toRemove, lf)
		}
	}

	s.rmu.Unlock()

	// 7. close and remove any newly dead log files now that we are no longer holding the mutex.
	otbl.Close()
	for _, lf := range toRemove {
		lf.Close()
		lf.Remove()
	}

	// 8. before we allow writers to proceed, reinitialize the heap with the log files.
	s.initializeHeap()

	return nil
}
