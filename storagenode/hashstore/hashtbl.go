// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"encoding/binary"
	"math/bits"
	"os"
	"sync"

	"github.com/zeebo/errs"
	"github.com/zeebo/xxh3"

	"storj.io/drpc/drpcsignal"
)

// HashTbl is an on disk hash table of records.
type HashTbl struct {
	fh      *os.File
	lrec    uint64 // log_2 of number of records
	nrec    uint64 // 1 << lrec
	mask    uint64 // nrec - 1
	created uint32

	closed drpcsignal.Signal // closed state
	cloMu  sync.Mutex        // synchronizes closing

	opMu sync.RWMutex // protects operations

	mu    sync.Mutex // protects the following fields
	nset  uint64     // estimated number of set records
	alive uint64     // sum of lengths in set records
	pi    uint64     // index of cached page
	p     page       // cached page data
}

// CreateHashtbl allocates a new hash table with the given log base 2 number of records and created
// timestamp. The file is truncated and allocated to the correct size.
func CreateHashtbl(fh *os.File, lrec uint64, created uint32) (*HashTbl, error) {
	const maxLRec = 56
	const _ int64 = pSize + 1<<maxLRec*RSize // compiler error if overflows int64
	if lrec > maxLRec {
		return nil, errs.New("lrec too large: %d", lrec)
	}

	// clear the file and truncate it to the correct length and write the header page.
	size := int64(pSize + 1<<lrec*RSize)
	if err := fh.Truncate(0); err != nil {
		return nil, errs.New("unable to truncate hashtbl to 0: %w", err)
	} else if err := fh.Truncate(size); err != nil {
		return nil, errs.New("unable to truncate hashtbl to %d: %w", size, err)
	} else if err := fallocate(fh, size); err != nil {
		return nil, errs.New("unable to fallocate hashtbl to %d: %w", size, err)
	} else if err := writeHashtblHeader(fh, created); err != nil {
		return nil, errs.Wrap(err)
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
		return nil, errs.New("unable to determine hashtbl size: %w", err)
	} else if size < pSize+pSize { // header page + at least 1 page of records
		return nil, errs.New("hashtbl file too small: size=%d", size)
	}

	// compute the lrec from the size.
	lrec := uint64(bits.Len64(uint64(size-pSize)/RSize) - 1)

	// sanity check that our lrec is correct.
	if pSize+1<<lrec*RSize != size {
		return nil, errs.New("lrec calculation mismatch: size=%d lrec=%d", size, lrec)
	}

	// read the header information from the first page.
	created, err := readHashtblHeader(fh)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	h := &HashTbl{
		fh:      fh,
		lrec:    lrec,
		nrec:    1 << lrec,
		mask:    1<<lrec - 1,
		pi:      ^uint64(0),
		created: created,
	}

	// estimate nset and alive.
	h.nset, h.alive, err = h.computeEstimates()
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return h, nil
}

// Close closes the hash table and returns when no more operations are running.
func (h *HashTbl) Close() {
	h.cloMu.Lock()
	defer h.cloMu.Unlock()

	if !h.closed.Set(errs.New("hashtbl closed")) {
		return
	}

	// grab the lock to ensure all operations have finished before closing the file handle.
	h.opMu.Lock()
	defer h.opMu.Unlock()

	_ = h.fh.Close()
}

// writeHashtblHeader writes the header page to the file handle.
func writeHashtblHeader(fh *os.File, created uint32) error {
	var buf [pSize]byte

	copy(buf[0:4], "HTBL")
	binary.BigEndian.PutUint32(buf[4:8], created)
	binary.BigEndian.PutUint64(buf[pSize-8:pSize], xxh3.Hash(buf[:pSize-8]))

	// write the header page.
	_, err := fh.WriteAt(buf[:], 0)
	return errs.Wrap(err)
}

// readHashtblHeader reads the header page from the file handle.
func readHashtblHeader(fh *os.File) (created uint32, err error) {
	// read the magic bytes.
	var buf [pSize]byte
	if _, err := fh.ReadAt(buf[:], 0); err != nil {
		return 0, errs.New("unable to read header: %w", err)
	} else if string(buf[0:4]) != "HTBL" {
		return 0, errs.New("invalid header: %q", buf[0:4])
	}

	// check the checksum.
	hash := binary.BigEndian.Uint64(buf[pSize-8 : pSize])
	if computed := xxh3.Hash(buf[:pSize-8]); hash != computed {
		return 0, errs.New("invalid header checksum: %x != %x", hash, computed)
	}

	// read the created field.
	return binary.BigEndian.Uint32(buf[4:8]), nil
}

// keyIndex computes the record number for the given key.
func (h *HashTbl) keyIndex(k *Key) uint64 {
	v := binary.BigEndian.Uint64(k[0:8])
	s := (64 - h.lrec) % 64
	return (v >> s) & h.mask
}

// pageIndex computes the page and record index for the nth record.
func (h *HashTbl) pageIndex(n uint64) (pi uint64, ri uint64) {
	return n / rPerP, n % rPerP
}

// invalidatePageCache invalidates which page is currently cached in memory.
func (h *HashTbl) invalidatePageCache() {
	h.pi = ^uint64(0)
}

