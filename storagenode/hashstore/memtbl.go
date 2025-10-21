// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"sync"

	"github.com/zeebo/errs"

	"storj.io/common/memory"
	"storj.io/drpc/drpcsignal"
	"storj.io/storj/storagenode/hashstore/platform"
)

type memtblIdx uint32 // index of a record in the memtbl (^0 means promoted)

const memtbl_Promoted = ^memtblIdx(0)

func memtblIdxToValue(idx memtblIdx) (b [4]byte) {
	binary.LittleEndian.PutUint32(b[:], uint32(idx))
	return
}

func valueToMemTblIdx(b [4]byte) (idx memtblIdx) {
	return memtblIdx(binary.LittleEndian.Uint32(b[:]))
}

// MemTbl is an in-memory hash table of records with a file for persistence.
type MemTbl struct {
	fh     *os.File
	header TblHeader
	cfg    MmapCfg

	opMu  rwMutex   // protects operations
	idx   memtblIdx // insert index. always needs to match file length
	align bool      // set when an error happened and an align is needed

	entries    *flatMap
	collisions map[Key][4]byte

	mmap  []byte
	remap bool

	closed drpcsignal.Signal // closed state
	cloErr error             // close error
	cloMu  sync.Mutex        // synchronizes closing

	buffer []byte

	statsMu  sync.Mutex // protects the following fields
	recStats recordStats
}

// CreateMemTbl allocates a new mem table with the given log base 2 number of records and created
// timestamp.
func CreateMemTbl(ctx context.Context, fh *os.File, logSlots uint64, created uint32, cfg MmapCfg) (_ *MemTblConstructor, err error) {
	defer mon.Task()(&ctx)(&err)

	if logSlots > tbl_maxLogSlots {
		return nil, Error.New("logSlots too large: logSlots=%d", logSlots)
	} else if logSlots < tbl_minLogSlots {
		return nil, Error.New("logSlots too small: logSlots=%d", logSlots)
	}

	header := TblHeader{
		Created:  created,
		HashKey:  true,
		Kind:     TableKind_MemTbl,
		LogSlots: logSlots,
	}

	// clear the file and truncate it to the correct length and write the header page.
	if err := fh.Truncate(0); err != nil {
		return nil, Error.New("unable to truncate memtbl to 0: %w", err)
	} else if err := fh.Truncate(tbl_headerSize); err != nil {
		return nil, Error.New("unable to truncate memtbl to %d: %w", tbl_headerSize, err)
	} else if err := WriteTblHeader(fh, header); err != nil {
		return nil, Error.Wrap(err)
	}

	// this is a bit wasteful in the sense that we will do some stat calls, reread the header page,
	// but it reduces code paths and is not that expensive overall.
	m, _, err := OpenMemTbl(ctx, fh, cfg)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return newMemTblConstructor(m), nil
}

// OpenMemTbl opens an existing hash table stored in the given file handle.
func OpenMemTbl(ctx context.Context, fh *os.File, cfg MmapCfg) (_ *MemTbl, _ map[uint64]*RecordTail, err error) {
	defer mon.Task()(&ctx)(&err)

	// ensure the file is appropriately aligned and seek it to the end for writes.
	size, err := fh.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, nil, Error.New("unable to determine memtbl size: %w", err)
	} else if size < tbl_headerSize {
		return nil, nil, Error.New("memtbl size too small for header: size=%d", size)
	}

	// read the header information from the first page.
	header, err := ReadTblHeader(fh)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	} else if header.Kind != TableKind_MemTbl {
		return nil, nil, Error.New("invalid kind: %d", header.Kind)
	} else if header.LogSlots > tbl_maxLogSlots {
		return nil, nil, Error.New("logSlots too large: logSlots=%d", header.LogSlots)
	} else if header.LogSlots < tbl_minLogSlots {
		return nil, nil, Error.New("logSlots too small: logSlots=%d", header.LogSlots)
	}

	m := &MemTbl{
		fh:     fh,
		header: header,
		cfg:    cfg,

		idx:   memtblIdx((size - tbl_headerSize) / RecordSize),
		align: size%RecordSize != 0,

		entries:    newFlatMap(make([]byte, flatMapSize(1<<header.LogSlots))),
		collisions: make(map[Key][4]byte),
	}
	defer func() {
		if err != nil {
			m.fh = nil // don't close the file handle if we're returning an error
			_ = m.Close()
		}
	}()

	if err := m.ensureAlignedLocked(ctx); err != nil {
		return nil, nil, err
	}

	if cfg.Mmap && platform.MmapSupported {
		data, err := platform.Mmap(fh, int(size-size%platform.PageSize))
		if err != nil {
			return nil, nil, Error.Wrap(err)
		}
		m.mmap, m.remap = data, true

		if cfg.Mlock && platform.MmapSupported {
			_ = platform.Mlock(data)
		}

		// N.B. we don't bother with a memory advise here because loadEntries will be sequential and
		// defer setting it to random when it's done, so we don't want to confuse the kernel by
		// saying random => sequential => random.
	}

	// read the entries from the file and collect tails.
	tails, err := m.loadEntries(ctx)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	return m, tails, nil
}

