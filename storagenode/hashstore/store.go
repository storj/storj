// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"context"
	"fmt"
	"io"
	"math"
	"math/bits"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zeebo/mwc"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/exp/maps"

	"storj.io/common/context2"
	"storj.io/common/memory"
	"storj.io/drpc/drpcsignal"
	"storj.io/storj/storagenode/hashstore/platform"
)

var (
	// max size of a log file.
	compaction_MaxLogSize = uint64(envInt("STORJ_HASHSTORE_COMPACTION_MAX_LOG_SIZE", 1<<30))

	// number of days to keep trash records around.
	compaction_ExpiresDays = uint32(envInt("STORJ_HASHSTORE_COMPACTION_EXPIRES_DAYS", 7))

	// if the log file is not this alive, compact it.
	compaction_AliveFraction     = envFloat("STORJ_HASHSTORE_COMPACTION_ALIVE_FRAC", 0.25)
	compaction_ProbabilityFactor = compaction_AliveFraction / (1 - compaction_AliveFraction)

	// power to rase the rewrite probability to. >1 means must be closer to the alive fraction
	// to be compacted, <1 means the opposite.
	compaction_ProbabilityPower = envFloat("STORJ_HASHSTORE_COMPACTION_PROBABILITY_POWER", 2.0)

	// multiple of the hashtbl to rewrite in a single compaction.
	compaction_RewriteMultiple = envFloat("STORJ_HASHSTORE_COMPACTION_REWRITE_MULTIPLE", 10)

	// if set, deletes all trash immediately instead of after the ttl.
	compaction_DeleteTrashImmediately = envBool("STORJ_HASHSTORE_COMPACTION_DELETE_TRASH_IMMEDIATELY", false)

	// controls the number of concurrent flushes to log files.
	store_FlushSemaphore = int(envInt("STORJ_HASHSTORE_STORE_FLUSH_SEMAPHORE", 0))

	// controls if we collect records and sort them and rewrite them before the hashtbl.
	compaction_OrderedRewrite = envBool("STORJ_HASHSTORE_COMPACTION_ORDERED_REWRITE", false)

	// if set, writes to the log file and table are fsync'd to disk.
	store_SyncWrites = envBool("STORJ_HASHSTORE_STORE_SYNC_WRITES", false)

	// if set to true, the store pretends to not find values in the rewritten index during compaction.
	store_TestIgnoreRewrittenIndex = false

	// if set to true, the store does extra checks to ensure log file sizes match their seek offset.
	store_TestLogSizeAndOffset = false
)

// Store is a hash table based key-value store with compaction.
type Store struct {
	// immutable data
	logsPath  string         // directory containing log files
	tablePath string         // directory containing meta files (lock + hashtbl)
	log       *zap.Logger    // logger for unhandleable errors
	today     func() uint32  // hook for getting the current timestamp
	lock      *os.File       // lock file to prevent multiple processes from using the same store
	lfc       *logCollection // collection of log files ready to be written into

	closed drpcsignal.Signal // closed state
	cloMu  sync.Mutex        // synchronizes closing

	flushMu   *rwMutex // semaphore of active flushes to log files
	compactMu *mutex   // held during compaction to ensure only 1 compaction at a time
	reviveMu  *mutex   // held during revival to ensure only 1 object is revived from trash at a time

	maxLog  atomic.Uint64 // maximum log file id
	maxHash atomic.Uint64 // maximum hashtbl id

	stats struct { // contains statistics for monitoring the store
		compactions atomic.Uint64 // bumped every time a compaction call finishes
		lastCompact atomic.Uint32 // date of the last compaction

		logsRewritten atomic.Uint64 // bumped when a log file is marked to be rewritten
		dataRewritten atomic.Uint64 // bumped whenever a record is rewritten with the length of the record
		dataReclaimed atomic.Uint64 // bumped whenever a log is unlinked with the length of the log

		cached           atomic.Pointer[StoreStats] // set during compaction to maintain consistency of Stats calls
		startTime        atomic.Value               // time of the start of the current compaction
		writeTime        atomic.Value               // time of the start of writing the new hash table
		totalRecords     atomic.Uint64              // total number of records to be processed in current compaction
		processedRecords atomic.Uint64              // total number of records processed in current compaction
	}

	rmu sync.RWMutex                // protects consistency of lfs and tbl
	lfs atomicMap[uint64, *logFile] // all log files
	tbl Tbl                         // hash table of records
}

