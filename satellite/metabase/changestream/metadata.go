// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package changestream

import (
	"context"
	"strconv"
	"time"

	"cloud.google.com/go/spanner"
	database "cloud.google.com/go/spanner/admin/database/apiv1"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"github.com/zeebo/errs"

	"storj.io/storj/shared/dbutil/recordeddb"
	"storj.io/storj/shared/dbutil/spannerutil"
)

// PartitionState represents the processing state of a partition.
type PartitionState int

// Possible states for a partition in its processing lifecycle.
const (
	StateCreated   PartitionState = 0
	StateScheduled PartitionState = 1
	StateRunning   PartitionState = 2
	StateFinished  PartitionState = 3
)

// String constants for SQL queries.
const (
	stateCreated   = "0"
	stateScheduled = "1"
	stateRunning   = "2"
	stateFinished  = "3"
)

// Valid returns whether the PartitionState is a valid state.
func (s PartitionState) Valid() bool {
	switch s {
	case StateCreated, StateScheduled, StateRunning, StateFinished:
		return true
	default:
		return false
	}
}

// EncodeSpanner implements spanner.Encoder for PartitionState.
func (s PartitionState) EncodeSpanner() (interface{}, error) {
	if !s.Valid() {
		return nil, errs.New("invalid PartitionState value: %d", s)
	}

	return int64(s), nil
}

// DecodeSpanner implements spanner.Decoder for PartitionState.
func (s *PartitionState) DecodeSpanner(val interface{}) error {
	var intVal int64

	switch v := val.(type) {
	case int64:
		intVal = v
	case string:
		// Spanner may return INT64 columns as strings in some contexts
		var err error
		intVal, err = strconv.ParseInt(v, 10, 64)
		if err != nil {
			return errs.New("failed to parse PartitionState from string %q: %w", v, err)
		}
	default:
		return errs.New("failed to decode PartitionState: expected int64 or string, got %T", val)
	}

	state := PartitionState(intVal)
	if !state.Valid() {
		return errs.New("invalid PartitionState value in database: %d", intVal)
	}

	*s = state

	return nil
}

// NoPartitionMetadata checks if the metadata table for the change stream is empty.
func NoPartitionMetadata(ctx context.Context, client *recordeddb.SpannerClient, feedName string) (empty bool, err error) {
	defer mon.Task()(&ctx)(&err)

	metadataTable := spannerutil.QuoteIdentifier(feedName + "_metadata")

	stmt := spanner.Statement{
		SQL: `
			SELECT 1
			FROM ` + metadataTable + `
			LIMIT 1
		`,
	}

	var exists bool
	err = client.Single().QueryWithOptions(ctx, stmt,
		spanner.QueryOptions{RequestTag: "change-stream-no-partition-metadata"},
	).Do(func(row *spanner.Row) error {
		exists = true
		return nil
	})
	if err != nil {
		return false, errs.Wrap(err)
	}

	return !exists, nil
}

// GetPartitionsByState retrieves change stream partitions by their state from the metabase.
func GetPartitionsByState(ctx context.Context, client *recordeddb.SpannerClient, feedName string, state PartitionState) (partitions map[string]time.Time, err error) {
	defer mon.Task()(&ctx)(&err)

	metadataTable := spannerutil.QuoteIdentifier(feedName + "_metadata")

	stmt := spanner.Statement{
		SQL: `
			SELECT partition_token, watermark
			FROM ` + metadataTable + `
			WHERE state = @state
		`,
		Params: map[string]interface{}{
			"state": state,
		},
	}

	partitions = make(map[string]time.Time)

	err = client.Single().QueryWithOptions(ctx, stmt,
		spanner.QueryOptions{RequestTag: "change-stream-get-partitions-by-state"},
	).Do(func(row *spanner.Row) error {
		var token string
		var watermark time.Time
		if err := row.Columns(&token, &watermark); err != nil {
			return errs.Wrap(err)
		}
		partitions[token] = watermark
		return nil
	})
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return partitions, nil
}

