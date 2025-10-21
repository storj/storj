// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"context"
	"encoding/binary"
	"math/bits"
	"os"
	"sync"

	"github.com/zeebo/errs"
	"github.com/zeebo/xxh3"

	"storj.io/common/memory"
	"storj.io/drpc/drpcsignal"
	"storj.io/storj/storagenode/hashstore/platform"
)

const hashtbl_invalidPage = 1<<64 - 1

// HashTbl is an on disk hash table of records.
type HashTbl struct {
	fh       *os.File  // file handle backing the hashtbl
	logSlots uint64    // log_2 of the maximum number of slots
	numSlots slotIdxT  // 1 << logSlots, the actual maximum number of slots
	slotMask slotIdxT  // numSlots - 1, a bit mask for the maximum number of slots
	header   TblHeader // header information

	opMu rwMutex // protects operations

	closed drpcsignal.Signal // closed state
	cloErr error             // close error
	cloMu  sync.Mutex        // synchronizes closing

	buffer *rwBigPageCache // buffer for inserts

	mmap *mmapCache // memory mapped file if in mmap mode

	statsMu  sync.Mutex // protects the following fields
	recStats recordStats
}

// hashtblSize returns the size in bytes of the hashtbl given an logSlots.
func hashtblSize(logSlots uint64) uint64 { return tbl_headerSize + 1<<logSlots*RecordSize }

type (
	slotIdxT    uint64 // index of a slot in the hashtbl
	pageIdxT    uint64 // index of a page in the hashtbl
	bigPageIdxT uint64 // index of a bigPage in the hashtbl
)

func (s slotIdxT) PageIndexes() (pageIdxT, uint64) {
	return pageIdxT(s / recordsPerPage), uint64(s % recordsPerPage)
}

func (s slotIdxT) BigPageIndexes() (bigPageIdxT, uint64) {
	return bigPageIdxT(s / recordsPerBigPage), uint64(s % recordsPerBigPage)
}

func (s slotIdxT) Offset() int64    { return tbl_headerSize + int64(s*RecordSize) }
func (p pageIdxT) Offset() int64    { return tbl_headerSize + int64(p*pageSize) }
func (p bigPageIdxT) Offset() int64 { return tbl_headerSize + int64(p*bigPageSize) }

// CreateHashTbl allocates a new hash table with the given log base 2 number of records and created
// timestamp. The file is truncated and allocated to the correct size.
func CreateHashTbl(ctx context.Context, fh *os.File, logSlots uint64, created uint32, cfg MmapCfg) (_ *HashTblConstructor, err error) {
	defer mon.Task()(&ctx)(&err)

	if logSlots > tbl_maxLogSlots {
		return nil, Error.New("logSlots too large: logSlots=%d", logSlots)
	} else if logSlots < tbl_minLogSlots {
		return nil, Error.New("logSlots too small: logSlots=%d", logSlots)
	}

	header := TblHeader{
		Created:  created,
		HashKey:  true,
		Kind:     TableKind_HashTbl,
		LogSlots: logSlots,
	}

	// clear the file and truncate it to the correct length and write the header page.
	size := int64(hashtblSize(logSlots))
	if size < tbl_headerSize+bigPageSize {
		return nil, Error.New("hashtbl size too small: size=%d logSlots=%d", size, logSlots)
	} else if err := fh.Truncate(0); err != nil {
		return nil, Error.New("unable to truncate hashtbl to 0: %w", err)
	} else if err := fh.Truncate(size); err != nil {
		return nil, Error.New("unable to truncate hashtbl to %d: %w", size, err)
	} else if err := platform.Fallocate(fh, size); err != nil {
		return nil, Error.New("unable to fallocate hashtbl to %d: %w", size, err)
	} else if err := WriteTblHeader(fh, header); err != nil {
		return nil, Error.Wrap(err)
	}

	// this is a bit wasteful in the sense that we will do some stat calls, reread the header page,
	// and compute estimates, but it reduces code paths and is not that expensive overall.
	h, _, err := OpenHashTbl(ctx, fh, cfg)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return newHashTblConstructor(h), nil
}