// NewStore creates or opens a store in the given directory.
func NewStore(ctx context.Context, logsPath string, tablePath string, log *zap.Logger) (_ *Store, err error) {
	defer mon.Task()(&ctx)(&err)

	if log == nil {
		log = zap.NewNop()
	}

	if tablePath == "" {
		tablePath = filepath.Join(logsPath, "meta")
	}

	s := &Store{
		logsPath:  logsPath,
		tablePath: tablePath,
		log:       log,
		today:     func() uint32 { return TimeToDateDown(time.Now()) },
		lfc:       newLogCollection(),

		flushMu:   newRWMutex(store_FlushSemaphore),
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
	if err := os.MkdirAll(s.tablePath, 0755); err != nil {
		return nil, Error.New("unable to create directory=%q: %w", s.tablePath, err)
	}

	if err := os.MkdirAll(s.logsPath, 0755); err != nil {
		return nil, Error.New("unable to create directory=%q: %w", s.logsPath, err)
	}

	{ // acquire the lock file to prevent concurrent use of the hash table.
		s.lock, err = os.OpenFile(filepath.Join(s.tablePath, "lock"), os.O_CREATE|os.O_RDONLY, 0666)
		if err != nil {
			return nil, Error.New("unable to create lock file: %w", err)
		}
		if err := platform.Flock(s.lock); err != nil {
			return nil, Error.New("unable to flock: %w", err)
		}
	}

	{ // open all of the log files
		paths, err := allFiles(s.logsPath)
		if err != nil {
			return nil, err
		}

		// load all of the log files and keep track of if there is a hashtbl file.
		for _, path := range paths {
			name := filepath.Base(path)

			// skip any files that don't look like log files. log file names are either
			//     log-<16 bytes of id>
			//     log-<16 bytes of id>-<8 bytes of ttl>
			// so they always begin with "log-" and are either 20 or 29 bytes long.
			if (len(name) != 20 && len(name) != 29) || name[0:4] != "log-" {
				continue
			}

			id, err := strconv.ParseUint(name[4:20], 16, 64)
			if err != nil {
				return nil, Error.New("unable to parse name=%q: %w", name, err)
			}

			var ttl uint32
			if len(name) == 29 && name[20] == '-' {
				ttl64, err := strconv.ParseUint(name[21:29], 16, 32)
				if err != nil {
					return nil, Error.New("unable to parse name=%q: %w", name, err)
				}
				ttl = uint32(ttl64)
			}

			fh, err := os.OpenFile(path, os.O_RDWR, 0)
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

			lf := newLogFile(fh, id, ttl, uint64(size))
			s.lfs.Set(id, lf)
			s.lfc.Include(lf)
		}
	}

	{ // open or create the hash table
		entries, err := os.ReadDir(s.tablePath)
		if err != nil {
			return nil, Error.New("unable to read meta directory=%q: %w", s.tablePath, err)
		}

		maxName := "hashtbl" // backwards compatible with old hashtbl files
		for _, entry := range entries {
			name := entry.Name()

			// skip any files that don't look like hashtbl files. hashtbl file names are always
			//     hashtbl-<16 bytes of id>
			// so they always begin with "hashtbl-" and are 24
			if len(name) != 24 || name[0:8] != "hashtbl-" {
				continue
			}

			id, err := strconv.ParseUint(name[8:24], 16, 64)
			if err != nil {
				return nil, Error.New("unable to parse name=%q: %w", name, err)
			}

			if maxHash := s.maxHash.Load(); id > maxHash {
				s.maxHash.Store(id)
				maxName = name
			}
		}
		maxPath := filepath.Join(s.tablePath, maxName)

		// try to open the hashtbl file and create it if it doesn't exist.
		fh, err := os.OpenFile(maxPath, os.O_RDWR, 0)
		if os.IsNotExist(err) {
			// file did not exist, so try to create it with an initial hashtbl.
			err = func() error {
				af, err := newAtomicFile(maxPath)
				if err != nil {
					return Error.New("unable to create hashtbl: %w", err)
				}
				defer af.Cancel()

				cons, err := CreateTable(ctx, af.File, tbl_minLogSlots, s.today(), table_DefaultKind)
				if err != nil {
					return Error.Wrap(err)
				}
				tbl, err := cons.Done(ctx)
				if err != nil {
					return Error.Wrap(err)
				}
				defer tbl.Close()

				return af.Commit()
			}()
			if err != nil {
				return nil, err
			}

			// now try to reopen the file handle after it should be created.
			fh, err = os.OpenFile(maxPath, os.O_RDWR, 0)
		}
		if err != nil {
			return nil, Error.Wrap(err)
		}

		ntbl, err := OpenTable(ctx, fh)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		s.tbl = ntbl

		// best effort clean up any tmp files or previous hashtbls that were left behind from a
		// previous execution.
		for _, entry := range entries {
			if name := entry.Name(); strings.HasPrefix(name, "hashtbl") && name != maxName {
				_ = os.Remove(filepath.Join(s.tablePath, name))
			}
		}
	}

	return s, nil
}

// StoreStats is a collection of statistics about a store.
type StoreStats struct {
	NumLogs    uint64      // total number of log files.
	LenLogs    memory.Size // total number of bytes in the log files.
	NumLogsTTL uint64      // total number of log files with ttl set.
	LenLogsTTL memory.Size // total number of bytes in log files with ttl set.

	SetPercent   float64 // percent of bytes that are set in the log files.
	TrashPercent float64 // percent of bytes that are trash in the log files.
	TTLPercent   float64 // percent of bytes that have expiration but not trash in the log files.

	Compacting      bool        // if true, a compaction is in progress.
	Compactions     uint64      // number of compaction calls that finished
	Today           uint32      // the current date.
	LastCompact     uint32      // the date of the last compaction.
	LogsRewritten   uint64      // number of log files attempted to be rewritten.
	DataRewritten   memory.Size // number of bytes rewritten in the log files.
	DataReclaimed   memory.Size // number of bytes reclaimed in the log files.
	DataReclaimable memory.Size // number of bytes potentially reclaimable in the log files.
	Table           TblStats    // stats about the hash table.

	Compaction struct { // stats about the current compaction
		Elapsed          float64 // number of seconds elapsed in the compaction
		Remaining        float64 // estimated number of seconds remaining in the compaction
		TotalRecords     uint64  // total number of records expected to be processed in the compaction
		ProcessedRecords uint64  // total number of records processed in the compaction
	}
}

// Stats returns a StoreStats about the store.
func (s *Store) Stats() StoreStats {
	if statsPtr := s.stats.cached.Load(); statsPtr != nil {
		stats := *statsPtr

		start := s.stats.startTime.Load().(time.Time)
		write := s.stats.writeTime.Load().(time.Time)
		total := s.stats.totalRecords.Load()
		processed := s.stats.processedRecords.Load()

		elapsed := time.Since(start).Seconds()
		remaining := time.Since(write).Seconds() * safeDivide(float64(total-processed), float64(processed))

		stats.Compacting = true
		stats.Compaction.Elapsed = elapsed
		stats.Compaction.Remaining = remaining
		stats.Compaction.TotalRecords = total
		stats.Compaction.ProcessedRecords = processed

		stats.LogsRewritten = s.stats.logsRewritten.Load()
		stats.DataRewritten = memory.Size(s.stats.dataRewritten.Load())
		stats.DataReclaimed = memory.Size(s.stats.dataReclaimed.Load())

		return stats
	}

	s.rmu.RLock()
	stats := s.tbl.Stats()

	var numLogs, lenLogs uint64
	var numLogsTTL, lenLogsTTL uint64
	_ = s.lfs.Range(func(_ uint64, lf *logFile) (bool, error) {
		size := lf.size.Load()
		numLogs++
		lenLogs += size
		if lf.ttl > 0 {
			numLogsTTL++
			lenLogsTTL += size
		}
		return true, nil
	})
	s.rmu.RUnlock()

	// account for record footers in log files not included in the length field in the record.
	stats.LenSet += memory.Size(RecordSize * stats.NumSet)
	stats.AvgSet = safeDivide(float64(stats.LenSet), float64(stats.NumSet))
	stats.LenTrash += memory.Size(RecordSize * stats.NumTrash)
	stats.AvgTrash = safeDivide(float64(stats.LenTrash), float64(stats.NumTrash))

	return StoreStats{
		NumLogs:    numLogs,
		LenLogs:    memory.Size(lenLogs),
		NumLogsTTL: numLogsTTL,
		LenLogsTTL: memory.Size(lenLogsTTL),

		SetPercent:   safeDivide(float64(stats.LenSet), float64(lenLogs)),
		TrashPercent: safeDivide(float64(stats.LenTrash), float64(lenLogs)),
		TTLPercent:   safeDivide(float64(stats.LenTTL), float64(lenLogs)),

		Compacting:      false,
		Compactions:     s.stats.compactions.Load(),
		Today:           s.today(),
		LastCompact:     s.stats.lastCompact.Load(),
		LogsRewritten:   s.stats.logsRewritten.Load(),
		DataRewritten:   memory.Size(s.stats.dataRewritten.Load()),
		DataReclaimed:   memory.Size(s.stats.dataReclaimed.Load()),
		DataReclaimable: memory.Size(lenLogs) - stats.LenSet,
		Table:           stats,
	}
}

func (s *Store) createLogFile(ttl uint32) (*logFile, error) {
	id := s.maxLog.Add(1)
	dir := filepath.Join(s.logsPath, fmt.Sprintf("%02x", byte(id)))
	path := filepath.Join(dir, fmt.Sprintf("log-%016x-%08x", id, ttl))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, Error.Wrap(err)
	}
	fh, err := platform.CreateFile(path)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	lf := newLogFile(fh, id, ttl, 0)
	s.lfs.Set(id, lf)
	return lf, nil
}

