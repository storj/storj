// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/csv"
	"errors"
	"os"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/private/cfgstruct"
	"storj.io/private/dbutil/pgutil"
	"storj.io/private/process"
	"storj.io/private/tagsql"
	"storj.io/storj/satellite/metabase"
)

var mon = monkit.Package()

var (
	rootCmd = &cobra.Command{
		Use:   "migrate-segment-copies",
		Short: "migrate-segment-copies",
	}

	runCmd = &cobra.Command{
		Use:   "run",
		Short: "run migrate-segment-copies",
		RunE:  run,
	}

	config Config
)

func init() {
	rootCmd.AddCommand(runCmd)

	cfgstruct.Bind(pflag.CommandLine, &config)
}

// Config defines configuration for migration.
type Config struct {
	MetabaseDB          string `help:"connection URL for metabaseDB"`
	BatchSize           int    `help:"number of entries from segment_copies processed at once" default:"2000"`
	SegmentCopiesBackup string `help:"cvs file where segment copies entries will be backup"`
}

// VerifyFlags verifies whether the values provided are valid.
func (config *Config) VerifyFlags() error {
	var errlist errs.Group
	if config.MetabaseDB == "" {
		errlist.Add(errors.New("flag '--metabasedb' is not set"))
	}
	return errlist.Err()
}

func run(cmd *cobra.Command, args []string) error {
	if err := config.VerifyFlags(); err != nil {
		return err
	}

	ctx, _ := process.Ctx(cmd)
	log := zap.L()
	return Migrate(ctx, log, config)
}

func main() {
	process.Exec(rootCmd)
}

// Migrate starts segment copies migration.
func Migrate(ctx context.Context, log *zap.Logger, config Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	db, err := metabase.Open(ctx, log, config.MetabaseDB, metabase.Config{})
	if err != nil {
		return errs.New("unable to connect %q: %w", config.MetabaseDB, err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	return MigrateSegments(ctx, log, db, config)
}

// MigrateSegments updates segment copies with proper metadata (pieces and placment).
func MigrateSegments(ctx context.Context, log *zap.Logger, metabaseDB *metabase.DB, config Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	var backupCSV *csv.Writer
	if config.SegmentCopiesBackup != "" {
		f, err := os.Create(config.SegmentCopiesBackup)
		if err != nil {
			return err
		}

		defer func() {
			err = errs.Combine(err, f.Close())
		}()

		backupCSV = csv.NewWriter(f)

		defer backupCSV.Flush()

		if err := backupCSV.Write([]string{"stream_id", "ancestor_stream_id"}); err != nil {
			return err
		}
	}

	db := metabaseDB.UnderlyingTagSQL()

	var streamIDCursor uuid.UUID
	ancestorStreamIDs := []uuid.UUID{}
	streamIDs := []uuid.UUID{}
	processed := 0

	// what we are doing here:
	// * read batch of entries from segment_copies table
	// * read ancestors (original) segments metadata from segments table
	// * update segment copies with missing metadata, one by one
	// * delete entries from segment_copies table
	for {
		log.Info("Processed entries", zap.Int("processed", processed))

		ancestorStreamIDs = ancestorStreamIDs[:0]
		streamIDs = streamIDs[:0]

		idsMap := map[uuid.UUID][]uuid.UUID{}
		err := withRows(db.QueryContext(ctx, `
				SELECT stream_id, ancestor_stream_id FROM segment_copies WHERE stream_id > $1 ORDER BY stream_id LIMIT $2
			`, streamIDCursor, config.BatchSize))(func(rows tagsql.Rows) error {
			for rows.Next() {
				var streamID, ancestorStreamID uuid.UUID
				err := rows.Scan(&streamID, &ancestorStreamID)
				if err != nil {
					return err
				}

				streamIDCursor = streamID
				ancestorStreamIDs = append(ancestorStreamIDs, ancestorStreamID)
				streamIDs = append(streamIDs, streamID)

				idsMap[ancestorStreamID] = append(idsMap[ancestorStreamID], streamID)
			}
			return nil
		})
		if err != nil {
			return err
		}

		type Update struct {
			StreamID          uuid.UUID
			AncestorStreamID  uuid.UUID
			Position          int64
			RemoteAliasPieces []byte
			RootPieceID       []byte
			RepairedAt        *time.Time
			Placement         int64
		}

		updates := []Update{}
		err = withRows(db.QueryContext(ctx, `
				SELECT stream_id, position, remote_alias_pieces, root_piece_id, repaired_at, placement FROM segments WHERE stream_id = ANY($1::BYTEA[])
			`, pgutil.UUIDArray(ancestorStreamIDs)))(func(rows tagsql.Rows) error {
			for rows.Next() {
				var ancestorStreamID uuid.UUID
				var position int64
				var remoteAliasPieces, rootPieceID []byte
				var repairedAt *time.Time
				var placement int64
				err := rows.Scan(&ancestorStreamID, &position, &remoteAliasPieces, &rootPieceID, &repairedAt, &placement)
				if err != nil {
					return err
				}

				streamIDs, ok := idsMap[ancestorStreamID]
				if !ok {
					return errs.New("unable to map ancestor stream id: %s", ancestorStreamID)
				}

				for _, streamID := range streamIDs {
					updates = append(updates, Update{
						StreamID:          streamID,
						AncestorStreamID:  ancestorStreamID,
						Position:          position,
						RemoteAliasPieces: remoteAliasPieces,
						RootPieceID:       rootPieceID,
						RepairedAt:        repairedAt,
						Placement:         placement,
					})
				}
			}
			return nil
		})
		if err != nil {
			return err
		}

		for _, update := range updates {
			_, err := db.ExecContext(ctx, `
					UPDATE segments SET
						remote_alias_pieces = $3,
						root_piece_id       = $4,
						repaired_at         = $5,
						placement           = $6
					WHERE (stream_id, position) = ($1, $2)
				`, update.StreamID, update.Position, update.RemoteAliasPieces, update.RootPieceID, update.RepairedAt, update.Placement)
			if err != nil {
				return err
			}

			if backupCSV != nil {
				if err := backupCSV.Write([]string{update.StreamID.String(), update.AncestorStreamID.String()}); err != nil {
					return err
				}
			}
		}

		if backupCSV != nil {
			backupCSV.Flush()
		}

		processed += len(streamIDs)

		if len(updates) == 0 {
			return nil
		}
	}
}

func withRows(rows tagsql.Rows, err error) func(func(tagsql.Rows) error) error {
	return func(callback func(tagsql.Rows) error) error {
		if err != nil {
			return err
		}
		err := callback(rows)
		return errs.Combine(rows.Err(), rows.Close(), err)
	}
}
