// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package piecemigrate

import (
	"context"
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
	BatchSize         int           `help:"how many pieces to migrate at once" default:"10000"`
	Interval          time.Duration `help:"how long to wait between the batches" default:"10m"`
	MigrateRegardless bool          `help:"whether to also migrate pieces for satellites outside currently set" default:"false"`
}

// Chore migrates pieces.
//
// architecture: Chore
type Chore struct {
	log  *zap.Logger
	Loop *sync2.Cycle

	config Config
	old    Backend
	new    piecestore.PieceBackend

	migrationQueue   chan migrationItem
	baselineDataRate *monkit.FloatVal
	mu               sync.Mutex
	migrated         map[storj.NodeID]bool // map[sat](activeMigration?)
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

		config: config,
		old:    old,
		new:    new,

		migrationQueue:   make(chan migrationItem, config.BatchSize),
		baselineDataRate: mon.FloatVal("migration_chore"),
		migrated:         make(map[storj.NodeID]bool),
	}

	_ = store.Range(func(sat storj.NodeID, data []byte) error {
		b, _ := strconv.ParseBool(string(data))
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

func (chore *Chore) getMigrate(sat storj.NodeID) bool {
	chore.mu.Lock()
	defer chore.mu.Unlock()

	_, ok := chore.migrated[sat]
	return ok
}

// Run starts the chore loop to migrate pieces based on the
// configuration.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return chore.Loop.Run(ctx, chore.RunOnce)
}

// RunOnce executes a single iteration of the chore to migrate pieces
// based on the configuration.
func (chore *Chore) RunOnce(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	chore.mu.Lock()
	sats := maps.Clone(chore.migrated)
	chore.mu.Unlock()

	for sat, active := range sats {
		n, size, err := chore.runSatellite(ctx, sat, active)

		chore.log.Info("batch processed",
			zap.Error(err),
			zap.Stringer("sat", sat),
			zap.Int("successes", n),
			zap.Int64("total", size))

		if err != nil {
			return err
		}
	}

	// NOTE(artur): we will keep going if we have inflight pieces in the
	// queue, and that's probably fine. however, we will sleep for
	// chore.config.Interval after a processed batch.
	return nil
}

func (chore *Chore) runSatellite(ctx context.Context, sat storj.NodeID, activeMigration bool) (n int, total int64, err error) {
	defer mon.Task()(&ctx)(&err)

	if activeMigration {
		walkCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		if err := chore.old.WalkSatellitePieces(walkCtx, sat, func(spa pieces.StoredPieceAccess) error {
			select {
			case chore.migrationQueue <- migrationItem{satellite: sat, piece: spa.PieceID()}:
				mon.Counter("enqueued", monkit.NewSeriesTag("sat", sat.String())).Inc(1)
			default:
				cancel()
			}
			return nil
		}); err != nil && !errs2.IsCanceled(err) {
			chore.log.Info("couldn't list new pieces to migrate",
				zap.Error(err),
				zap.Stringer("sat", sat))
			// even if we couldn't list new pieces, we might have new
			// pieces from other sources in the queue now. let's keep
			// going and process them
		}
	}

	for {
		select {
		case m := <-chore.migrationQueue:
			if !chore.config.MigrateRegardless && !chore.getMigrate(m.satellite) {
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
		default:
			return n, total, nil
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
//
// NOTE/TODO?(artur): there might be a situation where there are two
// identical items in the queue due to different callers adding to the
// migration queue. that's okay, but should we proactively check for
// whether it's already in the new backend to skip the rest of the
// method? maybe this wouldn't be too frequent, so maybe that's fine.
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

	if err = dst.Commit(ctx, hdr); err != nil {
		return 0, errs.New("committing: %w", err)
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

// Close shuts down the chore's loop and releases associated resources.
// Always returns nil.
func (chore *Chore) Close() (err error) {
	chore.Loop.Close()
	return nil
}