func (s *Store) acquireLogFile(ttl uint32) (*logFile, error) {
	// if the ttl is too far in the future, just ignore it for the hint so that we can't create an
	// unbounded amount of log files. besides, something with no ttl is approximately something with
	// a huge ttl, so the clumping isn't as useful very far out.
	if ttl-s.today() > 100 {
		ttl = 0
	}

	if lf := s.lfc.Acquire(ttl); lf != nil {
		return lf, nil
	}

	// if we couldn't acquire a log file, try to create one. if it fails, we can try again but with
	// a zero ttl because maybe the problem is too many file handles or something but we may already
	// have a log file ready for pieces with no ttl.
	lf, err := s.createLogFile(ttl)
	if err != nil && ttl != 0 {
		return s.acquireLogFile(0)
	}
	return lf, err
}

func (s *Store) addRecord(ctx context.Context, rec Record) error {
	ok, err := s.tbl.Insert(ctx, rec)
	if err != nil {
		return Error.Wrap(err)
	} else if !ok {
		return Error.New("hashtbl full")
	}
	return nil
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

	// acquire the flush mutex to ensure no writes are committing.
	s.flushMu.WaitLock()
	defer s.flushMu.Unlock()

	// we can now close all of the resources.
	_ = s.lfs.Range(func(id uint64, lf *logFile) (bool, error) {
		s.lfs.Delete(id)
		lf.Close()
		return true, nil
	})
	s.lfc.Clear()

	if s.tbl != nil {
		s.tbl.Close()
	}

	if s.lock != nil {
		_ = s.lock.Close()
	}
}

