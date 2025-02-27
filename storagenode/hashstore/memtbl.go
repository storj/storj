// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"sync"

	"storj.io/common/memory"
	"storj.io/drpc/drpcsignal"
)

type shortKey = [5]byte

type memtblIdx uint32 // index of a record in the memtbl (^0 means promoted)

const memtbl_Promoted = ^memtblIdx(0)

var le = binary.LittleEndian

func memtblIdxToValue(idx memtblIdx) (b [4]byte) { le.PutUint32(b[:], uint32(idx)); return }
func valueToMemtblIdx(b [4]byte) (idx memtblIdx) { return memtblIdx(le.Uint32(b[:])) }

// MemTbl is an in-memory hash table of records with a file for persistence.
type MemTbl struct {
	fh     *os.File
	header TblHeader

	opMu rwMutex          // protects operations
	idx  memtblIdx        // insert index
	tmp  [RecordSize]byte // used to save allocations on Insert

	entries    map[shortKey][4]byte
	collisions map[Key][4]byte

	closed drpcsignal.Signal // closed state
	cloMu  sync.Mutex        // synchronizes closing

	buffer *bufio.Writer // buffer for inserts

	statsMu  sync.Mutex // protects the following fields
	recStats recordStats
}

// CreateMemtbl allocates a new mem table with the given log base 2 number of records and created
// timestamp.
func CreateMemtbl(ctx context.Context, fh *os.File, logSlots uint64, created uint32) (_ *MemTbl, err error) {
	defer mon.Task()(&ctx)(&err)

	if logSlots > tbl_maxLogSlots {
		return nil, Error.New("logSlots too large: logSlots=%d", logSlots)
	} else if logSlots < tbl_minLogSlots {
		return nil, Error.New("logSlots too small: logSlots=%d", logSlots)
	}

	header := TblHeader{
		Created:  created,
		HashKey:  true,
		Kind:     kind_MemTbl,
		LogSlots: logSlots,
	}

	// clear the file and truncate it to the correct length and write the header page.
	if err := fh.Truncate(0); err != nil {
		return nil, Error.New("unable to truncate memtbl to 0: %w", err)
	} else if err := fh.Truncate(headerSize); err != nil {
		return nil, Error.New("unable to truncate memtbl to %d: %w", headerSize, err)
	} else if err := WriteTblHeader(fh, header); err != nil {
		return nil, Error.Wrap(err)
	}

	// this is a bit wasteful in the sense that we will do some stat calls, reread the header page,
	// but it reduces code paths and is not that expensive overall.
	return OpenMemtbl(ctx, fh)
}

// OpenMemtbl opens an existing hash table stored in the given file handle.
func OpenMemtbl(ctx context.Context, fh *os.File) (_ *MemTbl, err error) {
	defer mon.Task()(&ctx)(&err)

	// ensure the file is appropriately aligned and seek it to the end for writes.
	size, err := fh.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, Error.New("unable to determine memtbl size: %w", err)
	} else if size < headerSize || (size-headerSize)%RecordSize != 0 {
		return nil, Error.New("memtbl size not aligned to record: size=%d", size)
	}

	// read the header information from the first page.
	header, err := ReadTblHeader(fh)
	if err != nil {
		return nil, Error.Wrap(err)
	} else if header.Kind != kind_MemTbl {
		return nil, Error.New("invalid kind: %d", header.Kind)
	}

	h := &MemTbl{
		fh:     fh,
		header: header,

		idx: memtblIdx((size - headerSize) / RecordSize),

		entries:    make(map[shortKey][4]byte, 1<<header.LogSlots),
		collisions: make(map[Key][4]byte),
	}

	// read the entries from the file.
	if err := h.loadEntries(ctx); err != nil {
		return nil, Error.Wrap(err)
	}

	return h, nil
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
	} else if size < headerSize {
		return Error.New("file too small: size=%d", size)
	}

	// a bufio.Reader on a SectionReader so that it doesn't mess up the file pos for write calls.
	br := bufio.NewReaderSize(io.NewSectionReader(m.fh, headerSize, size-headerSize), 1<<20)

	var recStats recordStats
	var buf [RecordSize]byte
	var rec Record
	for idx := memtblIdx(0); ; idx++ {
		if _, err := io.ReadFull(br, buf[:]); errors.Is(err, io.EOF) {
			break
		} else if errors.Is(err, io.ErrUnexpectedEOF) {
			// if we had a failed write, we could be unaligned. range should ignore the partially
			// written final record. we ensure alignment and truncate at file open so this is the
			// only time this should happen.
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

		recStats.include(rec)
	}

	m.statsMu.Lock()
	m.recStats = recStats
	m.statsMu.Unlock()

	return nil
}