// readPageLocked ensures that the pi'th page is cached in memory.
func (h *HashTbl) readPageLocked(pi uint64) error {
	if pi == h.pi {
		return nil
	}
	h.invalidatePageCache()           // invalidate the current page in case of errors
	offset := pSize + int64(pi*pSize) // add pSize to skip header page
	if _, err := h.fh.ReadAt(h.p[:], offset); err != nil {
		return errs.New("unable to read page=%d off=%d: %w", pi, pi*pSize, err)
	}
	h.pi = pi // no errors so the page is fully read correctly
	return nil
}

// readRecord reads the nth slot into the record pointed at by rec.
func (h *HashTbl) readRecord(n uint64, rec *Record) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	pi, ri := h.pageIndex(n)
	if err := h.readPageLocked(pi); err != nil {
		return false, errs.Wrap(err)
	}
	return h.p.readRecord(ri, rec), nil
}

// writeRecord writes rec into the nth slot.
func (h *HashTbl) writeRecord(n uint64, rec Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	pi, ri := h.pageIndex(n)

	var buf [RSize]byte
	rec.Write(&buf)

	offset := pSize + int64(n*RSize) // add pSize to skip header page
	_, err := h.fh.WriteAt(buf[:], offset)

	if pi == h.pi {
		// update our cached page depending on the results of the write.
		if err == nil {
			// update the page in memory.
			h.p.writeRecord(ri, rec)
		} else {
			// we don't know what state the current page is, so invalidate it.
			h.invalidatePageCache()
		}
	}

	return errs.Wrap(err)
}

// computeEstimates samples the hash table to compute the number of set keys and the total length of
// the length fields in all of the set records.
func (h *HashTbl) computeEstimates() (nset uint64, length uint64, err error) {
	// sample some pages worth of records but less than the total
	srec := uint64(rPerP) * 256
	if srec > h.nrec {
		srec = h.nrec
	}

	var tmp Record
	for ri := uint64(0); ri < srec; ri++ {
		if ok, err := h.readRecord(ri, &tmp); err != nil {
			return 0, 0, err
		} else if ok {
			nset++
			length += uint64(tmp.Length)
		}
	}

	// scale the number found by the number of total records divided by the number of sampled
	// records. because the hashtbl is always a power of 2 number of records, we know that
	// this evenly divides.
	factor := h.nrec / srec
	return nset * factor, length * factor, nil
}

// Estimates returns the estimated values for the number of set keys and the total length of the
// length fields in all of the set records.
func (h *HashTbl) Estimates() (nset, alive uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.nset, h.alive
}

// Load returns an estimate of what fraction of the hash table is occupied.
func (h *HashTbl) Load() float64 {
	h.mu.Lock()
	defer h.mu.Unlock()

	return float64(h.nset) / float64(h.nrec)
}

// Range iterates over the records in hash table order.
func (h *HashTbl) Range(fn func(Record, error) bool) {
	h.opMu.RLock()
	defer h.opMu.RUnlock()

	if err := h.closed.Err(); err != nil {
		fn(Record{}, err)
		return
	}

	var tmp Record
	var nset, length uint64
	for n := uint64(0); n < h.nrec; n++ {
		if ok, err := h.readRecord(n, &tmp); err != nil {
			fn(Record{}, err)
			return
		} else if !ok {
			continue
		}

		nset++
		length += uint64(tmp.Length)

		if !fn(tmp, nil) {
			return
		}
	}

	// if we read the whole thing, then we have accurate estimates, so update.
	h.mu.Lock()
	h.nset, h.alive = nset, length
	h.mu.Unlock()
}

// Insert adds a record to the hash table. It returns (true, nil) if the record was inserted, it
// returns (false, nil) if the hash table is full, and (false, err) if any errors happened trying
// to insert the record.
func (h *HashTbl) Insert(rec Record) (_ bool, err error) {
	h.opMu.Lock()
	defer h.opMu.Unlock()

	if err := h.closed.Err(); err != nil {
		return false, err
	}

	var tmp Record

	for n, i := h.keyIndex(&rec.Key), uint64(0); i < h.nrec; n, i = (n+1)&h.mask, i+1 {
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

		valid, err := h.readRecord(n, &tmp)
		if err != nil {
			return false, errs.Wrap(err)
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
				return false, errs.New("collision detected: put:%v != exist:%v", rec, tmp)
			}

			rec.Expires = MaxExpiration(rec.Expires, tmp.Expires)
		}

		// thus it is either invalid or the key matches and the record is updated, so we can write.
		if err := h.writeRecord(n, rec); err != nil {
			return false, errs.Wrap(err)
		}

		// if the slot was invalid, we are adding a new key. we don't need to change the alive field
		// on update because we ensure that the records are equalish above so the length field could
		// not have changed.
		h.mu.Lock()
		if !valid {
			h.nset++
			h.alive += uint64(rec.Length)
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

	if err := h.closed.Err(); err != nil {
		return Record{}, false, err
	}

	var tmp Record

	for n, i := h.keyIndex(&key), uint64(0); i < h.nrec; n, i = (n+1)&h.mask, i+1 {
		if ok, err := h.readRecord(n, &tmp); err != nil {
			return Record{}, false, errs.Wrap(err)
		} else if !ok {
			// even if the record is invalid, keep looking for up to 2 pages. this causes us more
			// reads when looking up a key that does not exist, but helps us find keys that maybe do
			// exist if a page write was lost. fortunately, we often do not get queried for keys
			// that do not exist, so this should not be expensive.
			if i < 2*rPerP {
				continue
			}

			return Record{}, false, nil
		} else if tmp.Key == key {
			return tmp, true, nil
		}
	}

	return Record{}, false, nil
}