// Create returns a Handle that writes data to the store. The error on Close must be checked.
// Expires is when the data expires, or zero if it never expires.
func (s *Store) Create(ctx context.Context, key Key, expires time.Time) (w *Writer, err error) {
	defer mon.Task()(&ctx)(&err)

	// if we're already closed, return an error.
	if err, ok := s.closed.Get(); ok {
		return nil, err
	}

	// compute an expiration field if one is set.
	var exp Expiration
	if !expires.IsZero() {
		exp = NewExpiration(TimeToDateUp(expires), false)
	}

	// return the writer for the piece that commits the record into the hash table on Close.
	return newWriter(ctx, s, key, exp), nil
}

// Read returns a Reader that reads data from the store. The Reader will be nil if the key does not
// exist.
func (s *Store) Read(ctx context.Context, key Key) (_ *Reader, err error) {
	defer mon.Task()(&ctx)(&err)

	// check if we're already closed so we don't have to worry about select nondeterminism: a
	// closed store will definitely error.
	if err := signalError(&s.closed); err != nil {
		return nil, err
	} else if err := ctx.Err(); err != nil {
		return nil, err
	}

	// ensure that tbl and lfs are consistent.
	s.rmu.RLock()
	defer s.rmu.RUnlock()

	if rec, ok, err := s.tbl.Lookup(ctx, key); err != nil {
		return nil, Error.Wrap(err)
	} else if !ok {
		return nil, nil
	} else {
		return s.readerForRecord(ctx, rec)
	}
}

func (s *Store) readerForRecord(ctx context.Context, rec Record) (_ *Reader, err error) {
	defer mon.Task()(&ctx)(&err)

	lf, ok := s.lfs.Lookup(rec.Log)
	if !ok {
		return nil, Error.New("record points to unknown log file rec=%v", rec)
	} else if !lf.Acquire() {
		return nil, Error.New("unable to acquire log file for reading rec=%v", rec)
	}

	return newLogReader(s, lf, rec), nil
}