// loadEntries is responsible for inserting all of the entries from the backing file into memory.
func (m *MemTbl) loadEntries(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return m.rangeWithIdxLocked(ctx, func(ctx context.Context, idx memtblIdx, rec Record) (bool, error) {
		return true, m.insertKeyLocked(rec.Key, idx)
	})
}

// insertKeyLocked associates the key with the memtbl index, promoting in case there is a collision.
func (m *MemTbl) insertKeyLocked(key Key, idx memtblIdx) error {
	short := *(*shortKey)(key[:])
	value := memtblIdxToValue(idx)
	exValue, ok := m.entries[short]
	exIdx := valueToMemtblIdx(exValue)

	switch {
	case !ok:
		// the common case of no short collision means we just store the entry.
		m.entries[short] = value

	case exIdx == memtbl_Promoted:
		// already collided on the short key and has already been promoted, so we only need to
		// insert into the full collisions map.
		m.collisions[key] = value

	default:
		// an existing key at short is already set. we need to promote it to the collisions map, but
		// first we have to re-read its key from the entry at the existing index.
		var rec Record
		if err := m.readRecord(exIdx, &rec); err != nil {
			return Error.Wrap(err)
		}

		// um, actually, if it's the same key multiple times then we want to take the later one
		// and there's no need to promote. otherwise, we do need to promote.
		if rec.Key == key {
			m.entries[short] = value
		} else {
			m.entries[short] = memtblIdxToValue(memtbl_Promoted)
			m.collisions[rec.Key] = exValue
			m.collisions[key] = value
		}
	}

	return nil
}

// keyIndexLocked returns the index associated with the key.
//
// IMPORTANT! The returned index may not actually be for the given key if there is a short key
// collision. It is the responsibility of the caller to read the record associated with that index
// and check the equality of the keys.
func (m *MemTbl) keyIndexLocked(key Key) (idx memtblIdx, ok bool) {
	if value, ok := m.entries[*(*shortKey)(key[:])]; !ok {
		return 0, false
	} else if idx = valueToMemtblIdx(value); idx != memtbl_Promoted {
		return idx, true
	} else if value, ok = m.collisions[key]; !ok {
		return 0, false
	} else {
		return valueToMemtblIdx(value), true
	}
}

// readRecord reads into rec the record written at the given index from the file.
func (m *MemTbl) readRecord(idx memtblIdx, rec *Record) error {
	var buf [RecordSize]byte
	if _, err := m.fh.ReadAt(buf[:], headerSize+int64(idx)*RecordSize); err != nil {
		return Error.New("unable to read record %d: %w", idx, err)
	} else if !rec.ReadFrom(&buf) {
		return Error.New("record %d checksum failed", idx)
	}
	return nil
}

// lookupLocked returns the record associated with the key, if it exists.
func (m *MemTbl) lookupLocked(key Key) (rec Record, ok bool, err error) {
	if idx, ok := m.keyIndexLocked(key); !ok {
		return Record{}, false, nil
	} else if err := m.readRecord(idx, &rec); err != nil {
		return Record{}, false, err
	} else {
		return rec, rec.Key == key, nil
	}
}

// Close closes the mem table and returns when no more operations are running.
func (m *MemTbl) Close() {
	m.cloMu.Lock()
	defer m.cloMu.Unlock()

	if !m.closed.Set(Error.New("memtbl closed")) {
		return
	}

	// grab the lock to ensure all operations have finished before closing the file handle.
	m.opMu.WaitLock()
	defer m.opMu.Unlock()

	_ = m.fh.Close()
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

		NumSlots:  uint64(1 << m.header.LogSlots),
		TableSize: memory.Size(headerSize + RecordSize*m.recStats.numSet),
		Load:      safeDivide(float64(m.recStats.numSet), float64(uint64(1)<<m.header.LogSlots)),

		Created: m.header.Created,
		Kind:    m.header.Kind,
	}
}

// ComputeEstimates doesn't do anything because the memtbl always has exact estimates.
func (m *MemTbl) ComputeEstimates(ctx context.Context) error {
	if err := m.opMu.Lock(ctx, &m.closed); err != nil {
		return err
	}
	defer m.opMu.Unlock()

	// memtbl is always exact :smug:
	return nil
}

// CompactLoad returns the load factor the tbl should be compacted at.
func (m *MemTbl) CompactLoad() float64 { return 0.95 }

// MaxLoad returns the load factor at which no more inserts should happen.
func (m *MemTbl) MaxLoad() float64 { return 1.00 }

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

