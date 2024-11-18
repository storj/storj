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
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"storj.io/common/context2"
	"storj.io/drpc/drpcsignal"
)

const (
	store_minTableSize = 17 // log_2 of number of records for smallest hash table

	compaction_MaxLogSize    = 10 << 30 // max size of a log file
	compaction_AliveFraction = 0.75     // if the log file is not this alive, compact it
	compaction_ExpiresDays   = 7        // number of days to keep trash records around
)

// Store is a hash table based key-value store with compaction.
type Store struct {
	// immutable data
	dir   string        // directory containing log files
	meta  string        // directory containing meta files (lock + hashtbl)
	log   *zap.Logger   // logger for unhandleable errors
	today func() uint32 // hook for getting the current timestamp
	lock  *os.File      // lock file to prevent multiple processes from using the same store

	closed drpcsignal.Signal // closed state
	cloMu  sync.Mutex        // synchronizes closing

	active      *rwMutex      // semaphore of active writes to log files
	compactMu   *mutex        // held during compaction to ensure only 1 compaction at a time
	reviveMu    *mutex        // held during revival to ensure only 1 object is revived from trash at a time
	compactions atomic.Uint64 // bumped every time a compaction call finishes

	maxLog atomic.Uint64              // maximum log file id
	stats  atomic.Pointer[StoreStats] // set during compaction to maintain consistency of Stats calls

	hmu sync.Mutex // enforces atomic access to the heap
	lfh logHeap    // heap of log files sorted by size ready to be written into

	rmu sync.RWMutex                // protects consistency of lfs and tbl
	lfs atomicMap[uint64, *logFile] // all log files
	tbl *HashTbl                    // hash table of records
}

// NewStore creates or opens a store in the given directory.
func NewStore(dir string, log *zap.Logger) (_ *Store, err error) {
	if log == nil {
		log = zap.NewNop()
	}

	s := &Store{
		dir:   dir,
		meta:  filepath.Join(dir, "meta"),
		log:   log,
		today: func() uint32 { return timeToDateDown(time.Now()) },

		active:    newRWMutex(),
		compactMu: newMutex(),
		reviveMu:  newMutex(),
	}

	// if we have any errors, close the store. this means that Close must be
	// prepared to operate on a partially initialized store.
	defer func() {
		if err != nil {
			s.Close()
		}
	}()

	// attempt to make the meta directory which ensures all parent directories exist.
	if err := os.MkdirAll(s.meta, 0755); err != nil {
		return nil, Error.New("unable to create directory=%q: %w", dir, err)
	}

	// acquire the lock file to prevent concurrent use of the hash table.
	s.lock, err = os.OpenFile(filepath.Join(s.meta, "lock"), os.O_CREATE|os.O_RDONLY, 0444)
	if err != nil {
		return nil, Error.New("unable to acquire lock: %w", err)
	}
	if err := flock(s.lock); err != nil {
		return nil, Error.New("unable to flock: %w", err)
	}

	// read all of the files in the directories.
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, Error.New("unable to read log directory=%q: %w", dir, err)
	}

	// load all of the log files and keep track of if there is a hashtbl file.
	for _, entry := range entries {
		name := entry.Name()

		// skip any files that don't look like log files.
		if len(name) != 3+1+16 || !strings.HasPrefix(name, "log-") {
			continue
		}

		id, err := strconv.ParseUint(name[4:20], 16, 64)
		if err != nil {
			return nil, Error.New("unable to parse name=%q: %w", name, err)
		}
		fh, err := os.OpenFile(filepath.Join(s.dir, name), os.O_RDWR, 0)
		if err != nil {
			return nil, Error.New("unable to open log file: %w", err)
		}
		size, err := fh.Seek(0, io.SeekEnd)
		if err != nil {
			_ = fh.Close()
			return nil, Error.New("unable to seek name=%q: %w", name, err)
		}

		if maxLog := s.maxLog.Load(); id > maxLog {
			s.maxLog.Store(id)
		}
		s.lfs.Set(id, newLogFile(fh, id, uint64(size)))
	}

	// with the set of log files created, initialize the heap.
	s.initializeHeap()

	// try to open the hash table.
	hashtblPath := filepath.Join(s.meta, "hashtbl")
	fh, err := os.OpenFile(hashtblPath, os.O_RDWR, 0)
	if os.IsNotExist(err) {
		err = func() error {
			af, err := newAtomicFile(s.meta, "hashtbl")
			if err != nil {
				return Error.Wrap(err)
			}
			defer af.Cancel()

			ntbl, err := CreateHashtbl(af.File, store_minTableSize, s.today())
			if err != nil {
				return Error.Wrap(err)
			}
			defer ntbl.Close()

			if err := af.Commit(); err != nil {
				return Error.Wrap(err)
			}
			return nil
		}()
		if err != nil {
			return nil, err
		}

		// retry opening the hash table now that it should be created.
		fh, err = os.OpenFile(hashtblPath, os.O_RDWR, 0)
	}
	if err != nil {
		return nil, Error.Wrap(err)
	}

	// set up the hash table to use the handle.
	s.tbl, err = OpenHashtbl(fh)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	// best effort try to clean up temp files now that everything is open and flocked.
	entries, _ = os.ReadDir(s.meta)
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			_ = os.Remove(filepath.Join(s.meta, entry.Name()))
		}
	}

	return s, nil
}