// AddChildPartition adds a child partition to the metabase.
func AddChildPartition(ctx context.Context, client *recordeddb.SpannerClient, feedName, childToken string, parentTokens []string, start time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	metadataTable := spannerutil.QuoteIdentifier(feedName + "_metadata")

	// watermark is initialized to start time
	stmt := spanner.Statement{
		SQL: `
			INSERT INTO ` + metadataTable + `
				(partition_token, parent_tokens, start_timestamp, watermark)
			VALUES
				(@partition_token, @parent_tokens, @start_timestamp, @start_timestamp)
		`,
		Params: map[string]interface{}{
			"partition_token": childToken,
			"parent_tokens":   parentTokens,
			"start_timestamp": start,
		},
	}

	_, err = client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		_, err := tx.UpdateWithOptions(ctx, stmt,
			spanner.QueryOptions{RequestTag: "change-stream-add-child-partition"})
		return errs.Wrap(err)
	}, spanner.TransactionOptions{
		TransactionTag: "change-stream-add-child-partition",
	})

	if spannerutil.IsAlreadyExists(err) {
		// Expected error when Spanner merges partitions - all parents try to add the same child partition
		return nil
	}

	return errs.Wrap(err)
}

// SchedulePartitions checks each partition in created state, and if all its parent partitions are finished, it will update its state to scheduled.
//
// Some rules:
// - The initial partition (with partition_token = ") is scheduled immediately.
// - The children of the initial partition (with no parents) are scheduled once the initial partition is finished.
// - Other partitions are scheduled once all their parent partitions are finished.
func SchedulePartitions(ctx context.Context, client *recordeddb.SpannerClient, feedName string) (scheduledCount int64, err error) {
	defer mon.Task()(&ctx)(&err)

	metadataTable := spannerutil.QuoteIdentifier(feedName + "_metadata")

	stmt := spanner.Statement{
		SQL: `
			UPDATE ` + metadataTable + ` AS child
			SET
				state = ` + stateScheduled + `,
				scheduled_at = PENDING_COMMIT_TIMESTAMP()
			WHERE child.state = ` + stateCreated + `
			AND (
				child.partition_token = ''
				OR (
					(child.parent_tokens IS NULL OR ARRAY_LENGTH(child.parent_tokens) = 0)
					AND (
						SELECT state
						FROM ` + metadataTable + `
						WHERE partition_token = ''
					) = ` + stateFinished + `
				)
				OR (
					SELECT LOGICAL_AND(parent.state = ` + stateFinished + `)
					FROM UNNEST(child.parent_tokens) AS parent_token
					JOIN ` + metadataTable + ` AS parent ON parent.partition_token = parent_token
				) = TRUE
			)
		`,
	}

	_, err = client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		count, err := tx.UpdateWithOptions(ctx, stmt,
			spanner.QueryOptions{RequestTag: "change-stream-schedule-partitions"})
		if err != nil {
			return err
		}
		scheduledCount = count
		return nil
	}, spanner.TransactionOptions{
		TransactionTag: "change-stream-schedule-partitions",
	})

	return scheduledCount, errs.Wrap(err)
}

// UpdatePartitionWatermark updates the watermark for a change stream partition in the metabase.
func UpdatePartitionWatermark(ctx context.Context, client *recordeddb.SpannerClient, feedName, partitionToken string, newWatermark time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	metadataTable := spannerutil.QuoteIdentifier(feedName + "_metadata")

	stmt := spanner.Statement{
		SQL: `
			UPDATE ` + metadataTable + `
			SET watermark = @new_watermark
			WHERE partition_token = @partition_token
		`,
		Params: map[string]interface{}{
			"partition_token": partitionToken,
			"new_watermark":   newWatermark,
		},
	}

	_, err = client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		count, err := tx.UpdateWithOptions(ctx, stmt,
			spanner.QueryOptions{RequestTag: "change-stream-update-partition-watermark"})
		if err != nil {
			return err
		}
		if count == 0 {
			return errs.New("partition watermark update affected 0 rows: partition_token=%q", partitionToken)
		}
		return nil
	}, spanner.TransactionOptions{
		TransactionTag: "change-stream-update-partition-watermark",
	})

	return errs.Wrap(err)
}

