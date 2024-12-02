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

const invalidPage = ^uint64(0)

type hashtblHeader struct {
	created uint32 // when the hashtbl was created
	hashKey bool   // if we apply a hash function to the key
}

// HashTbl is an on disk hash table of records.
type HashTbl struct {
	fh       *os.File      // file handle backing the hashtbl
	logSlots uint64        // log_2 of the maximum number of slots
	numSlots uint64        // 1 << logSlots, the actual maximum number of slots
	slotMask uint64        // numSlots - 1, a bit mask for the maximum number of slots
	header   hashtblHeader // header information

	closed drpcsignal.Signal // closed state
	cloMu  sync.Mutex        // synchronizes closing

	opMu sync.RWMutex // protects operations

	mu       sync.Mutex // protects the following fields
	numSet   uint64     // estimated number of set records
	lenSet   uint64     // sum of lengths in set records
	numTrash uint64     // estimated number of set trash records
	lenTrash uint64     // sum of lengths in set trash records
}

// hashtblSize returns the size in bytes of the hashtbl given an logSlots.
func hashtblSize(logSlots uint64) uint64 { return pageSize + 1<<logSlots*RecordSize }

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

		NumSlots:  h.numSlots,
		TableSize: pageSize + h.numSlots*RecordSize,
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
func (h *HashTbl) slotForKey(k *Key) uint64 {
	var v uint64
	if h.header.hashKey {
		v = xxh3.Hash(k[:])
	} else {
		v = binary.BigEndian.Uint64(k[0:8])
	}
	s := (64 - h.logSlots) % 64
	return (v >> s) & h.slotMask
}

// pageAndRecordIndexForSlot computes the page and record index for the slot'th slot.
func (h *HashTbl) pageAndRecordIndexForSlot(slot uint64) (pageIdx uint64, recIdx uint64) {
	return slot / recordsPerPage, slot % recordsPerPage
}

// bigPageAndRecordIndexForSlot computes the page and record index for the slot'th slot.
func (h *HashTbl) bigPageAndRecordIndexForSlot(slot uint64) (pageIdx uint64, recIdx uint64) {
	return slot / recordsPerBigPage, slot % recordsPerBigPage
}

// readPage reads the page at pageIndex into p.
func (h *HashTbl) readPage(pageIndex uint64, p *page) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	offset := pageSize + int64(pageIndex*pageSize) // add pageSize to skip header page
	_, err := h.fh.ReadAt(p[:], offset)
	return Error.Wrap(err)
}

// readBigPage reads the bigPage at pageIndex into p.
func (h *HashTbl) readBigPage(pageIndex uint64, p *bigPage) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	offset := pageSize + int64(pageIndex*bigPageSize) // add pageSize to skip header page
	_, err := h.fh.ReadAt(p[:], offset)
	return Error.Wrap(err)
}

// writeRecord writes rec into the slot'th slot.
func (h *HashTbl) writeRecord(slot uint64, rec Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	var buf [RecordSize]byte
	rec.WriteTo(&buf)

	offset := pageSize + int64(slot*RecordSize) // add pageSize to skip header page
	_, err := h.fh.WriteAt(buf[:], offset)

	return Error.Wrap(err)
}

