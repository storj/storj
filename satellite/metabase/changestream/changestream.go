// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package changestream

import (
	"context"
	"time"

	"cloud.google.com/go/spanner"
	database "cloud.google.com/go/spanner/admin/database/apiv1"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"

	"storj.io/storj/shared/dbutil/recordeddb"
	"storj.io/storj/shared/dbutil/spannerutil"
)

// ChangeRecord represents a record from Spanner change stream.
type ChangeRecord struct {
	DataChangeRecord      []*DataChangeRecord      `spanner:"data_change_record"`
	HeartbeatRecord       []*HeartbeatRecord       `spanner:"heartbeat_record"`
	ChildPartitionsRecord []*ChildPartitionsRecord `spanner:"child_partitions_record"`
}

// ColumnType describes column metadata in change stream records.
type ColumnType struct {
	Name            string           `spanner:"name"`
	CodeType        spanner.NullJSON `spanner:"type"`
	IsPrimaryKey    bool             `spanner:"is_primary_key"`
	OrdinalPosition int64            `spanner:"ordinal_position"`
}

// TypeCode represents a column type code from Spanner.
type TypeCode struct {
	Code spanner.NullJSON `spanner:"code"`
}

// Mods contains row modification data from change stream.
type Mods struct {
	Keys      spanner.NullJSON `spanner:"keys"`
	NewValues spanner.NullJSON `spanner:"new_values"`
	OldValues spanner.NullJSON `spanner:"old_values"`
}

// DataChangeRecord represents a data change in the change stream.
type DataChangeRecord struct {
	CommitTimestamp                      time.Time     `spanner:"commit_timestamp"`
	RecordSequence                       string        `spanner:"record_sequence"`
	ServerTransactionId                  string        `spanner:"server_transaction_id"`
	IsLastRecordInTransactionInPartition bool          `spanner:"is_last_record_in_transaction_in_partition"`
	TableName                            string        `spanner:"table_name"`
	ColumnTypes                          []*ColumnType `spanner:"column_types"`
	Mods                                 []*Mods       `spanner:"mods"`
	ModType                              string        `spanner:"mod_type"`
	ValueCaptureType                     string        `spanner:"value_capture_type"`
	NumberOfRecordsInTransaction         int64         `spanner:"number_of_records_in_transaction"`
	NumberOfPartitionsInTransaction      int64         `spanner:"number_of_partitions_in_transaction"`
	TransactionTag                       string        `spanner:"transaction_tag"`
	IsSystemTransaction                  bool          `spanner:"is_system_transaction"`
}

// HeartbeatRecord represents a heartbeat record in the change stream.
type HeartbeatRecord struct {
	Timestamp time.Time `spanner:"timestamp"`
}

// ChildPartition represents a child partition in Spanner change stream.
type ChildPartition struct {
	Token                 string   `spanner:"token"`
	ParentPartitionTokens []string `spanner:"parent_partition_tokens"`
}

// ChildPartitionsRecord contains information about child partitions.
type ChildPartitionsRecord struct {
	StartTimestamp  time.Time         `spanner:"start_timestamp"`
	RecordSequence  string            `spanner:"record_sequence"`
	ChildPartitions []*ChildPartition `spanner:"child_partitions"`
}

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
	v, ok := val.(int64)
	if !ok {
		return errs.New("failed to decode PartitionState: expected int64, got %T", val)
	}

	state := PartitionState(v)
	if !state.Valid() {
		return errs.New("invalid PartitionState value in database: %d", v)
	}

	*s = state

	return nil
}

// Adapter provides methods for working with Spanner change streams.
type Adapter interface {
	ReadChangeStreamPartition(ctx context.Context, name string, partitionToken string, from time.Time, callback func(record ChangeRecord) error) error
	ChangeStreamNoPartitionMetadata(ctx context.Context, feedName string) (bool, error)
	GetChangeStreamPartitionsByState(ctx context.Context, name string, state PartitionState) (map[string]time.Time, error)
	AddChangeStreamPartition(ctx context.Context, feedName, childToken string, parentTokens []string, start time.Time) error
	ScheduleChangeStreamPartitions(ctx context.Context, feedName string) (int64, error)
	UpdateChangeStreamPartitionWatermark(ctx context.Context, feedName, partitionToken string, newWatermark time.Time) error
	UpdateChangeStreamPartitionState(ctx context.Context, feedName, partitionToken string, newState PartitionState) error

	TestCreateChangeStream(ctx context.Context, name string) error
	TestDeleteChangeStream(ctx context.Context, name string) error
}

