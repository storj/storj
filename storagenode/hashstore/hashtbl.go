// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"encoding/binary"
	"math/bits"
	"os"
	"sync"

	"github.com/zeebo/mwc"
	"github.com/zeebo/xxh3"

	"storj.io/drpc/drpcsignal"
)

const invalidPage = 1<<64 - 1

type hashtblHeader struct {
	created uint32 // when the hashtbl was created
	hashKey bool   // if we apply a hash function to the key
}

// HashTbl is an on disk hash table of records.
type HashTbl struct {
	fh       *os.File      // file handle backing the hashtbl
	logSlots uint64        // log_2 of the maximum number of slots
	numSlots slotIdxT      // 1 << logSlots, the actual maximum number of slots
	slotMask slotIdxT      // numSlots - 1, a bit mask for the maximum number of slots
	header   hashtblHeader // header information

	closed drpcsignal.Signal // closed state
	cloMu  sync.Mutex        // synchronizes closing

	opMu sync.RWMutex // protects operations

	buffer *rwBigPageCache // buffer for inserts

	mu       sync.Mutex // protects the following fields
	numSet   uint64     // estimated number of set records
	lenSet   uint64     // sum of lengths in set records
	numTrash uint64     // estimated number of set trash records
	lenTrash uint64     // sum of lengths in set trash records
}

// hashtblSize returns the size in bytes of the hashtbl given an logSlots.
func hashtblSize(logSlots uint64) uint64 { return pageSize + 1<<logSlots*RecordSize }

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

func (s slotIdxT) Offset() int64    { return pageSize + int64(s*RecordSize) }
func (p pageIdxT) Offset() int64    { return pageSize + int64(p*pageSize) }
func (p bigPageIdxT) Offset() int64 { return pageSize + int64(p*bigPageSize) }

// CreateHashtbl allocates a new hash table with the given log base 2 number of records and created
// timestamp. The file is truncated and allocated to the correct size.
func CreateHashtbl(fh *os.File, logSlots uint64, created uint32) (*HashTbl, error) {
	const maxLogSlots = 56
	const _ int64 = pageSize + 1<<maxLogSlots*RecordSize // compiler error if overflows int64
	if logSlots > maxLogSlots {
		return nil, Error.New("logSlots too large: logSlots=%d", logSlots)
	}

	header := hashtblHeader{
		created: created,
		hashKey: true,
	}

	// clear the file and truncate it to the correct length and write the header page.
	size := int64(hashtblSize(logSlots))
	if size < pageSize+bigPageSize {
		return nil, Error.New("hashtbl size too small: size=%d logSlots=%d", size, logSlots)
	} else if err := fh.Truncate(0); err != nil {
		return nil, Error.New("unable to truncate hashtbl to 0: %w", err)
	} else if err := fh.Truncate(size); err != nil {
		return nil, Error.New("unable to truncate hashtbl to %d: %w", size, err)
	} else if err := fallocate(fh, size); err != nil {
		return nil, Error.New("unable to fallocate hashtbl to %d: %w", size, err)
	} else if err := writeHashtblHeader(fh, header); err != nil {
		return nil, Error.Wrap(err)
	}

	// this is a bit wasteful in the sense that we will do some stat calls, reread the header page,
	// and compute estimates, but it reduces code paths and is not that expensive overall.
	return OpenHashtbl(fh)
}