// OpenHashTbl opens an existing hash table stored in the given file handle.
func OpenHashTbl(ctx context.Context, fh *os.File, cfg MmapCfg) (_ *HashTbl, _ map[uint64]*RecordTail, err error) {
	defer mon.Task()(&ctx)(&err)

	// compute the number of records from the file size of the hash table.
	size, err := fileSize(fh)
	if err != nil {
		return nil, nil, Error.New("unable to determine hashtbl size: %w", err)
	} else if size < tbl_headerSize+pageSize { // header page + at least 1 page of records
		return nil, nil, Error.New("hashtbl file too small: size=%d", size)
	}

	// compute the logSlots from the size.
	logSlots := uint64(bits.Len64(uint64(size-tbl_headerSize)/RecordSize) - 1)

	// sanity check that our logSlots is correct.
	if int64(hashtblSize(logSlots)) != size {
		return nil, nil, Error.New("logSlots calculation mismatch: size=%d logSlots=%d", size, logSlots)
	} else if logSlots > tbl_maxLogSlots {
		return nil, nil, Error.New("logSlots too large: logSlots=%d", logSlots)
	} else if logSlots < tbl_minLogSlots {
		return nil, nil, Error.New("logSlots too small: logSlots=%d", logSlots)
	}

	// read the header information from the first page.
	header, err := ReadTblHeader(fh)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	} else if header.Kind != TableKind_HashTbl {
		return nil, nil, Error.New("invalid kind: %d", header.Kind)
	}

	// zero is allowed for backward compatibility. but if it's specified, it had better match the
	// file size or we got truncated or something.
	if header.LogSlots != 0 && header.LogSlots != logSlots {
		return nil, nil, Error.New("logSlots mismatch: header=%d file=%d", header.LogSlots, logSlots)
	}

	h := &HashTbl{
		fh:       fh,
		logSlots: logSlots,
		numSlots: 1 << logSlots,
		slotMask: 1<<logSlots - 1,
		header:   header,
	}
	defer func() {
		if err != nil {
			h.fh = nil // don't close the file handle if we're returning an error
			_ = h.Close()
		}
	}()

	if cfg.Mmap && platform.MmapSupported {
		data, err := platform.Mmap(fh, int(size))
		if err != nil {
			return nil, nil, Error.Wrap(err)
		}
		h.mmap = newMMAPCache(data)

		if cfg.Mlock && platform.MmapSupported {
			_ = platform.Mlock(data)
		}

		// lookups do random access, and range is sequential but sets it itself, so default to
		// random.
		platform.AdviseRandom(data)
	}

	// compute initial stats and collect tails.
	tails, err := h.loadEntries(ctx)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	return h, tails, nil
}

// Stats returns a TblStats about the hash table.
func (h *HashTbl) Stats() TblStats {
	h.statsMu.Lock()
	defer h.statsMu.Unlock()

	return TblStats{
		NumSet: h.recStats.numSet,
		LenSet: memory.Size(h.recStats.lenSet),
		AvgSet: safeDivide(float64(h.recStats.lenSet), float64(h.recStats.numSet)),

		NumTrash: h.recStats.numTrash,
		LenTrash: memory.Size(h.recStats.lenTrash),
		AvgTrash: safeDivide(float64(h.recStats.lenTrash), float64(h.recStats.numTrash)),

		NumTTL: h.recStats.numTTL,
		LenTTL: memory.Size(h.recStats.lenTTL),
		AvgTTL: safeDivide(float64(h.recStats.lenTTL), float64(h.recStats.numTTL)),

		NumSlots:  uint64(h.numSlots),
		TableSize: memory.Size(hashtblSize(h.logSlots)),
		Load:      safeDivide(float64(h.recStats.numSet), float64(h.numSlots)),

		Created: h.header.Created,
		Kind:    h.header.Kind,
	}
}

// LogSlots returns the logSlots the table was created with.
func (h *HashTbl) LogSlots() uint64 { return h.logSlots }

// Header returns the TblHeader the table was created with.
func (h *HashTbl) Header() TblHeader { return h.header }

// Handle returns the file handle the table was created with.
func (h *HashTbl) Handle() *os.File { return h.fh }

