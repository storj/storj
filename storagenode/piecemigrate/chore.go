// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package piecemigrate

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"strconv"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"

	"storj.io/common/errs2"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/storagenode/hashstore"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/piecestore"
	"storj.io/storj/storagenode/satstore"
)

var mon = monkit.Package()

// Backend is the minimal interface that the old piece backend needs to
// implement for the migration to work.
//
// TODO(artur): make at least OldPieceBackend implement this interface,
// or give up and just put the pieces' type for the old backend.
type Backend interface {
	Writer(context.Context, storj.NodeID, storj.PieceID, pb.PieceHashAlgorithm) (*pieces.Writer, error)
	Reader(context.Context, storj.NodeID, storj.PieceID) (*pieces.Reader, error)
	WalkSatellitePieces(context.Context, storj.NodeID, func(pieces.StoredPieceAccess) error) error
	Delete(context.Context, storj.NodeID, storj.PieceID) error
}

// Config defines the configuration for the chore.
type Config struct {
	BufferSize        int           `help:"how many pieces to buffer" default:"1"`
	Interval          time.Duration `help:"how long to wait between pooling satellites for active migration" default:"10m"`
	MigrateRegardless bool          `help:"whether to also migrate pieces for satellites outside currently set" default:"false"`
}

// Chore migrates pieces.
//
// architecture: Chore
type Chore struct {
	log      *zap.Logger
	services errs2.Group
	Loop     *sync2.Cycle

	config             Config
	old                Backend
	new                piecestore.PieceBackend
	reportingBatchSize int

	mu       sync.Mutex
	migrated map[storj.NodeID]bool // map[sat](activeMigration?)

	migrationQueue   chan migrationItem
	baselineDataRate *monkit.FloatVal
	closing          sync2.Event
}

type migrationItem struct {
	satellite storj.NodeID
	piece     storj.PieceID
}

// NewChore initializes and returns a new Chore instance.
func NewChore(log *zap.Logger, config Config, store *satstore.SatelliteStore, old Backend, new piecestore.PieceBackend) *Chore {
	chore := &Chore{
		log:  log,
		Loop: sync2.NewCycle(config.Interval),

		config:             config,
		old:                old,
		new:                new,
		reportingBatchSize: 10000,

		migrated: make(map[storj.NodeID]bool),

		migrationQueue:   make(chan migrationItem, config.BufferSize),
		baselineDataRate: mon.FloatVal("migration_chore"),
	}

	_ = store.Range(func(sat storj.NodeID, data []byte) error {
		b, _ := strconv.ParseBool(string(bytes.TrimSpace(data)))
		chore.SetMigrate(sat, true, b)
		return nil
	})

	return chore
}

// Stats implements monkit.StatSource.
func (chore *Chore) Stats(cb func(key monkit.SeriesKey, field string, val float64)) {
	b2f64 := func(b bool) float64 {
		if b {
			return 1
		}
		return 0
	}

	chore.mu.Lock()
	sats := maps.Clone(chore.migrated)
	chore.mu.Unlock()

	for sat, active := range sats {
		cb(monkit.NewSeriesKey("migration_status").WithTag("sat", sat.String()), "active", b2f64(active))
	}
	cb(monkit.NewSeriesKey("queue"), "length", float64(len(chore.migrationQueue)))
}

// TryMigrateOne enqueues a migration item for the given satellite and
// piece if the queue has capacity. Fails silently if the queue is full.
func (chore *Chore) TryMigrateOne(sat storj.NodeID, piece storj.PieceID) {
	select {
	case chore.migrationQueue <- migrationItem{satellite: sat, piece: piece}:
	default:
	}
}

// SetMigrate enables or disables migration for the given satellite. If
// migrate is true, adds the satellite with its migration status to the
// active set; otherwise, removes it.
func (chore *Chore) SetMigrate(sat storj.NodeID, migrate, activeMigration bool) {
	chore.mu.Lock()
	defer chore.mu.Unlock()

	if migrate {
		chore.migrated[sat] = activeMigration
	} else {
		delete(chore.migrated, sat)
	}
}

func (chore *Chore) swapMigrate(sat storj.NodeID, oldMigrate, newMigrate, oldActive, newActive bool) {
	chore.mu.Lock()
	defer chore.mu.Unlock()

	active, migrate := chore.migrated[sat]

	if active != oldActive || migrate != oldMigrate {
		return
	}

	if newMigrate {
		chore.migrated[sat] = newActive
	} else {
		delete(chore.migrated, sat)
	}
}