// OpenHashtbl opens an existing hash table stored in the given file handle.
func OpenHashtbl(fh *os.File) (_ *HashTbl, err error) {
	// compute the number of records from the file size of the hash table.
	size, err := fileSize(fh)
	if err != nil {
		return nil, Error.New("unable to determine hashtbl size: %w", err)
	} else if size < pageSize+pageSize { // header page + at least 1 page of records
		return nil, Error.New("hashtbl file too small: size=%d", size)
	}

	// compute the logSlots from the size.
	logSlots := uint64(bits.Len64(uint64(size-pageSize)/RecordSize) - 1)

	// sanity check that our logSlots is correct.
	if int64(hashtblSize(logSlots)) != size {
		return nil, Error.New("logSlots calculation mismatch: size=%d logSlots=%d", size, logSlots)
	}

	// read the header information from the first page.
	header, err := readHashtblHeader(fh)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	h := &HashTbl{
		fh:       fh,
		logSlots: logSlots,
		numSlots: 1 << logSlots,
		slotMask: 1<<logSlots - 1,
		header:   header,
	}

	// estimate numSet, lenSet, numTrash and lenTrash.
	if err := h.computeEstimates(); err != nil {
		return nil, Error.Wrap(err)
	}

	return h, nil
}

// HashTblStats contains statistics about the hash table.
type HashTblStats struct {
	NumSet uint64  // number of set records.
	LenSet uint64  // sum of lengths in set records.
	AvgSet float64 // average size of length of records.

	NumTrash uint64  // number of set trash records.
	LenTrash uint64  // sum of lengths in set trash records.
	AvgTrash float64 // average size of length of trash records.

	NumSlots  uint64  // total number of records available.
	TableSize uint64  // total number of bytes in the hash table.
	Load      float64 // percent of slots that are set.

	Created uint32 // date that the hashtbl was created.
}

// Stats returns a HashTblStats about the hash table.
func (h *HashTbl) Stats() HashTblStats {
	h.mu.Lock()
	defer h.mu.Unlock()

	return HashTblStats{
		NumSet: h.numSet,
		LenSet: h.lenSet,
		AvgSet: safeDivide(float64(h.lenSet), float64(h.numSet)),

		NumTrash: h.numTrash,
		LenTrash: h.lenTrash,
		AvgTrash: safeDivide(float64(h.lenTrash), float64(h.numTrash)),

		NumSlots:  uint64(h.numSlots),
		TableSize: hashtblSize(h.logSlots),
		Load:      safeDivide(float64(h.numSet), float64(h.numSlots)),

		Created: h.header.created,
	}
}

// Close closes the hash table and returns when no more operations are running.
func (h *HashTbl) Close() {
	h.cloMu.Lock()
	defer h.cloMu.Unlock()

	if !h.closed.Set(Error.New("hashtbl closed")) {
		return
	}

	// grab the lock to ensure all operations have finished before closing the file handle.
	h.opMu.Lock()
	defer h.opMu.Unlock()

	_ = h.fh.Close()
}

// writeHashtblHeader writes the header page to the file handle.
func writeHashtblHeader(fh *os.File, header hashtblHeader) error {
	var buf [pageSize]byte

	copy(buf[0:4], "HTBL")
	binary.BigEndian.PutUint32(buf[4:8], header.created) // write the created field.
	if header.hashKey {
		buf[8] = 1 // write the hashKey field.
	} else {
		buf[8] = 0
	}

	// write the checksum
	binary.BigEndian.PutUint64(buf[pageSize-8:pageSize], xxh3.Hash(buf[:pageSize-8]))

	// write the header page.
	_, err := fh.WriteAt(buf[:], 0)
	return Error.Wrap(err)
}

// readHashtblHeader reads the header page from the file handle.
func readHashtblHeader(fh *os.File) (header hashtblHeader, err error) {
	// read the magic bytes.
	var buf [pageSize]byte
	if _, err := fh.ReadAt(buf[:], 0); err != nil {
		return hashtblHeader{}, Error.New("unable to read header: %w", err)
	} else if string(buf[0:4]) != "HTBL" {
		return hashtblHeader{}, Error.New("invalid header: %q", buf[0:4])
	}

	// check the checksum.
	hash := binary.BigEndian.Uint64(buf[pageSize-8 : pageSize])
	if computed := xxh3.Hash(buf[:pageSize-8]); hash != computed {
		return hashtblHeader{}, Error.New("invalid header checksum: %x != %x", hash, computed)
	}

	header.created = binary.BigEndian.Uint32(buf[4:8]) // read the created field.
	header.hashKey = buf[8] != 0                       // read the hashKey field.

	return header, nil
}