// Close closes the hash table and returns when no more operations are running.
func (h *HashTbl) Close() error {
	h.cloMu.Lock()
	defer h.cloMu.Unlock()

	if !h.closed.Set(Error.New("hashtbl closed")) {
		return h.cloErr
	}

	// grab the lock to ensure all operations have finished before closing the file handle.
	h.opMu.WaitLock()
	defer h.opMu.Unlock()

	if h.mmap != nil {
		_ = platform.Munmap(h.mmap.data)
		h.mmap = nil
	}

	if h.fh != nil {
		h.cloErr = errs.Combine(
			h.fh.Sync(),
			h.fh.Close(),
		)
	}

	return h.cloErr
}

// slotForKey computes the slot for the given key.
func (h *HashTbl) slotForKey(k *Key) slotIdxT {
	var v uint64
	if h.header.HashKey {
		v = xxh3.Hash(k[:])
	} else {
		v = binary.BigEndian.Uint64(k[0:8])
	}
	s := (64 - h.logSlots) % 64
	return slotIdxT(v>>s) & h.slotMask
}

func (h *HashTbl) loadEntries(ctx context.Context) (_ map[uint64]*RecordTail, err error) {
	defer mon.Task()(&ctx)(&err)

	tails := make(map[uint64]*RecordTail)
	if err := h.Range(ctx, func(ctx context.Context, rec Record) (bool, error) {
		rt := tails[rec.Log]
		if rt == nil {
			rt = new(RecordTail)
			tails[rec.Log] = rt
		}
		rt.Push(rec)

		return true, nil
	}); err != nil {
		return nil, err
	}

	for _, rt := range tails {
		rt.Sort()
	}

	return tails, nil
}

// Load returns an estimate of what fraction of the hash table is occupied.
func (h *HashTbl) Load() float64 {
	h.statsMu.Lock()
	defer h.statsMu.Unlock()

	return safeDivide(float64(h.recStats.numSet), float64(h.numSlots))
}

// Range iterates over the records in hash table order.
func (h *HashTbl) Range(ctx context.Context, fn func(context.Context, Record) (bool, error)) (err error) {
	if err := h.opMu.RLock(ctx, &h.closed); err != nil {
		return err
	}
	defer h.opMu.RUnlock()

	// if we are in mmap mode, range is sequential, so advise that and then advise random after.
	if h.mmap != nil {
		platform.AdviseSequential(h.mmap.data)
		defer platform.AdviseRandom(h.mmap.data)
	}

	var (
		recStats recordStats

		rec   Record
		valid bool
	)

	var cache *roBigPageCache
	if h.mmap == nil {
		cache = new(roBigPageCache)
		cache.Init(h.fh)
	}

	for slot := slotIdxT(0); slot < h.numSlots; slot++ {
		if cache != nil {
			valid, err = cache.ReadRecord(slot, &rec)
		} else {
			valid, err = h.mmap.ReadRecord(slot, &rec)
		}
		if err != nil {
			return Error.Wrap(err)
		} else if valid {
			if ok, err := fn(ctx, rec); err != nil {
				return err
			} else if !ok {
				return nil
			}

			recStats.Include(rec)
		}
	}

	h.statsMu.Lock()
	h.recStats = recStats
	h.statsMu.Unlock()

	return nil
}

// Insert adds a record to the hash table. It returns (true, nil) if the record was inserted, it
// returns (false, nil) if the hash table is full, and (false, err) if any errors happened trying to
// insert the record.
func (h *HashTbl) Insert(ctx context.Context, rec Record) (_ bool, err error) {
	if err := h.opMu.Lock(ctx, &h.closed); err != nil {
		return false, err
	}
	defer h.opMu.Unlock()

	return h.insertLocked(ctx, rec)
}

