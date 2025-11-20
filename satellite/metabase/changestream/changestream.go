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
	TestCreateChangeStreamMetadata(ctx context.Context, name string) error
	TestDeleteChangeStreamMetadata(ctx context.Context, name string) error
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

// TestCreateChangeStream creates a change stream for testing purposes.
func TestCreateChangeStream(ctx context.Context, admin *database.DatabaseAdminClient, path string, name string) error {
	// Create metadata table and index first
	err := TestCreateChangeStreamMetadata(ctx, admin, path, name)
	if err != nil {
		return err
	}

	changeStream := spannerutil.QuoteIdentifier(name)
	changeStreamDDL := `
		CREATE CHANGE STREAM ` + changeStream + `
		FOR objects (stream_id, status, total_plain_size)
		OPTIONS (
			value_capture_type = 'NEW_ROW_AND_OLD_VALUES',
			exclude_ttl_deletes = TRUE
		)
	`

	op, err := admin.UpdateDatabaseDdl(ctx, &databasepb.UpdateDatabaseDdlRequest{
		Database:   path,
		Statements: []string{changeStreamDDL},
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

	// Retry with exponential backoff to handle transient conflicts with active streaming queries
	const maxRetries = 7
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
			Statements: []string{dropChangeStreamDDL},
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

		// Successfully dropped change stream, now delete metadata table and index
		return TestDeleteChangeStreamMetadata(ctx, admin, path, name)
	}

	return errs.Wrap(lastErr)
}