// computeEstimates samples the hash table to compute the number of set keys and the total length of
// the length fields in all of the set records.
func (h *HashTbl) computeEstimates() (err error) {
	defer mon.Task()(nil)(&err)

	h.opMu.RLock()
	defer h.opMu.RUnlock()

	// sample some pages worth of records but less than the total
	maxPages := h.numSlots / recordsPerPage
	samplePages := uint64(256)
	if samplePages > maxPages {
		samplePages = maxPages
	}

	var (
		numSet, lenSet     uint64
		numTrash, lenTrash uint64

		p   page
		rng = mwc.Rand()
	)

	for i := uint64(0); i < samplePages; i++ {
		pageIdx := rng.Uint64n(maxPages)
		if err := h.readPage(pageIdx, &p); err != nil {
			return err
		}

		for recIdx := uint64(0); recIdx < recordsPerPage; recIdx++ {
			var tmp Record
			if p.readRecord(recIdx, &tmp) {
				numSet++
				lenSet += uint64(tmp.Length)

				if tmp.Expires.Trash() {
					numTrash++
					lenTrash += uint64(tmp.Length)
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

var rangeTask = mon.Task()

// Range iterates over the records in hash table order.
func (h *HashTbl) Range(fn func(Record, error) bool) {
	defer rangeTask(nil)(nil)

	h.opMu.RLock()
	defer h.opMu.RUnlock()

	if err := h.closed.Err(); err != nil {
		fn(Record{}, err)
		return
	}

	var (
		tmp           Record
		cachedPage    bigPage
		cachedPageIdx = invalidPage
	)

	for slot := uint64(0); slot < h.numSlots; slot++ {
		pageIdx, recIdx := h.bigPageAndRecordIndexForSlot(slot)
		if pageIdx != cachedPageIdx {
			if err := h.readBigPage(pageIdx, &cachedPage); err != nil {
				fn(Record{}, err)
				return
			}
			cachedPageIdx = pageIdx
		}

		if cachedPage.readRecord(recIdx, &tmp) {
			if !fn(tmp, nil) {
				return
			}
		}
	}
}

var insertTask = mon.Task()

// Insert adds a record to the hash table. It returns (true, nil) if the record was inserted, it
// returns (false, nil) if the hash table is full, and (false, err) if any errors happened trying
// to insert the record.
func (h *HashTbl) Insert(rec Record) (_ bool, err error) {
	defer insertTask(nil)(&err)

	h.opMu.Lock()
	defer h.opMu.Unlock()

	if err := h.closed.Err(); err != nil {
		return false, err
	}

	var (
		tmp           Record
		cachedPage    page
		cachedPageIdx = invalidPage
	)

	for slot, attempt := h.slotForKey(&rec.Key), uint64(0); attempt < h.numSlots; slot, attempt = (slot+1)&h.slotMask, attempt+1 {
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

		pageIdx, recIdx := h.pageAndRecordIndexForSlot(slot)
		if pageIdx != cachedPageIdx {
			if err := h.readPage(pageIdx, &cachedPage); err != nil {
				return false, Error.Wrap(err)
			}
			cachedPageIdx = pageIdx
		}
		valid := cachedPage.readRecord(recIdx, &tmp)

		// if we have a valid record, we need to do some checks.
		if valid {
			// if it is some other key, the slot is occupied and we need to probe further.
			if tmp.Key != rec.Key {
				continue
			}

			// otherwise, it is our key, and as noted above we need to merge the records, erroring
			// if fields are mutated, and picking the "larger" expiration time.
			if !RecordsEqualish(rec, tmp) {
				return false, Error.New("collision detected: put:%v != exist:%v", rec, tmp)
			}

			rec.Expires = MaxExpiration(rec.Expires, tmp.Expires)
		}

		// thus it is either invalid or the key matches and the record is updated, so we can write.
		if err := h.writeRecord(slot, rec); err != nil {
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

var lookupTask = mon.Task()

// Lookup returns the record for the given key if it exists in the hash table. It returns (rec,
// true, nil) if the record existed, (rec{}, false, nil) if it did not exist, and (rec{}, false,
// err) if any errors happened trying to look up the record.
func (h *HashTbl) Lookup(key Key) (_ Record, _ bool, err error) {
	defer lookupTask(nil)(&err)

	h.opMu.RLock()
	defer h.opMu.RUnlock()

	if err := h.closed.Err(); err != nil {
		return Record{}, false, err
	}

	var (
		tmp           Record
		cachedPage    page
		cachedPageIdx = invalidPage
	)

	for slot, attempt := h.slotForKey(&key), uint64(0); attempt < h.numSlots; slot, attempt = (slot+1)&h.slotMask, attempt+1 {
		pageIdx, recIdx := h.pageAndRecordIndexForSlot(slot)
		if pageIdx != cachedPageIdx {
			if err := h.readPage(pageIdx, &cachedPage); err != nil {
				return Record{}, false, Error.Wrap(err)
			}
			cachedPageIdx = pageIdx
		}
		valid := cachedPage.readRecord(recIdx, &tmp)

		if !valid {
			// even if the record is invalid, keep looking for up to 2 pages. this causes us more
			// reads when looking up a key that does not exist, but helps us find keys that maybe do
			// exist if a page write was lost. fortunately, we often do not get queried for keys
			// that do not exist, so this should not be expensive.
			if attempt < 2*recordsPerPage {
				continue
			}

			return Record{}, false, nil
		} else if tmp.Key == key {
			return tmp, true, nil
		}
	}

	return Record{}, false, nil
}