func (h *HashTbl) insertLocked(ctx context.Context, rec Record) (_ bool, err error) {
	var (
		tmp   Record
		valid bool

		insert = true // keep track of if this is an insert or an update.
	)

	var cache *rwPageCache
	if h.mmap == nil {
		cache = new(rwPageCache)
		cache.Init(h.fh)
	}

	for slot, attempt := h.slotForKey(&rec.Key), slotIdxT(0); attempt < h.numSlots; slot, attempt = (slot+1)&h.slotMask, attempt+1 {
		if err := ctx.Err(); err != nil {
			return false, err
		} else if err := signalError(&h.closed); err != nil {
			return false, err
		}

		// note that in lookup, we protect against lost pages by reading at least 2 pages worth of
		// records before bailing due to an invalid record. we don't do that here so it's possible
		// in the presence of lost pages to have the same key present twice and the latter one be
		// effectively unreadable and take up a slot. this isn't that big of a deal because reads
		// will find the newer entry first, and the hash table should be compacted eventually and
		// the earlier value removed. unfortunately, the later value will be iterated over first
		// (most of the time. in rare cases the later value may probe past the end of the hash table
		// into the earlier pages), and we don't want compaction to cause values to go backwards by
		// overwriting the later value with the earlier value. fortunately, the only time records
		// should ever be mutated is if they are revived after being flagged trash during a previous
		// compaction and so we can error if the fields don't match except for the expiration field
		// which we can take to be the longer lasting value.

		if h.buffer != nil {
			valid, err = h.buffer.ReadRecord(slot, &tmp)
		} else if cache != nil {
			valid, err = cache.ReadRecord(slot, &tmp)
		} else {
			valid, err = h.mmap.ReadRecord(slot, &tmp)
		}
		if err != nil {
			return false, Error.Wrap(err)
		}

		// if we have a valid record, we need to do some checks.
		if valid {
			// if it is some other key, the slot is occupied and we need to probe further.
			if tmp.Key != rec.Key {
				continue
			}

			// otherwise, it is our key, and as noted above we need to merge the records, erroring
			// if fields are mutated, and picking the "larger" expiration time.
			if !RecordsEqualish(rec, tmp) {
				return false, Error.New("put:%v != exist:%v: %w", rec, tmp, ErrCollision)
			}

			rec.Expires = MaxExpiration(rec.Expires, tmp.Expires)
			insert = false
		}

		// thus it is either invalid or the key matches and the record is updated, so we can write.
		if h.buffer != nil {
			err = h.buffer.WriteRecord(slot, &rec)
		} else if cache != nil {
			err = cache.WriteRecord(slot, &rec)
		} else {
			err = h.mmap.WriteRecord(slot, &rec)
		}
		if err != nil {
			return false, Error.Wrap(err)
		}

		// if we had an insert record, we are adding a new key, so include it in the stats. every other
		// case of update does not change the length, so we don't need to update the stats.
		if insert {
			h.statsMu.Lock()
			h.recStats.Include(rec)
			h.statsMu.Unlock()
		}

		return true, nil
	}

	return false, nil
}

// Lookup returns the record for the given key if it exists in the hash table. It returns (rec,
// true, nil) if the record existed, (rec{}, false, nil) if it did not exist, and (rec{}, false,
// err) if any errors happened trying to look up the record.
func (h *HashTbl) Lookup(ctx context.Context, key Key) (_ Record, _ bool, err error) {
	if err := h.opMu.RLock(ctx, &h.closed); err != nil {
		return Record{}, false, err
	}
	defer h.opMu.RUnlock()

	var (
		rec   Record
		valid bool
	)

	var cache *roPageCache
	if h.mmap == nil {
		cache = new(roPageCache)
		cache.Init(h.fh)
	}

	for slot, attempt := h.slotForKey(&key), slotIdxT(0); attempt < h.numSlots; slot, attempt = (slot+1)&h.slotMask, attempt+1 {
		if err := ctx.Err(); err != nil {
			return Record{}, false, err
		} else if err := signalError(&h.closed); err != nil {
			return Record{}, false, err
		}

		if cache != nil {
			valid, err = cache.ReadRecord(slot, &rec)
		} else {
			valid, err = h.mmap.ReadRecord(slot, &rec)
		}
		if err != nil {
			return Record{}, false, Error.Wrap(err)
		} else if !valid {
			// even if the record is invalid, keep looking for up to 2 pages. this causes us more
			// reads when looking up a key that does not exist, but helps us find keys that maybe do
			// exist if a page write was lost. fortunately, we often do not get queried for keys
			// that do not exist, so this should not be expensive.
			if attempt < 2*recordsPerPage {
				continue
			}
			return Record{}, false, nil
		} else if rec.Key == key {
			return rec, true, nil
		}
	}

	return Record{}, false, nil
}

