// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/spanner"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
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

// ChangeStreamAdapter provides methods for working with Spanner change streams.
type ChangeStreamAdapter interface {
	ChangeStream(ctx context.Context, name string, partitionToken string, from time.Time, callback func(record DataChangeRecord) error) ([]ChildPartitionsRecord, error)

	TestCreateChangeStream(ctx context.Context, name string) error
	TestDeleteChangeStream(ctx context.Context, name string) error
}

// ChangeFeedRecord represents a processed change feed record.
type ChangeFeedRecord struct {
	TableName      string
	OperationType  string
	CommitTime     time.Time
	RecordSequence string
	PrimaryKey     map[string]interface{}
	Data           map[string]interface{}
}

var _ ChangeStreamAdapter = &SpannerAdapter{}

// ChangeStream listens to Spanner change stream and processes records via callback.
func (s *SpannerAdapter) ChangeStream(ctx context.Context, name string, partitionToken string, from time.Time, callback func(record DataChangeRecord) error) ([]ChildPartitionsRecord, error) {
	s.log.Info("Listening on change stream", zap.String("name", name), zap.Time("from", from), zap.String("partition_token", partitionToken))

	query := `SELECT ChangeRecord FROM READ_%s(start_timestamp => @start_time,heartbeat_milliseconds => @heartbeat_milliseconds`

	params := map[string]interface{}{
		"start_time":             from,
		"heartbeat_milliseconds": 60000,
	}
	if partitionToken != "" {
		params["partition_token"] = partitionToken
		query += `, partition_token => @partition_token`
	}
	query += `)`

	stmt := spanner.Statement{
		SQL:    fmt.Sprintf(query, name),
		Params: params,
	}

	iter := s.client.Single().Query(ctx, stmt)

	var childPartitions []ChildPartitionsRecord
	err := iter.Do(func(row *spanner.Row) error {
		records := make([]*ChangeRecord, 0)
		err := row.Columns(&records)
		if err != nil {
			return errs.Wrap(err)
		}
		for _, record := range records {
			for _, dataChange := range record.DataChangeRecord {
				err := callback(*dataChange)
				if err != nil {
					return errs.Wrap(err)
				}
			}
			for _, partition := range record.ChildPartitionsRecord {
				childPartitions = append(childPartitions, *partition)
			}
		}
		return nil
	})
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return childPartitions, nil
}

// TestCreateChangeStream creates a change stream for testing purposes.
func (s *SpannerAdapter) TestCreateChangeStream(ctx context.Context, name string) error {
	ddlStatement := fmt.Sprintf(`
		CREATE CHANGE STREAM %s
		FOR objects
		OPTIONS (
			retention_period = "1d",
			value_capture_type = "NEW_ROW"
		)
	`, name)

	op, err := s.adminClient.UpdateDatabaseDdl(ctx, &databasepb.UpdateDatabaseDdlRequest{
		Database:   s.connParams.DatabasePath(),
		Statements: []string{ddlStatement},
	})
	if err != nil {
		return errs.Wrap(err)
	}

	err = op.Wait(ctx)
	return errs.Wrap(err)
}

// TestDeleteChangeStream deletes the change stream with the given name.
func (s *SpannerAdapter) TestDeleteChangeStream(ctx context.Context, name string) error {
	ddlStatement := fmt.Sprintf("DROP CHANGE STREAM %s", name)

	op, err := s.adminClient.UpdateDatabaseDdl(ctx, &databasepb.UpdateDatabaseDdlRequest{
		Database:   s.connParams.DatabasePath(),
		Statements: []string{ddlStatement},
	})
	if err != nil {
		return errs.Wrap(err)
	}

	err = op.Wait(ctx)
	return errs.Wrap(err)
}