// ExpectOrdered signals that incoming writes to the memtbl will be ordered so that a large shared
// buffer across Insert calls would be effective. This is useful when rewriting a memtbl during a
// Compaction, for instance. It returns a flush callback that both flushes any potentially buffered
// records and disables the expectation. Additionally, Lookups may not find entries written until
// after the flush callback is called. If flush returns an error there is no guarantee about what
// records were written. It returns a done callback that discards any potentially buffered records
// and disables the expectation. At least one of flush or done must be called. It returns an error
// if called again before flush or done is called.
func (m *MemTbl) ExpectOrdered(ctx context.Context) (commit func() error, done func(), err error) {
	defer mon.Task()(&ctx)(&err)

	if err := m.opMu.Lock(ctx, &m.closed); err != nil {
		return nil, nil, err
	}
	defer m.opMu.Unlock()

	if m.buffer != nil {
		return nil, nil, Error.New("buffer already exists")
	}

	buffer := bufio.NewWriterSize(m.fh, bigPageSize)
	m.buffer = buffer

	return func() (err error) {
			m.opMu.WaitLock()
			defer m.opMu.Unlock()

			if buffer == m.buffer {
				err = Error.Wrap(m.buffer.Flush())
				m.buffer = nil
			}

			return err
		}, func() {
			m.opMu.WaitLock()
			defer m.opMu.Unlock()

			if buffer == m.buffer {
				m.buffer = nil
			}
		}, nil
}

// Insert adds a record to the mem table. It returns (true, nil) if the record was inserted, it
// returns (false, nil) if the mem table is full, and (false, err) if any errors happened trying to
// insert the record.
func (m *MemTbl) Insert(ctx context.Context, rec Record) (_ bool, err error) {
	if err := m.opMu.Lock(ctx, &m.closed); err != nil {
		return false, err
	}
	defer m.opMu.Unlock()

	// if we already have this record, then we need to ensure they are equalish
	// before allowing the update, and taking the larger expiration.
	tmp, existing, err := m.lookupLocked(rec.Key)
	if err != nil {
		return false, err
	} else if existing {
		if !RecordsEqualish(rec, tmp) {
			return false, Error.New("put:%v != exist:%v: %w", rec, tmp, ErrCollision)
		}
		rec.Expires = MaxExpiration(rec.Expires, tmp.Expires)
	}

	// N.B. this is safe because opMu is exclusive. we do this because m.buffer is a bufio.Writer
	// which virtual dispatches to any io.Writer, causing it to escape in the common case where
	// it's not even set.
	rec.WriteTo(&m.tmp)

	if m.buffer != nil {
		_, err = m.buffer.Write(m.tmp[:])
	} else {
		_, err = m.fh.Write(m.tmp[:])
	}
	if err != nil {
		// TODO: uh oh. we wrote some number of bytes. we should go into a mode where we try to fix
		// what we did and set a flag if we can't to cause the next write to try to fix. this is an
		// even bigger problem if m.buffer is set. we may just have to flag that to fail until it's
		// unset.
		return false, Error.New("unable to write record: %w", err)
	}

	if err := m.insertKeyLocked(rec.Key, m.idx); err != nil {
		// TODO: uh oh. we wrote the record but we couldn't store it in memory because apparently
		// the overflow check failed. is this a problem to have the record in the file? i dunno.
		return false, err
	}

	// TODO: the idx needs to always be consistent with where we are going to write to into the file
	// and this might not if we end up doing some recovery things.
	m.idx++

	// if we didn't have an existing record, we are adding a new key. we don't need to change the
	// alive field on update because we ensure that the records are equalish above so the length
	// field could not have changed. we're ignoring the update case for trash because it should be
	// very rare and doing it properly would require subtracting which may underflow in situations
	// where the estimate was too small. this technically means that in very rare scenarios, the
	// amount considered trash could be off, but it will be fixed on the next Range call, Store
	// compaction, or node restart.
	if !existing {
		m.statsMu.Lock()
		m.recStats.include(rec)
		m.statsMu.Unlock()
	}

	return true, nil
}

// Lookup returns the record for the given key if it exists in the mem table. It returns (rec, true,
// nil) if the record existed, (rec{}, false, nil) if it did not exist, and (rec{}, false, err) if
// any errors happened trying to look up the record.
func (m *MemTbl) Lookup(ctx context.Context, key Key) (rec Record, ok bool, err error) {
	if err := m.opMu.RLock(ctx, &m.closed); err != nil {
		return Record{}, false, err
	}
	defer m.opMu.RUnlock()

	return m.lookupLocked(key)
}