// rangeWithIdxLocked reads the file handle calling the provided cb with all of the records that
// have a valid checksum along with their index in the file.
func (m *MemTbl) rangeWithIdxLocked(
	ctx context.Context,
	cb func(context.Context, memtblIdx, Record) (bool, error),
) (err error) {
	defer mon.Task()(&ctx)(&err)

	size, err := fileSize(m.fh)
	if err != nil {
		return Error.New("unable to determine file size: %w", err)
	} else if size < tbl_headerSize {
		return Error.New("file too small: size=%d", size)
	}

	// create the reader we use for the range. if we have mmap data, read from that first to avoid
	// any syscalls and inform the kernel we'll be doing sequential reads. after reading all of the
	// mmap data, go back to bufio.NewReader with an io.SectionReader so that we use ReadAt calls
	// and avoid modifying the file pos for writes.
	var r io.Reader
	if len(m.mmap) < tbl_headerSize {
		r = bufio.NewReaderSize(
			io.NewSectionReader(m.fh, tbl_headerSize, size-tbl_headerSize),
			1<<20,
		)
	} else {
		platform.AdviseSequential(m.mmap)
		defer platform.AdviseRandom(m.mmap)

		r = io.MultiReader(
			bytes.NewReader(m.mmap[tbl_headerSize:]),
			bufio.NewReaderSize(
				io.NewSectionReader(m.fh, int64(len(m.mmap)), size-int64(len(m.mmap))),
				1<<20,
			),
		)
	}

	var recStats recordStats
	var buf [RecordSize]byte
	var rec Record

	for idx := memtblIdx(0); ; idx++ {
		if _, err := io.ReadFull(r, buf[:]); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return Error.New("unable to read record: %w", err)
		}

		// if the record is invalid, just skip it. localized corruption should not take down the
		// entire table.
		if !rec.ReadFrom(&buf) {
			continue
		}

		if ok, err := cb(ctx, idx, rec); err != nil {
			return err
		} else if !ok {
			return nil
		}

		recStats.Include(rec)
	}

	m.statsMu.Lock()
	m.recStats = recStats
	m.statsMu.Unlock()

	return nil
}