func (s *Store) reviveRecord(ctx context.Context, lf *logFile, rec Record) (err error) {
	defer mon.Task()(&ctx)(&err)

	// we don't want to respect cancelling if the reader for the trashed piece goes away. we know it
	// was trashed so we should revive it no matter what.
	ctx = context2.WithoutCancellation(ctx)

	// 1. acquire the revive mutex. revive happens infrequently enough and the concurrency
	// considerations are difficult enough to limit it to one at a time.
	if err := s.reviveMu.Lock(ctx, &s.closed); err != nil {
		return err
	}
	defer s.reviveMu.Unlock()

	// 2. acquire the compaction mutex. while we have an open handle to the log file, we know that
	// we will be able to read it, so we don't have to worry about that. but we do have to worry
	// about compaction changing the state of the hashtbl and log file set, so we need to be
	// exclusive with that.
	if err := s.compactMu.Lock(ctx, &s.closed); err != nil {
		return err
	}
	defer s.compactMu.Unlock()

	// 3. create a writer for the entry.
	w, err := s.Create(ctx, rec.Key, time.Time{})
	if err != nil {
		return Error.Wrap(err)
	}
	defer w.Cancel()

	// 4. find the current state of the record. if found, we can just update the expiration and be
	// happy. as noted in 2, we're safe to do a lookup into s.tbl here even without the rmu held
	// because we know no compaction is ongoing due to having the mutex acquired, and compaction is
	// the only thing that does anything other than add entries to the hash table.
	if tmp, ok, err := s.tbl.Lookup(ctx, rec.Key); err == nil && ok {
		if tmp.Expires == 0 {
			return nil
		}

		tmp.Expires = 0
		return s.addRecord(ctx, tmp)
	}

	// 5. otherwise, we either had an error looking up the current record, or the entry got fully
	// deleted, and the open file handle is the last remaining evidence that it exists, so we have
	// to rewrite it. note that we purposefully do not close the log reader because after this
	// function exits, a log reader will be created and returned to the user using the same log
	// file.
	_, err = io.Copy(w, newLogReader(s, lf, rec))
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
	defer s.stats.compactions.Add(1) // increase the number of compactions that have finished

	start := time.Now()
	s.log.Info("beginning compaction", zap.Any("stats", s.Stats()))
	defer func() {
		s.log.Info("finished compaction",
			zap.Duration("duration", time.Since(start)),
			zap.Error(err),
			zap.Any("stats", s.Stats()),
		)
	}()

	// ensure only one compaction at a time.
	if err := s.compactMu.Lock(ctx, &s.closed); err != nil {
		return err
	}
	defer s.compactMu.Unlock()

	// stop all writers from flushing and wait for all current writers to finish flushing.
	if err := s.flushMu.Lock(ctx, &s.closed); err != nil {
		return err
	}
	defer s.flushMu.Unlock()

	// log that we acquired the locks only if it took a while.
	if dur := time.Since(start); dur > time.Second {
		s.log.Info("compaction acquired locks",
			zap.Duration("duration", dur),
		)
	}

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

	// define some functions to tell what state records are in based on what today is and what the
	// last restore time is.
	today := s.today()
	defer s.stats.lastCompact.Store(today)

	var restore uint32
	if !lastRestore.IsZero() {
		restore = TimeToDateUp(lastRestore)
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
		// if we delete trash immediately and it is trash, it is expired.
		if compaction_DeleteTrashImmediately && e.Trash() {
			return true
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

	// we will loop looking for a log to rewrite and compact the hash table without that log file
	// until we have no log files left to rewrite. this does more work (reads and writes the hash
	// table each time we need to write a log file) but ensures we use minimal extra disk space when
	// we need to rewrite multiple log files.
	for {
		completed, err := s.compactOnce(ctx, today, expired, restored, shouldTrash)
		if err != nil {
			return err
		} else if completed {
			break
		}
	}

	return nil
}

func (s *Store) compactOnce(
	ctx context.Context,
	today uint32,
	expired func(e Expiration) bool,
	restored func(e Expiration) bool,
	shouldTrash func(ctx context.Context, key Key, created time.Time) bool,
) (completed bool, err error) {
	defer mon.Task()(&ctx)(&err)

	start := time.Now()
	s.log.Info("compact once started", zap.Uint32("today", today))
	defer func() {
		s.log.Info("compact once finished",
			zap.Duration("duration", time.Since(start)),
			zap.Bool("completed", completed),
			zap.Error(err),
		)
	}()

	// reset the compaction values before storing the cached stats so that any Stats calls get
	// correct values. these are only read when s.stats.cached is set, so they must be cleared
	// before we set it.
	s.stats.startTime.Store(time.Now())
	s.stats.writeTime.Store(time.Time{}) // we store a zero value so that we always have a set time.
	s.stats.totalRecords.Store(0)
	s.stats.processedRecords.Store(0)

	// cache stats so that the call doesn't get inconsistent internal values and clear them out when
	// we're finished.
	stats := s.Stats()
	s.stats.cached.Store(&stats)
	defer s.stats.cached.Store(nil)

	// collect statistics about the hash table and how live each of the log files are.
	nset := uint64(0)
	nexist := uint64(0)
	alive := make(map[uint64]uint64)
	total := make(map[uint64]uint64)
	modifications := false

	if err := s.tbl.Range(ctx, func(ctx context.Context, rec Record) (bool, error) {
		if err := ctx.Err(); err != nil {
			return false, err
		}

		// keep track of every record that exists in the hash table and the total size of the
		// record and its footer. this differs from the log size field because of optimistic
		// padding.
		nexist++
		total[rec.Log] += uint64(rec.Length) + RecordSize // RecordSize for the record footer

		// if we're not yet sure we're modifying the hash table, we need to check our callbacks
		// on the record to see if the table would be modified. a record is modified when it is
		// flagged as trash or when it is restored.
		if !modifications {
			if shouldTrash != nil && !rec.Expires.Trash() && shouldTrash(ctx, rec.Key, DateToTime(rec.Created)) {
				modifications = true
			}
			if restored(rec.Expires) {
				modifications = true
			}
		}

		// if the record is expired, we will modify the hash table by not including the record.
		if expired(rec.Expires) {
			modifications = true
			return true, nil
		}

		// the record is included in the future hash table, so account for it in alive space.
		nset++
		alive[rec.Log] += uint64(rec.Length) + RecordSize // RecordSize for the record footer

		return true, nil
	}); err != nil {
		return false, err
	}

	// calculate a hash table size so that it targets just under a 0.5 load factor.
	logSlots := uint64(bits.Len64(nset)) + 1
	if logSlots < tbl_minLogSlots {
		logSlots = tbl_minLogSlots
	}

	// using the information, determine which log files are candidates for rewriting.
	rewriteCandidates := make(map[uint64]bool)
	if err := s.lfs.Range(func(id uint64, lf *logFile) (bool, error) {
		if err := ctx.Err(); err != nil {
			return false, err
		}

		if func() bool {
			// if the log is empty, no need to delete it just to create it again later.
			size := lf.size.Load()
			if size == 0 {
				return false
			}
			// compute the alive percent. if it's zero, always try to rewrite it.
			alive := float64(alive[id]) / float64(size)
			if alive == 0 {
				return true
			}
			// compute the probability and include it that frequently.
			prob := compaction_ProbabilityFactor * (1 - alive) / alive
			return mwc.Float64() < math.Pow(prob, compaction_ProbabilityPower)
		}() {
			rewriteCandidates[id] = true
		}

		return true, nil
	}); err != nil {
		return false, err
	}

	// if we have no rewrite candidates, then rewrite the log with the largest amount of dead data.
	// this helps the steady state of a node that is basically full to more eagerly reclaim space
	// for more uploads.
	if len(rewriteCandidates) == 0 {
		var maxDead uint64
		var maxLog *logFile
		_ = s.lfs.Range(func(id uint64, lf *logFile) (bool, error) {
			if dead := total[id] - alive[id]; dead > maxDead {
				maxDead, maxLog = dead, lf
			}
			return true, nil
		})
		if maxLog != nil {
			s.log.Info("including log due to no rewrite candidates",
				zap.Uint64("id", maxLog.id),
				zap.String("path", maxLog.fh.Name()),
				zap.String("dead", memory.FormatBytes(int64(maxDead))),
			)
			rewriteCandidates[maxLog.id] = true
		}
	}

	// sort the rewrite candidates by the amount of dead data in them. log files with more dead data
	// reclaim more space per amount of data rewritten, so do them first to minimize cost.
	rewriteCandidatesByDead := maps.Keys(rewriteCandidates)
	sort.Slice(rewriteCandidatesByDead, func(i, j int) bool {
		deadI := total[rewriteCandidatesByDead[i]] - alive[rewriteCandidatesByDead[i]]
		deadJ := total[rewriteCandidatesByDead[j]] - alive[rewriteCandidatesByDead[j]]
		return deadI > deadJ
	})

	// limit the number of log files we rewrite in a single compaction to so that we write around
	// the amount of a size of the new hashtbl times the multiple. this bounds the extra space
	// necessary to compact.
	rewrite := make(map[uint64]bool)
	target := uint64(float64(hashtblSize(logSlots)) * compaction_RewriteMultiple)
	for _, id := range rewriteCandidatesByDead {
		if alive[id] <= target {
			rewrite[id] = true
			target -= alive[id]
		}
	}

	// special case: if we have some values in rewriteCandidates but we have no files in rewrite we
	// need to include one to ensure progress, unless the RewriteMultiple is 0 so we don't want to
	// do any rewriting.
	if len(rewriteCandidates) > 0 && len(rewrite) == 0 && compaction_RewriteMultiple > 0 {
		for id := range rewriteCandidates {
			rewrite[id] = true
			break
		}
	}

	// log about the compaction read stats, skipping the construction of the slices for which logs
	// we are rewriting if the log level is disabled.
	if ce := s.log.Check(zapcore.InfoLevel, "compaction computed details"); ce != nil {
		rewriteSorted := maps.Keys(rewrite)
		sort.Slice(rewriteSorted, func(i, j int) bool {
			return rewriteSorted[i] < rewriteSorted[j]
		})

		ce.Write(
			zap.Uint64("nset", nset),
			zap.Uint64("nexist", nexist),
			zap.Bool("modifications", modifications),
			zap.Uint64("curr logSlots", s.tbl.LogSlots()),
			zap.Uint64("next logSlots", logSlots),
			zap.Uint64s("candidates", rewriteCandidatesByDead),
			zap.Uint64s("rewrite", rewriteSorted),
			zap.Duration("duration", time.Since(start)),
		)
	}

	// if there are no modifications to the hashtbl to remove expired records or flag records as
	// trash, and we have no log file candidates to rewrite, and the hashtable would be the same
	// size, and it would be the same kind of table, we can exit early.
	if !modifications &&
		len(rewriteCandidates) == 0 &&
		logSlots == s.tbl.LogSlots() &&
		s.tbl.Header().Kind == table_DefaultKind {
		return true, nil
	}

	// increment the number of log files we're attempting to rewrite.
	s.stats.logsRewritten.Add(uint64(len(rewrite)))

	// create a new hash table sized for the number of records.
	tblPath := filepath.Join(s.tablePath, fmt.Sprintf("hashtbl-%016x", s.maxHash.Add(1)))
	af, err := newAtomicFile(tblPath)
	if err != nil {
		return false, Error.Wrap(err)
	}
	defer af.Cancel()

	cons, err := CreateTable(ctx, af.File, logSlots, today, table_DefaultKind)
	if err != nil {
		return false, Error.Wrap(err)
	}
	defer cons.Close()

	// keep track of statistics about some events that can happen to records during the compaction.
	totalRecords := uint64(0)
	totalBytes := uint64(0)
	rewrittenRecords := uint64(0)
	rewrittenBytes := uint64(0)
	trashedRecords := uint64(0)
	trashedBytes := uint64(0)
	restoredRecords := uint64(0)
	restoredBytes := uint64(0)
	expiredRecords := uint64(0)
	expiredBytes := uint64(0)
	reclaimedLogs := uint64(0)
	reclaimedBytes := uint64(0)

	// keep track of all of the rewritten records
	ri := new(rewrittenIndex)

	// if we have any rewrite logs, then all of the records we will be rewriting when we create the
	// next hashtbl so we can sort them and rewrite them early.
	if len(rewrite) > 0 && compaction_OrderedRewrite {
		rewriteStart := time.Now()

		if err := s.tbl.Range(ctx, func(ctx context.Context, rec Record) (bool, error) {
			if err := ctx.Err(); err != nil {
				return false, err
			}

			// we can rewrite records without breaking anything and rather than call into all the
			// shouldTrash/restored callbacks multiple times, we'll just grab an approximation
			// and if we rewrite too many, fine. and if we rewrite too few, we just have to do
			// some double checks in the next loop over the table. this should still be the vast
			// majority of rewrites, so we'll still get mostly linear writes.
			if rewrite[rec.Log] && !expired(rec.Expires) {
				ri.add(rec)
			}

			return true, nil
		}); err != nil {
			return false, err
		}

		// sort by log and offset so that we can rewrite them in disk order.
		ri.sortByLogOff()

		// update the total number of records expected to be rewritten in this compaction.
		s.stats.totalRecords.Store(uint64(len(ri.records)))
		s.stats.processedRecords.Store(0)

		// rewrite each record and update the record in the index.
		for i := range ri.records {
			s.stats.processedRecords.Add(1) // bump the number of records processed for progress reporting.

			err := func() error {
				if err := ctx.Err(); err != nil {
					return err
				}

				rec, err := s.rewriteRecord(ctx, ri.records[i], rewriteCandidates)
				if err != nil {
					return Error.Wrap(err)
				}
				ri.records[i] = rec

				// bump the amount of data we rewrote.
				s.stats.dataRewritten.Add(uint64(rec.Length) + RecordSize)

				// keep track of the number of records and bytes we rewrote for logs.
				rewrittenRecords++
				rewrittenBytes += uint64(rec.Length)

				return nil
			}()
			if err != nil {
				return false, Error.Wrap(err)
			}
		}

		// now sort by key so that we can efficiently look up if we rewrote them in the next loop.
		ri.sortByKey()

		// log about the time we took rewriting the records.
		if ce := s.log.Check(zapcore.InfoLevel, "records rewritten"); ce != nil {
			ce.Write(
				zap.Uint64("records", rewrittenRecords),
				zap.String("bytes", memory.FormatBytes(int64(rewrittenBytes))),
				zap.Duration("duration", time.Since(rewriteStart)),
			)
		}

	}

	// update the values for progress reporting.
	s.stats.writeTime.Store(time.Now())
	s.stats.totalRecords.Store(nexist)
	s.stats.processedRecords.Store(0)

	// copy all of the entries from the hash table to the new table, skipping expired entries, and
	// rewriting any entries that are in the log files that we are rewriting.
	if err := s.tbl.Range(ctx, func(ctx context.Context, rec Record) (bool, error) {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		s.stats.processedRecords.Add(1) // bump the number of records processed for progress reporting.

		// trash records are flagged as expired some number of days from now with a bit set to
		// signal if they are read that there was a problem. we only check records that are not
		// already flagged as trashed and keep the minimum time for the record to live. we do this
		// after compaction so that we don't mistakenly count it as a "revive".
		if shouldTrash != nil && !rec.Expires.Trash() && shouldTrash(ctx, rec.Key, DateToTime(rec.Created)) {
			expiresTime := today + compaction_ExpiresDays
			// if we have an existing ttl time and it's smaller, use that instead.
			if existingTime := rec.Expires.Time(); existingTime > 0 && existingTime < expiresTime {
				expiresTime = existingTime
			}
			// only update the expired time if it won't immediately be restored. this ensures we
			// dont clear out the ttl field for no reason right after this.
			if exp := NewExpiration(expiresTime, true); !restored(exp) {
				rec.Expires = exp

				trashedRecords++
				trashedBytes += uint64(rec.Length)
			}
		}

		// if the record is restored, clear the expiration. we do this after checking if the record
		// should be trashed to ensure that restore always has precedence.
		if restored(rec.Expires) {
			rec.Expires = 0

			// we bump created so that the shouldTrash callback likely ignores it in case the bloom
			// filter was bad or something. this may change once the hashstore is more integrated
			// with the system and it has more details about the bloom filter.
			rec.Created = today

			restoredRecords++
			restoredBytes += uint64(rec.Length)
		}

		// totally ignore any expired records.
		if expired(rec.Expires) {
			expiredRecords++
			expiredBytes += uint64(rec.Length)

			return true, nil
		}

		// if the log is being rewritten, copy the record into the a different log file.
		if rewrite[rec.Log] {
			// if we already rewrote the record earlier, then update the record to be the new
			// rewritten one. otherwise, we have to rewrite it now. note that the above code may
			// have changed some fields, so we only want to update the log and offset fields. the
			// earlier loop ensures that only the log and offset fields are different.
			if i, ok := ri.findKey(rec.Key); ok && !store_TestIgnoreRewrittenIndex {
				rec.Log, rec.Offset = ri.records[i].Log, ri.records[i].Offset
			} else {
				rewrittenRec, err := s.rewriteRecord(ctx, rec, rewriteCandidates)
				if err != nil {
					return false, Error.Wrap(err)
				}
				rec = rewrittenRec

				// bump the amount of data we rewrote.
				s.stats.dataRewritten.Add(uint64(rec.Length) + RecordSize)

				// keep track of the number of records and bytes we rewrote for logs.
				rewrittenRecords++
				rewrittenBytes += uint64(rec.Length)
			}
		}

		// insert the record into the new hash table.
		if ok, err := cons.Append(ctx, rec); err != nil {
			return false, Error.Wrap(err)
		} else if !ok {
			return false, Error.New("compaction hash table is full")
		}

		totalRecords++
		totalBytes += uint64(rec.Length)

		return true, nil
	}); err != nil {
		return false, err
	}

	// help out the compiler/runtime by explicitly clearing out the index now that it's not needed.
	// it's possible the liveness analysis does this, but it's nice to not have to rely on that.
	ri = nil

	ntbl, err := cons.Done(ctx)
	if err != nil {
		return false, Error.Wrap(err)
	}

	// commit the new hash table. there should be no error cases in this function after this point
	// because a process restart may have the store open with this new hash table, so we have to go
	// forward with it.
	if err := af.Commit(); err != nil {
		return false, Error.New("unable to commit newly compacted hashtbl: %w", err)
	}

	// swap the new hash table in and collect the set of log files to remove. we don't close and
	// remove the log files while holding the lock to avoid doing i/o while blocking readers.
	s.rmu.Lock()
	otbl := s.tbl
	s.tbl = ntbl

	toRemove := make([]*logFile, 0, len(rewrite))
	for id := range rewrite {
		if lf, ok := s.lfs.LoadAndDelete(id); ok {
			toRemove = append(toRemove, lf)
		}
	}
	s.rmu.Unlock()

	// now that we are no longer holding the mutex, close and remove the old hashtbl and close and
	// remove the newly dead log files. log files have protection to not actually close the
	// underlying file handle until the last reader is finished. we have to strip the .tmp suffix on
	// the hashtbl file name because the file handles were potentially created with .tmp before
	// being renamed in place, which does not update their name.
	otbl.Close()
	_ = os.Remove(strings.TrimSuffix(otbl.Handle().Name(), ".tmp"))

	for _, lf := range toRemove {
		lf.Close()
		lf.Remove()

		s.stats.dataReclaimed.Add(lf.size.Load())

		reclaimedLogs++
		reclaimedBytes += lf.size.Load()
	}

	// best effort sync the directories now that we are done with mutations.
	syncDirectory(s.tablePath)
	syncDirectory(s.logsPath)

	// before we allow writers to proceed, reinitialize the heap with the log files so that it has
	// the best set of logs to write into and doesn't contain any now closed/removed logs.
	s.lfc.Clear()
	_ = s.lfs.Range(func(_ uint64, lf *logFile) (bool, error) {
		s.lfc.Include(lf)
		return true, nil
	})

	// log information about important events that happened to records during the writing of the new
	// hashtbl.
	if ce := s.log.Check(zapcore.InfoLevel, "hashtbl rewritten"); ce != nil {
		ce.Write(
			zap.Duration("duration", time.Since(s.stats.writeTime.Load().(time.Time))),
			zap.Uint64("total records", totalRecords),
			zap.String("total bytes", memory.FormatBytes(int64(totalBytes))),
			zap.Uint64("rewritten records", rewrittenRecords),
			zap.String("rewritten bytes", memory.FormatBytes(int64(rewrittenBytes))),
			zap.Uint64("trashed records", trashedRecords),
			zap.String("trashed bytes", memory.FormatBytes(int64(trashedBytes))),
			zap.Uint64("restored records", restoredRecords),
			zap.String("restored bytes", memory.FormatBytes(int64(restoredBytes))),
			zap.Uint64("expired records", expiredRecords),
			zap.String("expired bytes", memory.FormatBytes(int64(expiredBytes))),
			zap.Uint64("reclaimed logs", reclaimedLogs),
			zap.String("reclaimed bytes", memory.FormatBytes(int64(reclaimedBytes))),
		)
	}

	// if we rewrote every log file that we could potentially rewrite, then we're done. len is
	// sufficient here because rewrite is a subset of rewriteCandidates. also if our rewrite
	// multiple is 0, then we're done because we unlinked all the fully dead log files already.
	return len(rewriteCandidates) == len(rewrite) || compaction_RewriteMultiple == 0, nil
}

func (s *Store) rewriteRecord(ctx context.Context, rec Record, rewriteCandidates map[uint64]bool) (_ Record, err error) {
	defer mon.Task()(&ctx)(&err)

	r, err := s.readerForRecord(ctx, rec)
	if err != nil {
		return rec, Error.Wrap(err)
	}
	defer r.Release() // same as r.Close() but no error to worry about.

	// WARNING! this is subtle, but what we do is take the log file directly out of the reader, seek
	// it to the appropriate place, and use an io.LimitReader so that the go stdlib using io.Copy
	// will do copy_file_range if available avoiding the copy into userspace. it would be a problem
	// if multiple concurrent readers or writers were using the file pos at the same time. in the
	// case of this code it's safe to use Seek because rewriteRecord is only called during
	// compaction which means there are no writers and compaction does not call it in parallel so
	// there is only one reader that uses the pos and it must be us. additionally, all of our log
	// write operations seek to the end of the file before doing any writing to ensure that any
	// pos updates that happen here are not a problem.
	var from io.Reader = r
	if _, err := r.lf.fh.Seek(int64(rec.Offset), io.SeekStart); err == nil {
		from = r.lf.fh // we use io.CopyN below so it will be limited by the length.
	}

	// acquire a log file to write the entry into. if we're rewriting that log file or the record is
	// already in that log file, we have to pick a different one.
	var into *logFile
	for into == nil || rewriteCandidates[into.id] || into.id == rec.Log {
		into, err = s.acquireLogFile(rec.Expires.Time())
		if err != nil {
			return rec, Error.Wrap(err)
		}
		defer s.lfc.Include(into) //nolint intentionally defer in the loop
	}

	// get the current offset so that we are sure we have the correct spot for the record.
	offset, err := into.fh.Seek(0, io.SeekEnd)
	if err != nil {
		return rec, Error.Wrap(err)
	}

	// copy the record data.
	if _, err := io.CopyN(into.fh, from, int64(rec.Length)); err != nil {
		// if we couldn't write the data, we should abort the write operation and attempt to reclaim
		// space by truncating to the saved offset.
		_ = into.fh.Truncate(offset)
		return rec, Error.New("writing into compacted log: %w", err)
	}

	// update the record location.
	rec.Log = into.id
	rec.Offset = uint64(offset)

	// append the record to the log file for reconstruction.
	var buf [RecordSize]byte
	rec.WriteTo(&buf)

	if _, err := into.fh.Write(buf[:]); err != nil {
		// if we can't add the record, we should abort the write operation and attempt to reclaim
		// space by tuncating to the saved offset.
		_ = into.fh.Truncate(offset)
		return rec, Error.New("writing record into compacted log: %w", err)
	}

	// increase our in-memory estimate of the size of the log file for sorting. we use store to
	// ensure that it maintains correctness if there were some errors in the past.
	into.size.Store(uint64(offset) + uint64(rec.Length) + uint64(len(buf)))

	// return the updated record.
	return rec, nil
}