// StoreStats is a collection of statistics about a store.
type StoreStats struct {
	NumLogs uint64 // total number of log files.
	LenLogs uint64 // total number of bytes in the log files.

	SetPercent   float64 // percent of bytes that are set in the log files.
	TrashPercent float64 // percent of bytes that are trash in the log files.

	Compacting  bool         // if true, a compaction is in progress.
	Compactions uint64       // number of compaction calls that finished
	Today       uint32       // the current date.
	Table       HashTblStats // stats about the hash table.
}

// Stats returns a StoreStats about the store.
func (s *Store) Stats() StoreStats {
	if stats := s.stats.Load(); stats != nil {
		return *stats
	}

	s.rmu.RLock()
	stats := s.tbl.Stats()

	var numLogs, lenLogs uint64
	s.lfs.Range(func(_ uint64, lf *logFile) bool {
		numLogs++
		lenLogs += lf.size
		return true
	})
	s.rmu.RUnlock()

	// account for record footers in log files not included in the length field
	// in the record.
	stats.LenSet += RecordSize * stats.NumSet
	stats.AvgSet += RecordSize
	stats.LenTrash += RecordSize * stats.NumTrash
	stats.AvgTrash += RecordSize

	return StoreStats{
		NumLogs: numLogs,
		LenLogs: lenLogs,

		SetPercent:   safeDivide(float64(stats.LenSet), float64(lenLogs)),
		TrashPercent: safeDivide(float64(stats.LenTrash), float64(lenLogs)),

		Compacting:  false,
		Compactions: s.compactions.Load(),
		Today:       s.today(),
		Table:       stats,
	}
}

func (s *Store) createLogFile() (*logFile, error) {
	id := s.maxLog.Add(1)
	path := filepath.Join(s.dir, fmt.Sprintf("log-%016x", id))
	fh, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	lf := newLogFile(fh, id, 0)
	s.lfs.Set(id, lf)
	return lf, nil
}

func (s *Store) initializeHeap() {
	s.hmu.Lock()
	defer s.hmu.Unlock()

	// collect all of the writable log files into the heap.
	s.lfh = s.lfh[:0]
	s.lfs.Range(func(_ uint64, lf *logFile) bool {
		if lf.size < compaction_MaxLogSize {
			s.lfh.Push(lf)
		}
		return true
	})
	heap.Init(&s.lfh)
}

func (s *Store) replaceLogFile(lf *logFile) {
	// sometimes a Writer is created with a nil store so that it can turn off automatically
	// replacing log files.
	if s == nil {
		return
	}

	// acquire the heap mutex to push the log back into the heap of active log files.
	s.hmu.Lock()
	defer s.hmu.Unlock()

	// only put the file back into the heap if it's not too big.
	if lf.size < compaction_MaxLogSize {
		heap.Push(&s.lfh, lf)
	}
}

func (s *Store) tryPopLogFile() *logFile {
	s.hmu.Lock()
	defer s.hmu.Unlock()

	if s.lfh.Len() == 0 {
		return nil
	}
	return heap.Pop(&s.lfh).(*logFile)
}

func (s *Store) acquireLogFile() (*logFile, error) {
	if lf := s.tryPopLogFile(); lf != nil {
		return lf, nil
	}
	return s.createLogFile()
}

func (s *Store) addRecord(ctx context.Context, rec Record) error {
	// sometimes a Writer is created with a nil store so that it can disable automatically writing
	// records.
	if s == nil {
		return nil
	}

	ok, err := s.tbl.Insert(ctx, rec)
	if err != nil {
		return Error.Wrap(err)
	} else if !ok {
		return Error.New("hash table is full")
	} else {
		return nil
	}
}

// Load returns the estimated load factor of the hash table. If it's too large, a Compact call is
// indicated.
func (s *Store) Load() float64 {
	s.rmu.RLock()
	defer s.rmu.RUnlock()

	return s.tbl.Load()
}