// slotForKey computes the slot for the given key.
func (h *HashTbl) slotForKey(k *Key) slotIdxT {
	var v uint64
	if h.header.hashKey {
		v = xxh3.Hash(k[:])
	} else {
		v = binary.BigEndian.Uint64(k[0:8])
	}
	s := (64 - h.logSlots) % 64
	return slotIdxT(v>>s) & h.slotMask
}

// computeEstimates samples the hash table to compute the number of set keys and the total length of
// the length fields in all of the set records.
func (h *HashTbl) computeEstimates() (err error) {
	defer mon.Task()(nil)(&err)

	h.opMu.RLock()
	defer h.opMu.RUnlock()

	// sample some pages worth of records but less than the total
	maxPages := uint64(h.numSlots / recordsPerPage)
	samplePages := uint64(256)
	if samplePages > maxPages {
		samplePages = maxPages
	}

	var (
		numSet, lenSet     uint64
		numTrash, lenTrash uint64

		rng = mwc.Rand()
	)

	var cache roPageCache
	cache.Init(h.fh)

	for i := uint64(0); i < samplePages; i++ {
		pageIdx := rng.Uint64n(maxPages)
		for recIdx := uint64(0); recIdx < recordsPerPage; recIdx++ {
			slot := slotIdxT(pageIdx*recordsPerPage + recIdx)
			rec, valid, err := cache.ReadRecord(slot)
			if err != nil {
				return Error.Wrap(err)
			} else if valid {
				numSet++
				lenSet += uint64(rec.Length)

				if rec.Expires.Trash() {
					numTrash++
					lenTrash += uint64(rec.Length)
				}
			}
		}
	}

	// scale the number found by the number of total pages divided by the number of sampled
	// pages. because the hashtbl is always a power of 2 number of pages, we know that
	// this evenly divides.
	factor := maxPages / samplePages

	numSet *= factor
	lenSet *= factor
	numTrash *= factor
	lenTrash *= factor

	h.mu.Lock()
	h.numSet, h.lenSet = numSet, lenSet
	h.numTrash, h.lenTrash = numTrash, lenTrash
	h.mu.Unlock()

	return nil
}

// Load returns an estimate of what fraction of the hash table is occupied.
func (h *HashTbl) Load() float64 {
	h.mu.Lock()
	defer h.mu.Unlock()

	return float64(h.numSet) / float64(h.numSlots)
}

// Range iterates over the records in hash table order.
func (h *HashTbl) Range(fn func(Record, error) bool) {
	h.opMu.RLock()
	defer h.opMu.RUnlock()

	if err := signalError(&h.closed); err != nil {
		fn(Record{}, err)
		return
	}

	var cache roBigPageCache
	cache.Init(h.fh)

	for slot := slotIdxT(0); slot < h.numSlots; slot++ {
		rec, valid, err := cache.ReadRecord(slot)
		if err != nil {
			fn(Record{}, Error.Wrap(err))
			return
		} else if valid {
			if !fn(rec, nil) {
				return
			}
		}
	}
}