// loadEntries is responsible for inserting all of the entries from the backing file into memory.
func (m *MemTbl) loadEntries(ctx context.Context) (_ map[uint64]*RecordTail, err error) {
	defer mon.Task()(&ctx)(&err)

	tails := make(map[uint64]*RecordTail)
	if err := m.rangeWithIdxLocked(ctx, func(ctx context.Context, idx memtblIdx, rec Record) (bool, error) {
		rt := tails[rec.Log]
		if rt == nil {
			rt = new(RecordTail)
			tails[rec.Log] = rt
		}
		rt.Push(rec)

		key := rec.Key
		short := shortKeyFrom(key)
		value := memtblIdxToValue(idx)
		op := m.entries.find(short)

		switch {
		case !op.Valid():
			// if the op is invalid, the map is full.
			return false, Error.New("memtbl memory map filled up on load")

		case !op.Exists():
			// the common case of no short collsion means we just store the entry.
			op.set(value)
			return true, nil
		}

		exValue := op.Value()
		exIdx := valueToMemTblIdx(exValue)

		switch {
		case exIdx == memtbl_Promoted:
			// already collided on the short key and has already been promoted, so we only need to
			// insert into the full collisions map.
			m.collisions[key] = value

		default:
			// an existing key at short is already set. we need to promote it to the collisions map, but
			// first we have to re-read its key from the entry at the existing index.
			var rec Record
			if err := m.readRecord(ctx, exIdx, &rec); err != nil {
				return false, Error.Wrap(err)
			}

			// um, actually, if it's the same key multiple times then we want to take the later one
			// and there's no need to promote. otherwise, we do need to promote.
			if rec.Key == key {
				op.set(value)
			} else {
				op.set(memtblIdxToValue(memtbl_Promoted))
				m.collisions[rec.Key] = exValue
				m.collisions[key] = value
			}
		}

		return true, nil
	}); err != nil {
		return nil, Error.Wrap(err)
	}

	for _, rt := range tails {
		rt.Sort()
	}

	return tails, nil
}

// keyIndexLocked returns the index associated with the key.
//
// IMPORTANT! The returned index may not actually be for the given key if there is a short key
// collision. It is the responsibility of the caller to read the record associated with that index
// and check the equality of the keys.
func (m *MemTbl) keyIndexLocked(key Key) (idx memtblIdx, ok bool) {
	if op := m.entries.find(shortKeyFrom(key)); !op.Valid() || !op.Exists() {
		return 0, false
	} else if idx = valueToMemTblIdx(op.Value()); idx != memtbl_Promoted {
		return idx, true
	} else if value, ok := m.collisions[key]; !ok {
		return 0, false
	} else {
		return valueToMemTblIdx(value), true
	}
}

// readRecord reads into rec the record written at the given index from the file.
func (m *MemTbl) readRecord(ctx context.Context, idx memtblIdx, rec *Record) error {
	if err := m.flushBufferLocked(ctx); err != nil {
		return err
	}

	var (
		valid bool
		off   = tbl_headerSize + int64(idx)*RecordSize
		b     = uint64(off)
		e     = uint64(off + RecordSize)
	)

	// if the record we're reading is in the mmap portion, use that. otherwise
	// make the syscall to read it from the file handle.
	if mm := m.mmap; b < e && e <= uint64(len(mm)) {
		valid = rec.ReadFrom((*[RecordSize]byte)(mm[b:e]))
	} else {
		var buf [RecordSize]byte
		if _, err := m.fh.ReadAt(buf[:], off); err != nil {
			return Error.New("unable to read record %d: %w", idx, err)
		}
		valid = rec.ReadFrom(&buf)
	}

	if !valid {
		return Error.New("record %d checksum failed", idx)
	}
	return nil
}

// lookupLocked returns the record associated with the key, if it exists.
func (m *MemTbl) lookupLocked(ctx context.Context, key Key) (rec Record, ok bool, err error) {
	if idx, ok := m.keyIndexLocked(key); !ok {
		return Record{}, false, nil
	} else if err := m.readRecord(ctx, idx, &rec); err != nil {
		return Record{}, false, err
	} else {
		return rec, rec.Key == key, nil
	}
}

// Close closes the mem table and returns when no more operations are running.
func (m *MemTbl) Close() error {
	m.cloMu.Lock()
	defer m.cloMu.Unlock()

	if !m.closed.Set(Error.New("memtbl closed")) {
		return m.cloErr
	}

	// grab the lock to ensure all operations have finished before closing the file handle.
	m.opMu.WaitLock()
	defer m.opMu.Unlock()

	if m.mmap != nil {
		_ = platform.Munmap(m.mmap)
		m.mmap = nil
	}

	if m.fh != nil {
		m.cloErr = errs.Combine(
			m.fh.Sync(),
			m.fh.Close(),
		)
	}

	return m.cloErr
}

// LogSlots returns the logSlots the table was created with.
func (m *MemTbl) LogSlots() uint64 { return m.header.LogSlots }

// Header returns the TblHeader the table was created with.
func (m *MemTbl) Header() TblHeader { return m.header }