// Close interrupts any compactions and closes the store.
func (s *Store) Close() {
	s.cloMu.Lock()
	defer s.cloMu.Unlock()

	if !s.closed.Set(Error.New("store closed")) {
		return
	}

	// acquire the compaction lock to ensure no compactions are in progress. setting s.closed should
	// ensure that any ongoing compaction exits promptly.
	s.compactMu.WaitLock()
	defer s.compactMu.Unlock()

	// acquire the write mutex to ensure all writes are finished.
	s.active.WaitLock()
	defer s.active.Unlock()

	// we can now close all of the resources.
	s.lfs.Range(func(id uint64, lf *logFile) bool {
		s.lfs.Delete(id)
		lf.Close()
		return true
	})
	s.lfh = nil

	if s.tbl != nil {
		s.tbl.Close()
	}

	if s.lock != nil {
		_ = s.lock.Close()
	}
}

// Create returns a Handle that writes data to the store. The error on Close must be checked.
// Expires is when the data expires, or zero if it never expires.
func (s *Store) Create(ctx context.Context, key Key, expires time.Time) (_ *Writer, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := s.active.RLock(ctx, &s.closed); err != nil {
		return nil, err
	}

	// unlock if we return an error.
	defer func() {
		if err != nil {
			s.active.RUnlock()
		}
	}()

	// try to acquire the log file.
	lf, err := s.acquireLogFile()
	if err != nil {
		return nil, Error.Wrap(err)
	}

	// compute an expiration field if one is set.
	var exp Expiration
	if !expires.IsZero() {
		exp = NewExpiration(timeToDateUp(expires), false)
	}

	// return the automatic writer for the piece that unlocks and commits the record into the hash
	// table on Close.
	return newAutomaticWriter(ctx, s, lf, Record{
		Key:     key,
		Offset:  lf.size,
		Log:     lf.id,
		Created: s.today(),
		Expires: exp,
	}), nil
}

// Read returns a Reader that reads data from the store. The Reader will be nil if the key does not
// exist.
func (s *Store) Read(ctx context.Context, key Key) (_ *Reader, err error) {
	defer mon.Task()(&ctx)(&err)

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

	if rec, ok, err := s.tbl.Lookup(ctx, key); err != nil {
		return nil, Error.Wrap(err)
	} else if !ok {
		return nil, nil
	} else {
		return s.readerForRecord(ctx, rec, true)
	}
}

func (s *Store) readerForRecord(ctx context.Context, rec Record, revive bool) (_ *Reader, err error) {
	defer mon.Task()(&ctx)(&err)

	lf, ok := s.lfs.Lookup(rec.Log)
	if !ok {
		return nil, Error.New("record points to unknown log file rec=%v", rec)
	}
	if !lf.Acquire() {
		return nil, Error.New("unable to acquire log file for reading rec=%v", rec)
	}

	if revive && rec.Expires.Trash() {
		s.log.Warn("trashed record was read",
			zap.String("record", rec.String()),
		)

		if err := s.reviveRecord(ctx, lf, rec); err != nil {
			s.log.Error("unable to revive record",
				zap.String("record", rec.String()),
				zap.Error(err),
			)
		}
	}

	return newLogReader(lf, rec), nil
}