// ExpectOrdered signals that incoming writes to the hashtbl will be ordered so that a large shared
// buffer across Insert calls would be effective. This is useful when rewriting a hashtbl during a
// Compaction, for instance. It returns a flush callback that both flushes any potentially buffered
// records and disables the expectation. Additionally, Lookups may not find entries written until
// after the flush callback is called. If flush returns an error there is no guarantee about what
// records were written. It returns a done callback that discards any potentially buffered records
// and disables the expectation. At least one of flush or done must be called. It returns an error
// if called again before flush or done is called.
func (h *HashTbl) ExpectOrdered() (flush func() error, done func(), err error) {
	h.opMu.Lock()
	defer h.opMu.Unlock()

	if h.buffer != nil {
		return nil, nil, Error.New("buffer already exists")
	}

	buffer := new(rwBigPageCache)
	h.buffer = buffer
	h.buffer.Init(h.fh)

	return func() (err error) {
			h.opMu.Lock()
			defer h.opMu.Unlock()

			if h.buffer == buffer {
				err = h.buffer.Flush()
				h.buffer = nil
			}

			return Error.Wrap(err)
		}, func() {
			h.opMu.Lock()
			defer h.opMu.Unlock()

			if h.buffer == buffer {
				h.buffer = nil
			}
		}, nil
}

// Insert adds a record to the hash table. It returns (true, nil) if the record was inserted, it
// returns (false, nil) if the hash table is full, and (false, err) if any errors happened trying
// to insert the record.
func (h *HashTbl) Insert(rec Record) (_ bool, err error) {
	h.opMu.Lock()
	defer h.opMu.Unlock()

	if err := signalError(&h.closed); err != nil {
		return false, err
	}

	var cache rwPageCache
	cache.Init(h.fh)

	for slot, attempt := h.slotForKey(&rec.Key), slotIdxT(0); attempt < h.numSlots; slot, attempt = (slot+1)&h.slotMask, attempt+1 {
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

		var (
			tmp   Record
			valid bool
			err   error
		)

		if h.buffer != nil {
			tmp, valid, err = h.buffer.ReadRecord(slot)
		} else {
			tmp, valid, err = cache.ReadRecord(slot)
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
		}

		// thus it is either invalid or the key matches and the record is updated, so we can write.
		if h.buffer != nil {
			err = h.buffer.WriteRecord(slot, rec)
		} else {
			err = cache.WriteRecord(slot, rec)
		}
		if err != nil {
			return false, Error.Wrap(err)
		}

		// if the slot was invalid, we are adding a new key. we don't need to change the alive field
		// on update because we ensure that the records are equalish above so the length field could
		// not have changed. we're ignoring the update case for trash because it should be very rare
		// and doing it properly would require subtracting which may underflow in situations where
		// the estimate was too small. this technically means that in very rare scenarios, the
		// amount considered trash could be off, but it will be fixed on the next Range call, Store
		// compaction, or node restart.
		h.mu.Lock()
		if !valid {
			h.numSet++
			h.lenSet += uint64(rec.Length)

			if rec.Expires.Trash() {
				h.numTrash++
				h.lenTrash += uint64(rec.Length)
			}
		}
		h.mu.Unlock()

		return true, nil
	}

	return false, nil
}