func (chore *Chore) getMigrate(sat storj.NodeID) (bool, bool) {
	chore.mu.Lock()
	defer chore.mu.Unlock()

	v, ok := chore.migrated[sat]
	return v, ok
}

// Run runs the chore.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	chore.services.Go(func() error {
		return chore.Loop.Run(ctx, chore.runOnce)
	})
	chore.services.Go(func() error {
		return chore.processQueue(ctx)
	})

	return errs.Combine(chore.services.Wait()...)
}

func (chore *Chore) runOnce(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	chore.mu.Lock()
	sats := maps.Clone(chore.migrated)
	chore.mu.Unlock()

	for sat, active := range sats {
		if active {
			if err := chore.enqueueSatellite(ctx, sat); err != nil {
				chore.log.Error("failed to enqueue for migration",
					zap.Error(err),
					zap.Stringer("sat", sat))
			} else {
				chore.log.Info("enqueued for migration",
					zap.Stringer("sat", sat))
				chore.swapMigrate(sat, true, true, true, false)
			}
		}
	}

	chore.log.Info("all enqueued for migration; will sleep before next pooling",
		zap.Duration("interval", chore.config.Interval))

	return nil
}

// enqueueSatellite enqueues pieces for migration from the old to the
// new backend for a given satellite. Returns an error if it fails to
// list the pieces.
func (chore *Chore) enqueueSatellite(ctx context.Context, sat storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err = chore.old.WalkSatellitePieces(ctx, sat, func(spa pieces.StoredPieceAccess) error {
		chore.migrationQueue <- migrationItem{satellite: sat, piece: spa.PieceID()}
		mon.Counter("enqueued", monkit.NewSeriesTag("sat", sat.String())).Inc(1)
		return nil
	}); err != nil {
		return errs.New("couldn't list new pieces to migrate: %w", err)
	}

	return nil
}

// processQueue processes the migration queue, migrating pieces from the
// old to the new backend.
func (chore *Chore) processQueue(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	var (
		n     int
		total int64
	)
	for {
		if n%chore.reportingBatchSize == 0 {
			chore.log.Info("processed a bunch of pieces",
				zap.Error(err),
				zap.Int("successes", n),
				zap.Int64("size", total))
		}

		select {
		case <-chore.closing.Signaled():
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case m := <-chore.migrationQueue:
			if _, ok := chore.getMigrate(m.satellite); !chore.config.MigrateRegardless && !ok {
				incProcessedPieces(m.satellite, "skipped")
				chore.log.Debug("skipping a piece that's not part of the migration plan",
					zap.Stringer("sat", m.satellite),
					zap.Stringer("id", m.piece))
				n++
				continue
			}

			start := time.Now()
			if size, err := chore.migrateOne(ctx, m.satellite, m.piece); err != nil {
				incProcessedPieces(m.satellite, "error")
				chore.log.Info("couldn't migrate",
					zap.Error(err),
					zap.Stringer("sat", m.satellite),
					zap.Stringer("id", m.piece))
			} else {
				d := time.Since(start)
				incProcessedSuccesses(m.satellite, size, d)
				chore.log.Debug("migrated a piece",
					zap.Stringer("sat", m.satellite),
					zap.Stringer("id", m.piece),
					zap.Int64("size", size),
					zap.Duration("took", d))
				n++
				total += size
				// TODO(artur): use chore.baselineDataRate to determine
				// if we should be going slower
				chore.baselineDataRate.Observe(float64(size) / d.Seconds())
			}
		}
	}
}

func incProcessedSuccesses(sat storj.NodeID, size int64, d time.Duration) {
	incProcessedPieces(sat, "success")
	satTag := monkit.NewSeriesTag("sat", sat.String())
	mon.Counter("processed_pieces_size", satTag).Inc(size)
	mon.DurationVal("processed_pieces_duration", satTag).Observe(d)
}

func incProcessedPieces(sat storj.NodeID, result string) {
	mon.Counter("processed_pieces",
		monkit.NewSeriesTag("sat", sat.String()),
		monkit.NewSeriesTag("result", result),
	).Inc(1)
}

