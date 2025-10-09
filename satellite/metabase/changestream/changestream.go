// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package changestream

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/spanner"
	database "cloud.google.com/go/spanner/admin/database/apiv1"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/shared/dbutil/recordeddb"
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

// ReadPartitions listens to Spanner change stream and processes records via callback.
func ReadPartitions(ctx context.Context, log *zap.Logger, client *recordeddb.SpannerClient, name string, partitionToken string, from time.Time, callback func(record DataChangeRecord) error) (childPartitions []ChildPartitionsRecord, err error) {
	defer mon.Task()(&ctx)(&err)
	log.Info("Listening on change stream", zap.String("name", name), zap.Time("from", from), zap.String("partition_token", partitionToken))

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

	iter := client.Single().Query(ctx, stmt)

	err = iter.Do(func(row *spanner.Row) error {
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
				for _, child := range partition.ChildPartitions {
					log.Debug("Received child partition",
						zap.Time("start_timestamp", partition.StartTimestamp),
						zap.String("record_sequence", partition.RecordSequence),
						zap.String("token", partitionToken),
						zap.String("child_token", child.Token),
						zap.Strings("parent_token", child.ParentPartitionTokens))
				}
				childPartitions = append(childPartitions, *partition)
			}
			for _, hb := range record.HeartbeatRecord {
				log.Debug("Received heartbeat", zap.String("token", partitionToken), zap.Time("timestamp", hb.Timestamp))
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
func TestCreateChangeStream(ctx context.Context, admin *database.DatabaseAdminClient, path string, name string) error {
	ddlStatement := fmt.Sprintf(`
		CREATE CHANGE STREAM %s
		FOR objects
		OPTIONS (
			retention_period = "1d",
			value_capture_type = "NEW_ROW"
		)
	`, name)

	op, err := admin.UpdateDatabaseDdl(ctx, &databasepb.UpdateDatabaseDdlRequest{
		Database:   path,
		Statements: []string{ddlStatement},
	})
	if err != nil {
		return errs.Wrap(err)
	}

	err = op.Wait(ctx)
	return errs.Wrap(err)
}

// TestDeleteChangeStream deletes the change stream with the given name.
func TestDeleteChangeStream(ctx context.Context, admin *database.DatabaseAdminClient, path string, name string) error {
	ddlStatement := fmt.Sprintf("DROP CHANGE STREAM %s", name)

	op, err := admin.UpdateDatabaseDdl(ctx, &databasepb.UpdateDatabaseDdlRequest{
		Database:   path,
		Statements: []string{ddlStatement},
	})
	if err != nil {
		return errs.Wrap(err)
	}

	err = op.Wait(ctx)
	return errs.Wrap(err)
}