// Handle returns the file handle the table was created with.
func (m *MemTbl) Handle() *os.File { return m.fh }

// Stats returns a TblStats about the mem table.
func (m *MemTbl) Stats() TblStats {
	m.statsMu.Lock()
	defer m.statsMu.Unlock()

	return TblStats{
		NumSet: m.recStats.numSet,
		LenSet: memory.Size(m.recStats.lenSet),
		AvgSet: safeDivide(float64(m.recStats.lenSet), float64(m.recStats.numSet)),

		NumTrash: m.recStats.numTrash,
		LenTrash: memory.Size(m.recStats.lenTrash),
		AvgTrash: safeDivide(float64(m.recStats.lenTrash), float64(m.recStats.numTrash)),

		NumTTL: m.recStats.numTTL,
		LenTTL: memory.Size(m.recStats.lenTTL),
		AvgTTL: safeDivide(float64(m.recStats.lenTTL), float64(m.recStats.numTTL)),

		NumSlots:  uint64(1 << m.header.LogSlots),
		TableSize: memory.Size(tbl_headerSize + RecordSize*m.recStats.numSet),
		Load:      safeDivide(float64(m.recStats.numSet), float64(uint64(1)<<m.header.LogSlots)),

		Created: m.header.Created,
		Kind:    m.header.Kind,
	}
}

// Load returns an estimate of what fraction of the mem table is occupied.
func (m *MemTbl) Load() float64 {
	m.statsMu.Lock()
	defer m.statsMu.Unlock()

	return safeDivide(float64(m.recStats.numSet), float64(uint64(1)<<m.header.LogSlots))
}

// Range iterates over the records in the mem table.
func (m *MemTbl) Range(ctx context.Context, cb func(context.Context, Record) (bool, error)) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := m.opMu.RLock(ctx, &m.closed); err != nil {
		return err
	}
	defer m.opMu.RUnlock()

	return m.rangeWithIdxLocked(ctx, func(ctx context.Context, idx memtblIdx, rec Record) (bool, error) {
		// if we have an updated record, it will be present in the file twice. only return the most
		// recent set record by checking that the index matches.
		if current, ok := m.keyIndexLocked(rec.Key); !ok || current != idx {
			return true, nil
		}
		return cb(ctx, rec)
	})
}

// Insert adds a record to the mem table. It returns (true, nil) if the record was inserted, it
// returns (false, nil) if the mem table is full, and (false, err) if any errors happened trying to
// insert the record.
func (m *MemTbl) Insert(ctx context.Context, rec Record) (_ bool, err error) {
	if err := m.opMu.Lock(ctx, &m.closed); err != nil {
		return false, err
	}
	defer m.opMu.Unlock()

	return m.insertLocked(ctx, rec)
}