// Sync syncs any modifications to disk.
func (h *HashTbl) Sync(ctx context.Context) (err error) {
	if err := h.opMu.Lock(ctx, &h.closed); err != nil {
		return err
	}
	defer h.opMu.Unlock()

	return Error.Wrap(h.fh.Sync())
}

//
// hashtbl constructor
//

// HashTblConstructor constructs a HashTbl.
type HashTblConstructor struct {
	h   *HashTbl
	err error
}

// newHashTblConstructor constructs a new HashTblConstructor.
func newHashTblConstructor(h *HashTbl) *HashTblConstructor {
	// set the hashtbl into big page buffer mode if we're not in mmap mode.
	if h.mmap == nil {
		h.buffer = new(rwBigPageCache)
		h.buffer.Init(h.fh)
	}

	return &HashTblConstructor{h: h}
}

// valid is a helper function to convert failure conditions into an error. It is small enough to be
// inlined.
func (c *HashTblConstructor) valid() error {
	if c.err != nil {
		return c.err
	} else if c.h == nil {
		return Error.New("constructor already done")
	}
	return nil
}

// Cancel signals that we're done with the HashTblConstructor. It should always be called.
func (c *HashTblConstructor) Cancel() {
	if c.h != nil {
		_ = c.h.Close()
		c.h = nil
	}
}

// Append adds the record into the HashTbl. Errors are sticky and will prevent further appends.
// Appending records in the "naural" order for the HashTbl will go faster than random order.
func (c *HashTblConstructor) Append(ctx context.Context, r Record) (bool, error) {
	if err := c.valid(); err != nil {
		return false, err
	}
	var ok bool
	ok, c.err = c.h.insertLocked(ctx, r)
	return ok, c.err
}

// Done returns the constructed HashTbl or an error if there was a problem. The returned Tbl must be
// closed if it is not nil.
func (c *HashTblConstructor) Done(ctx context.Context) (t Tbl, err error) {
	if err := c.valid(); err != nil {
		return nil, err
	}

	if buf := c.h.buffer; buf != nil {
		c.err = buf.Flush()
		c.h.buffer = nil
		if c.err != nil {
			return nil, c.err
		}
	}

	// valid returns an error if the memtbl field is nil, so we don't have to worry about putting
	// a nil pointer in the interface.
	h := c.h
	c.h = nil

	return h, nil
}

//
// mmap based pass through cache
//

type mmapCache struct{ data []byte }

func newMMAPCache(data []byte) *mmapCache { return &mmapCache{data: data} }

func (c *mmapCache) ReadRecord(slot slotIdxT, rec *Record) (valid bool, err error) {
	start := uint64(slot.Offset())
	end := start + RecordSize
	if start < uint64(len(c.data)) && end <= uint64(len(c.data)) && start <= end {
		return rec.ReadFrom((*[64]byte)(c.data[start:end])), nil
	}
	return false, Error.New("slot out of bounds: slot=%d", slot)
}

func (c *mmapCache) WriteRecord(slot slotIdxT, rec *Record) (err error) {
	start := uint64(slot.Offset())
	end := start + RecordSize
	if start < uint64(len(c.data)) && end <= uint64(len(c.data)) && start <= end {
		rec.WriteTo((*[64]byte)(c.data[start:end]))
		return nil
	}
	return Error.New("slot out of bounds: slot=%d", slot)
}

//
// read-only hashtbl page caches
//

type roPageCache struct {
	fh *os.File
	i  pageIdxT
	p  page
}

func (c *roPageCache) Init(fh *os.File) {
	c.fh = fh
	c.i = hashtbl_invalidPage
}