// UpdatePartitionState updates the state for a change stream partition in the metabase.
func UpdatePartitionState(ctx context.Context, client *recordeddb.SpannerClient, feedName, partitionToken string, newState PartitionState) (err error) {
	defer mon.Task()(&ctx)(&err)

	metadataTable := spannerutil.QuoteIdentifier(feedName + "_metadata")

	// Build the SET clause based on the new state
	// Note: StateCreated is not a valid target state - partitions are created via AddChildPartition
	var setClause string
	switch newState {
	case StateCreated:
		return errs.New("cannot update to StateCreated: partitions are created via AddChildPartition")
	case StateScheduled:
		setClause = "SET state = " + stateScheduled + ", scheduled_at = PENDING_COMMIT_TIMESTAMP()"
	case StateRunning:
		setClause = "SET state = " + stateRunning + ", running_at = PENDING_COMMIT_TIMESTAMP()"
	case StateFinished:
		setClause = "SET state = " + stateFinished + ", finished_at = PENDING_COMMIT_TIMESTAMP()"
	default:
		return errs.New("invalid partition state: %d", newState)
	}

	stmt := spanner.Statement{
		SQL: `
			UPDATE ` + metadataTable + `
			` + setClause + `
			WHERE partition_token = @partition_token
		`,
		Params: map[string]interface{}{
			"partition_token": partitionToken,
		},
	}

	_, err = client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		count, err := tx.UpdateWithOptions(ctx, stmt,
			spanner.QueryOptions{RequestTag: "change-stream-update-partition-state"})
		if err != nil {
			return err
		}
		if count == 0 {
			return errs.New("partition state update affected 0 rows: partition_token=%q", partitionToken)
		}
		return nil
	}, spanner.TransactionOptions{
		TransactionTag: "change-stream-update-partition-state",
	})

	return errs.Wrap(err)
}

// TestCreateChangeStreamMetadata creates the metadata table and index for a change stream for testing purposes.
func TestCreateChangeStreamMetadata(ctx context.Context, admin *database.DatabaseAdminClient, path string, name string) error {
	metadataTable := spannerutil.QuoteIdentifier(name + "_metadata")
	metadataTableDDL := `
		CREATE TABLE IF NOT EXISTS ` + metadataTable + `
		(
			partition_token STRING(MAX) NOT NULL,
			parent_tokens   ARRAY<STRING(MAX)>,
			start_timestamp TIMESTAMP NOT NULL,
			state           INT64     NOT NULL DEFAULT (0),
			watermark       TIMESTAMP NOT NULL,
			created_at      TIMESTAMP NOT NULL DEFAULT (CURRENT_TIMESTAMP()),
			scheduled_at    TIMESTAMP OPTIONS (allow_commit_timestamp = TRUE),
			running_at      TIMESTAMP OPTIONS (allow_commit_timestamp = TRUE),
			finished_at     TIMESTAMP OPTIONS (allow_commit_timestamp = TRUE),
		)
		PRIMARY KEY (partition_token), ROW DELETION POLICY (OLDER_THAN(finished_at, INTERVAL 7 DAY))
	`

	stateIndex := spannerutil.QuoteIdentifier(name + "_metadata_state")
	indexDDL := `
		CREATE INDEX IF NOT EXISTS ` + stateIndex + ` ON ` + metadataTable + `(state)
	`

	op, err := admin.UpdateDatabaseDdl(ctx, &databasepb.UpdateDatabaseDdlRequest{
		Database:   path,
		Statements: []string{metadataTableDDL, indexDDL},
	})
	if err != nil {
		return errs.Wrap(err)
	}

	err = op.Wait(ctx)
	return errs.Wrap(err)
}

// TestDeleteChangeStreamMetadata deletes the metadata table and index for the given change stream name.
func TestDeleteChangeStreamMetadata(ctx context.Context, admin *database.DatabaseAdminClient, path string, name string) error {
	stateIndex := spannerutil.QuoteIdentifier(name + "_metadata_state")
	dropIndexDDL := "DROP INDEX " + stateIndex

	metadataTable := spannerutil.QuoteIdentifier(name + "_metadata")
	dropTableDDL := "DROP TABLE " + metadataTable

	op, err := admin.UpdateDatabaseDdl(ctx, &databasepb.UpdateDatabaseDdlRequest{
		Database:   path,
		Statements: []string{dropIndexDDL, dropTableDDL},
	})
	if err != nil {
		return errs.Wrap(err)
	}

	err = op.Wait(ctx)
	return errs.Wrap(err)
}