func (m *MemTbl) insertLocked(ctx context.Context, rec Record) (_ bool, err error) {
	// before we do any writes we have to make sure the file is aligned to the correct size and we
	// have room to insert.
	if err := m.ensureAlignedLocked(ctx); err != nil {
		return false, err
	} else if m.idx+1 == 0 { // overflow protection on memtbl idx
		return false, nil
	} else if uint64(m.idx) > 1<<m.header.LogSlots { // full table condition
		return false, nil
	}

	// we have to check if we already have the record, and also if we have a collision on the short
	// key. if we have a collision on the short key, we need to promote it to the collisions map.
	// if we already have the record we need to check if it is equalish to the existing record.
	// we do this in one step for efficiency.
	op := m.entries.find(shortKeyFrom(rec.Key))

	// 1. if the table is full, we can't add anything anyway, so no need to do any more work. this\
	// should never happen because of the earlier check, but it's defensive.
	if !op.Valid() {
		return false, nil
	}

	insert := true // keep track of if this is an insert or an update.

	// 2. if the short key exists, we have to promote it. while promoting, we check if the records
	// are equalish because then we won't need to promote and we also get the check out of the way.
	if op.Exists() {
		val := op.Value()
		idx := valueToMemTblIdx(val)

		if idx == memtbl_Promoted {
			// if we're already promoted, we either are in the collisions map already and we need to
			// check equalish. if we aren't, we'll just be doing an update into the collisions map.
			if val, ok := m.collisions[rec.Key]; ok {
				// if we are in the collisions map, we need to check if the record is equalish.
				var tmp Record
				if err := m.readRecord(ctx, valueToMemTblIdx(val), &tmp); err != nil {
					return false, Error.Wrap(err)
				}

				if !RecordsEqualish(rec, tmp) {
					return false, Error.New("put:%v != exist:%v: %w", rec, tmp, ErrCollision)
				}
				rec.Expires = MaxExpiration(rec.Expires, tmp.Expires)
				insert = false
			}
		} else {
			// if we're not promoted, then this was either a short collision or it was an update.
			// if it's just an update, we don't want to promote. either way, we need to read the
			// record at the current index and either check if it's equalish or promote it.
			var tmp Record
			if err := m.readRecord(ctx, idx, &tmp); err != nil {
				return false, Error.Wrap(err)
			}

			// if this write is an update, enforce that it's equalish to the existing record.
			// otherwise we need to promote the existing key.
			if tmp.Key == rec.Key {
				if !RecordsEqualish(rec, tmp) {
					return false, Error.New("put:%v != exist:%v: %w", rec, tmp, ErrCollision)
				}
				rec.Expires = MaxExpiration(rec.Expires, tmp.Expires)
				insert = false
			} else {
				m.collisions[tmp.Key] = val
				op.set(memtblIdxToValue(memtbl_Promoted))
			}
		}
	}

	// 3. now that the entry is promoted and equal, we can do the flush because the rest of the
	// operations are infallible.
	var buf [RecordSize]byte
	rec.WriteTo(&buf)
	m.buffer = append(m.buffer, buf[:]...)

	// if the buffer is full, we need to flush it. in every case it should be that when the buffer
	// is full, len(m.buffer) == cap(m.buffer), but defensively we just check if there's less room
	// than a record.
	if cap(m.buffer)-len(m.buffer) < RecordSize {
		if err := m.flushBufferLocked(ctx); err != nil {
			return false, err
		}
	}

	// we can insert being sure that if a value exists in entries it's already promoted.
	if op.Exists() && valueToMemTblIdx(op.Value()) == memtbl_Promoted {
		m.collisions[rec.Key] = memtblIdxToValue(m.idx)
	} else {
		op.set(memtblIdxToValue(m.idx))
	}

	// increment our index for the next write. this is safe because if we flushed multiple records
	// above we were in the constructor and so a single failure is sticky and will cause the entire
	// memtbl to be thrown out. otherwise, we either wrote a single full record or a partial record.
	// if we wrote a single full record, then a single increment is correct. if we wrote a partial
	// record, then the error above happened and we will have the align flag set which will truncate
	// that write away, and so the idx not being incremented is correct.
	m.idx++

	// if we have mmap and the remap flag is true, we need to remap if needed.
	if m.mmap != nil && m.remap {
		m.remapIfNeeded()
	}

	// if we had an insert record, we are adding a new key, so include it in the stats. every other
	// case of update does not change the length, so we don't need to update the stats.
	if insert {
		m.statsMu.Lock()
		m.recStats.Include(rec)
		m.statsMu.Unlock()
	}

	return true, nil
}

func (m *MemTbl) remapIfNeeded() {
	size := tbl_headerSize + int64(m.idx)*RecordSize
	if size-int64(len(m.mmap)) >= platform.PageSize {
		m.remapIfNeededSlow(size)
	}
}

func (m *MemTbl) remapIfNeededSlow(size int64) {
	data, err := platform.Mremap(m.mmap, int(size-size%platform.PageSize))
	if err == nil {
		m.mmap = data

		if m.cfg.Mlock && platform.MmapSupported {
			_ = platform.Mlock(data)
		}
	}
}

func (m *MemTbl) flushBufferLocked(ctx context.Context) error {
	if len(m.buffer) == 0 {
		return nil
	}
	return m.flushBufferLockedSlow(ctx)
}