// ReadPartition listens to Spanner change stream and processes records via callback.
func ReadPartition(ctx context.Context, log *zap.Logger, client *recordeddb.SpannerClient, name string, partitionToken string, from time.Time, callback func(record ChangeRecord) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	log.Info("Read partition", zap.String("Change Stream", name), zap.Time("From", from), zap.String("Partition Token", partitionToken))

	changeStream := spannerutil.QuoteIdentifier("READ_" + name)

	stmt := spanner.Statement{
		SQL: `
			SELECT ChangeRecord
			FROM ` + changeStream + ` (
				start_timestamp => @start_time,
				partition_token => @partition_token,
				heartbeat_milliseconds => @heartbeat_milliseconds
			)`,
		Params: map[string]interface{}{
			"start_time": from,
			"partition_token": spanner.NullString{
				StringVal: partitionToken,
				Valid:     partitionToken != "",
			},
			"heartbeat_milliseconds": 60000,
		},
	}

	err = client.Single().QueryWithOptions(ctx, stmt,
		spanner.QueryOptions{RequestTag: "change-stream-read-partition"},
	).Do(func(row *spanner.Row) error {
		records := make([]*ChangeRecord, 0)
		err := row.Columns(&records)
		if err != nil {
			return errs.Wrap(err)
		}
		for _, record := range records {
			if err := callback(*record); err != nil {
				return errs.Wrap(err)
			}
		}
		return nil
	})

	return errs.Wrap(err)
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
			SET state = ` + stateScheduled + `
			WHERE child.state = ` + stateCreated + `
			AND (
				child.partition_token = ''
				OR (
					ARRAY_LENGTH(child.parent_tokens) = 0
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
	var setClause string
	switch newState {
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

// TestCreateChangeStream creates a change stream for testing purposes.
func TestCreateChangeStream(ctx context.Context, admin *database.DatabaseAdminClient, path string, name string) error {
	changeStream := spannerutil.QuoteIdentifier(name)
	changeStreamDDL := `
		CREATE CHANGE STREAM ` + changeStream + `
		FOR objects (stream_id, status, total_plain_size)
		OPTIONS (
			value_capture_type = 'NEW_ROW_AND_OLD_VALUES',
			exclude_ttl_deletes = TRUE
		)
	`

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
		Statements: []string{changeStreamDDL, metadataTableDDL, indexDDL},
	})
	if err != nil {
		return errs.Wrap(err)
	}

	err = op.Wait(ctx)
	return errs.Wrap(err)
}

// TestDeleteChangeStream deletes the change stream with the given name.
func TestDeleteChangeStream(ctx context.Context, admin *database.DatabaseAdminClient, path string, name string) error {
	changeStream := spannerutil.QuoteIdentifier(name)
	dropChangeStreamDDL := "DROP CHANGE STREAM " + changeStream

	stateIndex := spannerutil.QuoteIdentifier(name + "_metadata_state")
	dropIndexDDL := "DROP INDEX " + stateIndex

	metadataTable := spannerutil.QuoteIdentifier(name + "_metadata")
	dropTableDDL := "DROP TABLE " + metadataTable

	// Retry with exponential backoff to handle transient conflicts with active streaming queries
	const maxRetries = 5
	backoff := 100 * time.Millisecond

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return errs.Wrap(ctx.Err())
			case <-time.After(backoff):
				backoff *= 2
			}
		}

		op, err := admin.UpdateDatabaseDdl(ctx, &databasepb.UpdateDatabaseDdlRequest{
			Database:   path,
			Statements: []string{dropChangeStreamDDL, dropIndexDDL, dropTableDDL},
		})
		if err != nil {
			lastErr = err
			// Check if it's a FailedPrecondition error (concurrent operation)
			if spanner.ErrCode(err) == codes.FailedPrecondition {
				continue
			}
			return errs.Wrap(err)
		}

		err = op.Wait(ctx)
		if err != nil {
			lastErr = err
			// Check if it's a FailedPrecondition error (concurrent operation)
			if spanner.ErrCode(err) == codes.FailedPrecondition {
				continue
			}
			return errs.Wrap(err)
		}

		return nil
	}

	return errs.Wrap(lastErr)
}