func (s *Store) reviveRecord(ctx context.Context, lf *logFile, rec Record) (err error) {
	defer mon.Task()(&ctx)(&err)

	// we don't want to respect cancelling if the reader for the trashed piece goes away. we know it
	// was trashed so we should revive it no matter what.
	ctx = context2.WithoutCancellation(ctx)

	if err := s.reviveMu.Lock(ctx, &s.closed); err != nil {
		return err
	}
	defer s.reviveMu.Unlock()

	// easy case: if we can acquire the compaction mutex, then we can be sure that no compaction is
	// ongoing so we can just write the record with a cleared expires field. we're already holding
	// the rmu so the tbl will not be mutated and the record points at a valid log file.
	if s.compactMu.TryLock() {
		defer s.compactMu.Unlock()

		rec.Expires = 0
		return s.addRecord(ctx, rec)
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

	// 1. acquire a write slot, ensuring that no compaction is ongoing and we can write to a log if
	// necessary. once we have a writer, we know the state of the hash table and logs can only be
	// added to.
	w, err := s.Create(ctx, rec.Key, time.Time{})
	if err != nil {
		return Error.Wrap(err)
	}
	defer w.Cancel()

	// 2. find the current state of the record. if found, we can just update the expiration and be
	// happy. as noted in 1, we're safe to do a lookup into s.tbl here even without the rmu held
	// because we know no compaction is ongoing due to having a writer acquired, and compaction is
	// the only thing that does anything other than than add entries to the hash table.
	if tmp, ok, err := s.tbl.Lookup(ctx, rec.Key); err == nil && ok {
		if tmp.Expires == 0 {
			return nil
		}

		tmp.Expires = 0
		return s.addRecord(ctx, tmp)
	}

	// 3. otherwise, we either had an error looking up the current record, or the entry got fully
	// deleted, and the open file handle is the last remaining evidence that it exists, so we have
	// to rewrite it. note that we purposefully do not close the log reader because after this
	// function exits, a log reader will be created and returned to the user using the same log
	// file.
	_, err = io.Copy(w, newLogReader(lf, rec))
	if err != nil {
		return Error.Wrap(err)
	}
	if err := w.Close(); err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// Compact removes keys and files that are definitely expired, and marks keys that are determined
// trash by the callback to expire in the future. It also rewrites any log files that have too much
// dead data.
func (s *Store) Compact(
	ctx context.Context,
	shouldTrash func(ctx context.Context, key Key, created time.Time) bool,
	lastRestore time.Time,
) (err error) {
	defer mon.Task()(&ctx)(&err)
	defer s.compactions.Add(1) // increase the number of compactions that have finished

	// create a context that inherits from the existing context that is canceled when the store is
	// closed. this ensures that the shouldTrash callback exits when the store is closed, and allows
	// us to only need to poll ctx.Err in any loops below.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-ctx.Done():
		case <-s.closed.Signal():
			cancel()
		}
	}()

	// ensure only one compaction at a time.
	if err := s.compactMu.Lock(ctx, &s.closed); err != nil {
		return err
	}
	defer s.compactMu.Unlock()

	// stop all writers from starting and wait for all current writers to finish.
	if err := s.active.Lock(ctx, &s.closed); err != nil {
		return err
	}
	defer s.active.Unlock()

	// cache stats so that the call doesn't get inconsistent internal values and clear them out when
	// we're finished.
	stats := s.Stats()
	stats.Compacting = true
	s.stats.Store(&stats)
	defer s.stats.Store(nil)

	// define some functions to tell what state records are in based on what today is and what the
	// last restore time is.
	today := s.today()

	var restore uint32
	if !lastRestore.IsZero() {
		restore = timeToDateUp(lastRestore)
	}

	restored := func(e Expiration) bool {
		// if the expiration is trash and it is before the restore time, it is restored.
		return e.Trash() && e.Time() <= restore+compaction_ExpiresDays
	}

	expired := func(e Expiration) bool {
		// if the record does not have an expiration, it is not expired.
		if e == 0 {
			return false
		}
		// if it is not currently after the expiration time, it is not expired.
		if today <= e.Time() {
			return false
		}
		// if it has been restored, it is not expired.
		if restored(e) {
			return false
		}
		// otherwise, it is expired.
		return true
	}

	// collect statistics about the hash table and how live each of the log files are.
	nset := uint64(0)
	used := make(map[uint64]uint64)
	rerr := error(nil)
	s.tbl.Range(ctx, func(rec Record, err error) bool {
		rerr = func() error {
			if err != nil {
				return Error.Wrap(err)
			} else if err := ctx.Err(); err != nil {
				return err
			}

			if expired(rec.Expires) {
				return nil
			}

			nset++
			used[rec.Log] += uint64(rec.Length) + RecordSize // rSize for the record footer
			return nil
		}()
		return rerr == nil
	})
	if rerr != nil {
		return rerr
	}

	// using the information, determine which logs need compaction.
	compact := make(map[uint64]bool)
	s.lfs.Range(func(id uint64, lf *logFile) bool {
		rerr = func() error {
			if err := ctx.Err(); err != nil {
				return err
			}

			// compact non-empty log files that don't contain enough alive data.
			if lf.size > 0 && float64(used[id])/float64(lf.size) < compaction_AliveFraction {
				compact[id] = true
			}

			return nil
		}()
		return rerr == nil
	})
	if rerr != nil {
		return rerr
	}

	// calculate a hash table size so that it targets just under a 0.25 load factor.
	lrec := uint64(bits.Len64(nset)) + 1
	if lrec < store_minTableSize {
		lrec = store_minTableSize
	}

	// create a new hash table sized for the number of records.
	af, err := newAtomicFile(s.meta, "hashtbl")
	if err != nil {
		return Error.Wrap(err)
	}
	defer af.Cancel()

	ntbl, err := CreateHashtbl(af.File, lrec, today)
	if err != nil {
		return Error.Wrap(err)
	}

	// copy all of the entries from the hash table to the new table, skipping expired entries, and
	// rewriting any entries that are in logs marked for compaction.
	s.tbl.Range(ctx, func(rec Record, err error) bool {
		rerr = func() error {
			if err != nil {
				return Error.Wrap(err)
			} else if err := ctx.Err(); err != nil {
				return err
			}

			// if the record is restored, clear the expiration.
			if restored(rec.Expires) {
				rec.Expires = 0

				// we bump created so that the shouldTrash callback likely ignores it in case the
				// bloom filter was bad or something. this may change once the hashstore is more
				// integrated with the system and it has more details about the bloom filter.
				rec.Created = today
			}

			// totally ignore any expired records.
			if expired(rec.Expires) {
				return nil
			}

			// trash records are flagged as expired some number of days from now with a bit set to
			// signal if they are read that there was a problem. we only check records that are not
			// already flagged as trashed and keep the minimum time for the record to live. we do
			// this after compaction so that we don't mistakenly count it as a "revive".
			if !rec.Expires.Trash() && shouldTrash != nil {
				if shouldTrash(ctx, rec.Key, dateToTime(rec.Created)) {
					expiresTime := today + compaction_ExpiresDays
					// if we have an existing ttl time and it's smaller, use that instead.
					if existingTime := rec.Expires.Time(); existingTime > 0 && existingTime < expiresTime {
						expiresTime = existingTime
					}
					rec.Expires = NewExpiration(expiresTime, true)
				}
			}

			// if the record is compacted, copy it into the new log file.
			if compact[rec.Log] {
				err := func() error {
					r, err := s.readerForRecord(ctx, rec, false)
					if err != nil {
						return Error.Wrap(err)
					}
					defer r.Release() // same as r.Close() but no error to worry about.

					// acquire a log file to write the entry into. if we're compacting that log file
					// we have to pick a different one. doing this can only increase the amount of
					// live data in a log file, so it can't end up wanting to be compacted.
				acquire:
					into, err := s.acquireLogFile()
					if err != nil {
						return Error.Wrap(err)
					} else if compact[into.id] {
						goto acquire
					}

					// create a Writer to handle writing the entry into the log file. manual mode is
					// set so that it doesn't attempt to add the record to the current hash table or
					// unlock the active mutex upon Close or Cancel.
					w := newManualWriter(ctx, s, into, Record{
						Key:     rec.Key,
						Offset:  into.size,
						Log:     into.id,
						Created: rec.Created,
						Expires: rec.Expires,
					})
					defer w.Cancel()

					// copy the record data.
					if _, err := io.Copy(w, r); err != nil {
						return Error.New("writing into compacted log: %w", err)
					}

					// finalize the data in the log file.
					if err := w.Close(); err != nil {
						return Error.New("closing compacted log: %w", err)
					}

					// get the updated record information from the writer.
					rec = w.rec
					return nil
				}()
				if err != nil {
					return Error.Wrap(err)
				}
			}

			// insert the record into the new hash table.
			if ok, err := ntbl.Insert(ctx, rec); err != nil {
				return Error.Wrap(err)
			} else if !ok {
				return Error.New("compaction hash table is full")
			}

			return nil
		}()
		return rerr == nil
	})
	if rerr != nil {
		return rerr
	}

	// commit the new hash table. there should be no error cases in this function after this point
	// because a process restart may have the store open with this new hash table, so we have to
	// go forward with it.
	if err := af.Commit(); err != nil {
		return Error.New("unable to commit newly compacted hashtbl: %w", err)
	}

	// swap the new hash table in and collect the set of log files to remove. we don't close and
	// remove the log files while holding the lock to avoid doing i/o while blocking readers.
	s.rmu.Lock()
	otbl := s.tbl
	s.tbl = ntbl

	toRemove := make([]*logFile, 0, len(compact))
	for id := range compact {
		if lf, ok := s.lfs.LoadAndDelete(id); ok {
			toRemove = append(toRemove, lf)
		}
	}
	s.rmu.Unlock()

	// now that we are no longer holding the mutex, close the old hashtbl and close and remove any
	// newly dead log files. log files have protection to not actually close the underlying file
	// handle until the last reader is finished.
	otbl.Close()
	for _, lf := range toRemove {
		lf.Close()
		lf.Remove()
	}

	// best effort sync the directories now that we are done with mutations.
	syncDirectory(s.meta)
	syncDirectory(s.dir)

	// before we allow writers to proceed, reinitialize the heap with the log files so that it has
	// the best set of logs to write into and doesn't contain any now closed/removed logs.
	s.initializeHeap()

	return nil
}