// Lookup returns the record for the given key if it exists in the hash table. It returns (rec,
// true, nil) if the record existed, (rec{}, false, nil) if it did not exist, and (rec{}, false,
// err) if any errors happened trying to look up the record.
func (h *HashTbl) Lookup(key Key) (_ Record, _ bool, err error) {
	h.opMu.RLock()
	defer h.opMu.RUnlock()

	if err := signalError(&h.closed); err != nil {
		return Record{}, false, err
	}

	var cache roPageCache
	cache.Init(h.fh)

	for slot, attempt := h.slotForKey(&key), slotIdxT(0); attempt < h.numSlots; slot, attempt = (slot+1)&h.slotMask, attempt+1 {
		rec, valid, err := cache.ReadRecord(slot)
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
	c.i = invalidPage
}

func (c *roPageCache) ReadRecord(slot slotIdxT) (rec Record, valid bool, err error) {
	pi, ri := slot.PageIndexes()
	if pi != c.i {
		c.i = invalidPage // invalidate the page in case the read fails
		if _, err := c.fh.ReadAt(c.p[:], pi.Offset()); err != nil {
			return Record{}, false, Error.Wrap(err)
		}
		c.i = pi
	}
	valid = c.p.readRecord(ri, &rec)
	return rec, valid, nil
}

type roBigPageCache struct {
	fh *os.File
	i  bigPageIdxT
	p  bigPage
}

func (c *roBigPageCache) Init(fh *os.File) {
	c.fh = fh
	c.i = invalidPage
}

func (c *roBigPageCache) ReadRecord(slot slotIdxT) (rec Record, valid bool, err error) {
	pi, ri := slot.BigPageIndexes()
	if pi != c.i {
		c.i = invalidPage // invalidate the page in case the read fails
		if _, err := c.fh.ReadAt(c.p[:], pi.Offset()); err != nil {
			return Record{}, false, Error.Wrap(err)
		}
		c.i = pi
	}
	valid = c.p.readRecord(ri, &rec)
	return rec, valid, nil
}

//
// write-back hashtbl page caches
//

type rwPageCache struct {
	fh *os.File
	i  pageIdxT
	p  page
}

func (c *rwPageCache) Init(fh *os.File) {
	c.fh = fh
	c.i = invalidPage
}

func (c *rwPageCache) setPage(pi pageIdxT) (err error) {
	if c.i == pi {
		return nil
	}
	c.i = invalidPage // invalidate the page in case the read fails
	if _, err := c.fh.ReadAt(c.p[:], pi.Offset()); err != nil {
		return Error.Wrap(err)
	}
	c.i = pi
	return nil
}

func (c *rwPageCache) ReadRecord(slot slotIdxT) (rec Record, valid bool, err error) {
	pi, ri := slot.PageIndexes()
	if err := c.setPage(pi); err != nil {
		return Record{}, false, Error.Wrap(err)
	}
	valid = c.p.readRecord(ri, &rec)
	return rec, valid, nil
}

func (c *rwPageCache) WriteRecord(slot slotIdxT, rec Record) (err error) {
	// directly write the record because this page cache is used in situations where we expect only
	// a single record to be written.
	var buf [RecordSize]byte
	rec.WriteTo(&buf)
	_, err = c.fh.WriteAt(buf[:], slot.Offset())

	// update or invalidate our in memory page
	if pi, ri := slot.PageIndexes(); pi == c.i {
		if err != nil {
			c.i = invalidPage
		} else {
			c.p.writeRecord(ri, rec)
		}
	}

	return Error.Wrap(err)
}

type rwBigPageCache struct {
	fh *os.File
	i  bigPageIdxT
	p  bigPage
}

func (c *rwBigPageCache) Init(fh *os.File) {
	c.fh = fh
	c.i = invalidPage
}

func (c *rwBigPageCache) setPage(pi bigPageIdxT) (err error) {
	if c.i == pi {
		return nil
	} else if err := c.Flush(); err != nil {
		return Error.Wrap(err)
	}
	c.i = invalidPage // invalidate the page in case the read fails
	if _, err := c.fh.ReadAt(c.p[:], pi.Offset()); err != nil {
		return Error.Wrap(err)
	}
	c.i = pi
	return nil
}

func (c *rwBigPageCache) ReadRecord(slot slotIdxT) (rec Record, valid bool, err error) {
	pi, ri := slot.BigPageIndexes()
	if err := c.setPage(pi); err != nil {
		return Record{}, false, Error.Wrap(err)
	}
	valid = c.p.readRecord(ri, &rec)
	return rec, valid, nil
}

func (c *rwBigPageCache) WriteRecord(slot slotIdxT, rec Record) (err error) {
	pi, ri := slot.BigPageIndexes()
	if err := c.setPage(pi); err != nil {
		return Error.Wrap(err)
	}
	c.p.writeRecord(ri, rec)
	return nil
}

func (c *rwBigPageCache) Flush() (err error) {
	if c.i == invalidPage {
		return nil
	}
	_, err = c.fh.WriteAt(c.p[:], c.i.Offset())
	return Error.Wrap(err)
}