func (c *roPageCache) ReadRecord(slot slotIdxT, rec *Record) (valid bool, err error) {
	pi, ri := slot.PageIndexes()
	if pi != c.i {
		c.i = hashtbl_invalidPage // invalidate the page in case the read fails
		if _, err := c.fh.ReadAt(c.p[:], pi.Offset()); err != nil {
			return false, Error.Wrap(err)
		}
		c.i = pi
	}
	return c.p.readRecord(ri, rec), nil
}

type roBigPageCache struct {
	fh *os.File
	i  bigPageIdxT
	p  bigPage
}

func (c *roBigPageCache) Init(fh *os.File) {
	c.fh = fh
	c.i = hashtbl_invalidPage
}

func (c *roBigPageCache) ReadRecord(slot slotIdxT, rec *Record) (valid bool, err error) {
	pi, ri := slot.BigPageIndexes()
	if pi != c.i {
		c.i = hashtbl_invalidPage // invalidate the page in case the read fails
		if _, err := c.fh.ReadAt(c.p[:], pi.Offset()); err != nil {
			return false, Error.Wrap(err)
		}
		c.i = pi
	}
	return c.p.readRecord(ri, rec), nil
}

//
// write-back hashtbl page cache
//

type rwPageCache struct {
	fh *os.File
	i  pageIdxT
	p  page
}

func (c *rwPageCache) Init(fh *os.File) {
	c.fh = fh
	c.i = hashtbl_invalidPage
}

func (c *rwPageCache) setPage(pi pageIdxT) (err error) {
	if c.i == pi {
		return nil
	}
	c.i = hashtbl_invalidPage // invalidate the page in case the read fails
	if _, err := c.fh.ReadAt(c.p[:], pi.Offset()); err != nil {
		return Error.Wrap(err)
	}
	c.i = pi
	return nil
}

func (c *rwPageCache) ReadRecord(slot slotIdxT, rec *Record) (valid bool, err error) {
	pi, ri := slot.PageIndexes()
	if err := c.setPage(pi); err != nil {
		return false, Error.Wrap(err)
	}
	return c.p.readRecord(ri, rec), nil
}

func (c *rwPageCache) WriteRecord(slot slotIdxT, rec *Record) (err error) {
	// directly write the record because this page cache is used in situations where we expect only
	// a single record to be written.
	var buf [RecordSize]byte
	rec.WriteTo(&buf)
	_, err = c.fh.WriteAt(buf[:], slot.Offset())

	// update or invalidate our in memory page
	if pi, ri := slot.PageIndexes(); pi == c.i {
		if err != nil {
			c.i = hashtbl_invalidPage
		} else {
			c.p.writeRecord(ri, rec)
		}
	}

	return Error.Wrap(err)
}

//
// write-back hashtbl big page cache
//

type rwBigPageCache struct {
	fh *os.File
	i  bigPageIdxT
	p  bigPage
}

func (c *rwBigPageCache) Init(fh *os.File) {
	c.fh = fh
	c.i = hashtbl_invalidPage
}

func (c *rwBigPageCache) setPage(pi bigPageIdxT) (err error) {
	if c.i == pi {
		return nil
	} else if err := c.Flush(); err != nil {
		return Error.Wrap(err)
	}
	c.i = hashtbl_invalidPage // invalidate the page in case the read fails
	if _, err := c.fh.ReadAt(c.p[:], pi.Offset()); err != nil {
		return Error.Wrap(err)
	}
	c.i = pi
	return nil
}

func (c *rwBigPageCache) ReadRecord(slot slotIdxT, rec *Record) (valid bool, err error) {
	pi, ri := slot.BigPageIndexes()
	if err := c.setPage(pi); err != nil {
		return false, Error.Wrap(err)
	}
	return c.p.readRecord(ri, rec), nil
}

func (c *rwBigPageCache) WriteRecord(slot slotIdxT, rec *Record) (err error) {
	pi, ri := slot.BigPageIndexes()
	if err := c.setPage(pi); err != nil {
		return Error.Wrap(err)
	}
	c.p.writeRecord(ri, rec)
	return nil
}

func (c *rwBigPageCache) Flush() (err error) {
	if c.i == hashtbl_invalidPage {
		return nil
	}
	_, err = c.fh.WriteAt(c.p[:], c.i.Offset())
	return Error.Wrap(err)
}
