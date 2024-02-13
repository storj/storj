// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"math"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/process"
	"storj.io/common/storj"
	"storj.io/common/tagsql"
	"storj.io/storj/satellite/satellitedb"
)

var mon = monkit.Package()

var (
	rootCmd = &cobra.Command{
		Use:   "delete-uncontacted-nodes",
		Short: "delete-uncontacted-nodes",
	}

	runCmd = &cobra.Command{
		Use:   "run",
		Short: "run delete-uncontacted-nodes",
		RunE:  run,
	}

	config Config
)

func init() {
	rootCmd.AddCommand(runCmd)

	config.BindFlags(runCmd.Flags())
}

// Config defines configuration for deletion.
type Config struct {
	SatelliteDB string
	Limit       int
	CreatedAt   string

	MaxIterations int
}

// BindFlags adds bench flags to the flagset.
func (config *Config) BindFlags(flag *flag.FlagSet) {
	flag.StringVar(&config.SatelliteDB, "satellitedb", "", "connection URL for satelliteDB")
	flag.IntVar(&config.Limit, "limit", 1000, "number of deletes to perform at once")
	flag.StringVar(&config.CreatedAt, "created-at", "", "latest node creation date for which to delete in iso8601 format YYYY-MM-DD")
	flag.IntVar(&config.MaxIterations, "max-iterations", -1, "number of maximum iterations (negative is unlimited)")
}

// VerifyFlags verifies whether the values provided are valid.
func (config *Config) VerifyFlags() error {
	var errlist errs.Group
	if config.SatelliteDB == "" {
		errlist.Add(errors.New("flag '--satellitedb' is not set"))
	}
	return errlist.Err()
}

func run(cmd *cobra.Command, args []string) error {
	if err := config.VerifyFlags(); err != nil {
		return err
	}

	ctx, _ := process.Ctx(cmd)
	log := zap.L()
	return Delete(ctx, log, config)
}

func main() {
	process.Exec(rootCmd)
}

// Delete opens the database and starts the database.
func Delete(ctx context.Context, log *zap.Logger, config Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	db, err := satellitedb.Open(ctx, log.Named("db"), config.SatelliteDB, satellitedb.Options{
		ApplicationName: "node-cleanup",
	})
	if err != nil {
		return errs.New("unable to connect %q: %w", config.SatelliteDB, err)
	}
	defer func() { err = errs.Combine(err, db.Close()) }()

	if err := db.CheckVersion(ctx); err != nil {
		return errs.New("database version not correct: %w", err)
	}

	return DeleteFromTables(ctx, log, db.Testing().RawDB(), config)
}

var maxNodeID = (func() storj.NodeID {
	var x storj.NodeID
	for i := range x {
		x[i] = 0xff
	}
	return x
})()

// DeleteFromTables deletes nodes matching the query in batches.
func DeleteFromTables(ctx context.Context, log *zap.Logger, db tagsql.DB, config Config) (err error) {
	var cursor storj.NodeID

	progress := 0
	if config.MaxIterations < 0 {
		config.MaxIterations = math.MaxInt
	}
	more := true
	for iteration := 0; more && iteration < config.MaxIterations; iteration++ {
		var batchEnd storj.NodeID
		err := db.QueryRowContext(ctx, `
			SELECT id
			FROM nodes
			WHERE id > $1
			ORDER BY id
			OFFSET $2 LIMIT 1
		`, cursor, config.Limit-1).Scan(&batchEnd)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				batchEnd = maxNodeID
				more = false
			} else {
				return errs.New("batch end query failed: %w", err)
			}
		}
		progress += config.Limit

		log.Info("deleting batch",
			zap.String("from", hex.EncodeToString(cursor[:])),
			zap.String("to", hex.EncodeToString(batchEnd[:])),
			zap.Int("progress", progress))
		start := time.Now()

		var deletedNodes, deletedPaystubs, deletedPeerIdentities, deletedNodeAPIVersions int64

		err = db.QueryRowContext(ctx, `
			WITH deleted_nodes AS (
				DELETE FROM nodes
				WHERE id > $1 AND id <= $2
				AND last_contact_success = '0001-01-01 00:00:00+00'
				AND created_at <= $3
				RETURNING id
			),
				deleted_paystubs AS (
				DELETE FROM storagenode_paystubs
				WHERE node_id in (select deleted_nodes.id FROM deleted_nodes)
				RETURNING 1
			),
				deleted_peer_identities AS (
				DELETE FROM peer_identities
				WHERE node_id in (select deleted_nodes.id FROM deleted_nodes)
				RETURNING 1
			),
				deleted_node_api_versions AS (
				DELETE FROM node_api_versions
				WHERE id in (select deleted_nodes.id FROM deleted_nodes)
				RETURNING 1
			)
			SELECT
				(select count(*) from deleted_nodes),
				(select count(*) from deleted_paystubs),
				(select count(*) from deleted_peer_identities),
				(select count(*) from deleted_node_api_versions)
		`, cursor, batchEnd, config.CreatedAt).Scan(&deletedNodes, &deletedPaystubs, &deletedPeerIdentities, &deletedNodeAPIVersions)
		if err != nil {
			return errs.New("batch deletion failed: %w", err)
		}
		log.Info("delete batch",
			zap.Duration("duration", time.Since(start)),
			zap.Int64("nodes", deletedNodes),
			zap.Int64("paystubs", deletedPaystubs),
			zap.Int64("peer identities", deletedPeerIdentities),
			zap.Int64("node api versions", deletedNodeAPIVersions),
		)

		cursor = batchEnd
	}
	return nil
}