// migrateOne migrates a piece returning the size of the migrated piece
// and any error encountered.
func (chore *Chore) migrateOne(ctx context.Context, sat storj.NodeID, piece storj.PieceID) (size int64, err error) {
	defer mon.Task()(&ctx)(&err)

	src, err := chore.old.Reader(ctx, sat, piece)
	if err != nil {
		if errs.Is(err, fs.ErrNotExist) {
			chore.log.Debug("not in the old backend (we might have already processed it)",
				zap.Stringer("sat", sat),
				zap.Stringer("id", piece))
			return 0, nil // not in the old one, so nothing to migrate
		}
		return 0, errs.New("opening the old reader: %w", err)
	}
	defer func() {
		// we don't want upstream to think that the piece hasn't been
		// migrated if we just couldn't close the reader; log it
		// instead.
		if errClose := src.Close(); errClose != nil {
			chore.log.Debug("couldn't close the reader",
				zap.Error(errClose),
				zap.Stringer("sat", sat),
				zap.Stringer("piece", piece))
		}
	}()

	hdr, err := src.GetPieceHeader()
	if err != nil {
		return 0, errs.New("getting the piece header: %w", err)
	}

	dst, err := chore.new.Writer(ctx, sat, piece, hdr.HashAlgorithm, hdr.OrderLimit.PieceExpiration)
	if err != nil {
		return 0, errs.New("opening the new writer: %w", err)
	}
	defer func() {
		// if it's necessary to cancel the write, it likely means that
		// committing it was unsuccessful. it's not a big deal if we
		// cannot cancel it afterward, but just to be aware of it
		// happening, we're going to log the error, if any.
		if errCancel := dst.Cancel(ctx); errCancel != nil {
			chore.log.Debug("couldn't close the writer",
				zap.Error(errCancel),
				zap.Stringer("sat", sat),
				zap.Stringer("piece", piece))
		}
	}()

	size, err = sync2.Copy(ctx, dst, src)
	if err != nil {
		return 0, errs.New("while copying the piece: %w", err)
	}

	if sizeSrc, sizeDst := src.Size(), dst.Size(); !allEqual(sizeSrc, size, sizeDst) {
		return 0, errs.New("size mismatch: source=%d,written=%d,destination=%d", sizeSrc, size, sizeDst)
	}

	if err = dst.Commit(ctx, hdr); err != nil && !errs.Is(err, hashstore.ErrCollision) {
		return 0, errs.New("committing: %w", err)
	}
	if errs.Is(err, hashstore.ErrCollision) {
		chore.log.Debug("will attempt to remove a duplicate",
			zap.Error(err),
			zap.Stringer("sat", sat),
			zap.Stringer("piece", piece))
		if err = compareWithPiecestore(ctx, chore.new, sat, piece, hdr, src); err != nil {
			return 0, errs.New("duplicate pieces differ: %w", err)
		}
		// the content matches. what most likely happened is the last
		// time it was migrated, delete failed or was interrupted. we
		// are free to delete it now.
	}

	// after committing, the piece has been successfully migrated; we
	// can now delete it from the old backend.
	if err = chore.old.Delete(ctx, sat, piece); err != nil {
		return 0, errs.New("deleting: %w", err)
	}

	return size, nil
}

func allEqual(a, b, c int64) bool {
	return a == b && b == c
}

func compareWithPiecestore(
	ctx context.Context,
	backend piecestore.PieceBackend,
	sat storj.NodeID,
	piece storj.PieceID,
	suspectHeader *pb.PieceHeader,
	suspect *pieces.Reader,
) error {
	existing, err := backend.Reader(ctx, sat, piece)
	if err != nil {
		return errs.New("opening a reader: %w", err)
	}
	defer func() { _ = existing.Close() }()

	existingHeader, err := existing.GetPieceHeader()
	if err != nil {
		return errs.New("getting the piece header: %w", err)
	}

	if !pb.Equal(existingHeader, suspectHeader) {
		return errs.New("headers don't match; new=%s != old=%s", existingHeader, suspectHeader)
	}

	if _, err := suspect.Seek(0, io.SeekStart); err != nil {
		return errs.New("couldn't rewind: %w", err)
	}

	if ok, err := compareReaders(existing, suspect); err != nil {
		return errs.New("comparing readers: %w", err)
	} else if !ok {
		return errs.New("content mismatch")
	}

	return nil
}

func compareReaders(r1, r2 io.Reader) (bool, error) {
	// NOTE(artur): this approach assumes that pieces are typically
	// small and processing is sequential, making it unlikely that
	// memory usage will become excessive by reading both entirely into
	// memory. however, if the piece sizes increase significantly or
	// parallel processing is introduced, this assumption should be
	// reevaluated.
	b1, err := io.ReadAll(r1)
	if err != nil {
		return false, err
	}
	b2, err := io.ReadAll(r2)
	if err != nil {
		return false, err
	}
	return bytes.Equal(b1, b2), nil
}

// Close shuts down the chore's loop and releases associated resources.
// Always returns nil.
func (chore *Chore) Close() (err error) {
	chore.Loop.Close()
	chore.closing.Signal()
	return nil
}