func (m *MemTbl) flushBufferLockedSlow(ctx context.Context) error {
	if err := m.ensureAlignedLocked(ctx); err != nil {
		return err
	}

	_, err := m.fh.Write(m.buffer)
	m.buffer = m.buffer[:0]
	m.align = err != nil
	return err
}

// Lookup returns the record for the given key if it exists in the mem table. It returns (rec, true,
// nil) if the record existed, (rec{}, false, nil) if it did not exist, and (rec{}, false, err) if
// any errors happened trying to look up the record.
func (m *MemTbl) Lookup(ctx context.Context, key Key) (rec Record, ok bool, err error) {
	if err := m.opMu.RLock(ctx, &m.closed); err != nil {
		return Record{}, false, err
	}
	defer m.opMu.RUnlock()

	return m.lookupLocked(ctx, key)
}

// ensureAlignedLocked ensures the file is aligned so that records are written aligned. it
// dispatches to a slow function so that it can be inlined in the common case where alignment
// is unnecessary.
func (m *MemTbl) ensureAlignedLocked(ctx context.Context) error {
	if !m.align {
		return nil
	}
	return m.ensureAlignedLockedSlow(ctx)
}

// ensureAlignedLockedSlow ensures the file is aligned so that records are written aligned. it does
// this by truncating off the unaligned end of the file.
func (m *MemTbl) ensureAlignedLockedSlow(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	size, err := fileSize(m.fh)
	if err != nil {
		return Error.New("unable to determine file size: %w", err)
	}

	size -= size % RecordSize

	if _, err := m.fh.Seek(size, io.SeekStart); err != nil {
		return Error.New("unable to seek to aligned size: %w", err)
	} else if err := m.fh.Truncate(size); err != nil {
		return Error.New("unable to truncate to aligned size: %w", err)
	}

	m.align = false
	return nil
}

// Sync syncs any modifications to disk.
func (m *MemTbl) Sync(ctx context.Context) (err error) {
	if err := m.opMu.Lock(ctx, &m.closed); err != nil {
		return err
	}
	defer m.opMu.Unlock()

	if err := m.flushBufferLocked(ctx); err != nil {
		return err
	}

	return Error.Wrap(m.fh.Sync())
}

//
// memtbl constructor
//

// MemTblConstructor constructs a MemTbl.
type MemTblConstructor struct {
	m   *MemTbl
	err error
}

// newMemTblConstructor is the constructor for MemTblConstructor.
func newMemTblConstructor(m *MemTbl) *MemTblConstructor {
	m.buffer = make([]byte, 0, bigPageSize)
	m.remap = false

	return &MemTblConstructor{m: m}
}

// valid is a helper function to convert failure conditions into an error. It is small enough to be
// inlined.
func (c *MemTblConstructor) valid() error {
	if c.err != nil {
		return c.err
	} else if c.m == nil {
		return Error.New("constructor already done")
	}
	return nil
}

// Cancel signals that we're done with the MemTblConstructor. It should always be called.
func (c *MemTblConstructor) Cancel() {
	if m := c.m; m != nil {
		_ = m.Close()
		c.m = nil
	}
}

// Append adds the record into the MemTbl. Errors are sticky and will prevent further appends.
func (c *MemTblConstructor) Append(ctx context.Context, r Record) (bool, error) {
	if err := c.valid(); err != nil {
		return false, err
	}
	var ok bool
	ok, c.err = c.m.insertLocked(ctx, r)
	return ok, c.err
}

// Done returns the constructed MemTbl or an error if there was a problem. The returned Tbl must be
// closed if it is not nil.
func (c *MemTblConstructor) Done(ctx context.Context) (Tbl, error) {
	if err := c.valid(); err != nil {
		return nil, err
	}

	// flush any remaining records in the buffer.
	if err := c.m.flushBufferLocked(ctx); err != nil {
		c.err = err
		return nil, err
	}

	// if we have mmap, remap to the final size and set the remap flag.
	if c.m.mmap != nil {
		c.m.remapIfNeeded()
		c.m.remap = true
	}

	// valid returns an error if the memtbl field is nil, so we don't have to worry about putting
	// a nil pointer in the interface.
	m := c.m
	c.m = nil

	m.buffer = make([]byte, 0, RecordSize)

	return m, nil
}
