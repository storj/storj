// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pgutil

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/private/dbutil"
	"storj.io/storj/private/dbutil/dbschema"
	"storj.io/storj/private/dbutil/pgutil/pgerrcode"
	"storj.io/storj/private/tagsql"
)

var (
	mon = monkit.Package()
)

const (
	// pgErrorClassConstraintViolation is the class of PostgreSQL errors indicating
	// integrity constraint violations.
	pgErrorClassConstraintViolation = "23"
)

// OpenUnique opens a postgres database with a temporary unique schema, which will be cleaned up
// when closed. It is expected that this should normally be used by way of
// "storj.io/storj/private/dbutil/tempdb".OpenUnique() instead of calling it directly.
func OpenUnique(ctx context.Context, connstr string, schemaPrefix string) (*dbutil.TempDatabase, error) {
	// sanity check, because you get an unhelpful error message when this happens
	if strings.HasPrefix(connstr, "cockroach://") {
		return nil, errs.New("can't connect to cockroach using pgutil.OpenUnique()! connstr=%q. try tempdb.OpenUnique() instead?", connstr)
	}

	schemaName := schemaPrefix + "-" + CreateRandomTestingSchemaName(8)
	connStrWithSchema := ConnstrWithSchema(connstr, schemaName)

	db, err := tagsql.Open(ctx, "pgx", connStrWithSchema)
	if err == nil {
		// check that connection actually worked before trying CreateSchema, to make
		// troubleshooting (lots) easier
		err = db.PingContext(ctx)
	}
	if err != nil {
		return nil, errs.New("failed to connect to %q with driver pgx: %w", connStrWithSchema, err)
	}

	err = CreateSchema(ctx, db, schemaName)
	if err != nil {
		return nil, errs.Combine(err, db.Close())
	}

	cleanup := func(cleanupDB tagsql.DB) error {
		return DropSchema(ctx, cleanupDB, schemaName)
	}

	dbutil.Configure(ctx, db, "tmp_postgres", mon)
	return &dbutil.TempDatabase{
		DB:             db,
		ConnStr:        connStrWithSchema,
		Schema:         schemaName,
		Driver:         "pgx",
		Implementation: dbutil.Postgres,
		Cleanup:        cleanup,
	}, nil
}

// QuerySnapshot loads snapshot from database.
func QuerySnapshot(ctx context.Context, db dbschema.Queryer) (*dbschema.Snapshot, error) {
	schema, err := QuerySchema(ctx, db)
	if err != nil {
		return nil, err
	}

	data, err := QueryData(ctx, db, schema)
	if err != nil {
		return nil, err
	}

	return &dbschema.Snapshot{
		Version: -1,
		Schema:  schema,
		Data:    data,
	}, err
}

// CheckApplicationName ensures that the Connection String contains an application name.
func CheckApplicationName(s string) (r string) {
	if !strings.Contains(s, "application_name") {
		if !strings.Contains(s, "?") {
			r = s + "?application_name=Satellite"
			return
		}
		r = s + "&application_name=Satellite"
		return
	}
	// return source as is if application_name is set
	return s
}

// IsConstraintError checks if given error is about constraint violation.
func IsConstraintError(err error) bool {
	errCode := pgerrcode.FromError(err)
	return strings.HasPrefix(errCode, pgErrorClassConstraintViolation)
}

// The following XArray() helper methods exist alongside similar methods in the
// jackc/pgtype library. The difference with the methods in pgtype is that they
// will accept any of a wide range of types. That is nice, but it comes with
// the potential that someone might pass in an invalid type; thus, those
// methods have to return (*pgtype.XArray, error).
//
// The methods here do not need to return an error because they require passing
// in the correct type to begin with.
//
// An alternative implementation for the following methods might look like
// calls to pgtype.ByteaArray() followed by `if err != nil { panic }` blocks.
// That would probably be ok, but we decided on this approach, as it ought to
// require fewer allocations and less time, in addition to having no error
// return.

// ByteaArray returns an object usable by pg drivers for passing a [][]byte slice
// into a database as type BYTEA[].
func ByteaArray(bytesArray [][]byte) *pgtype.ByteaArray {
	pgtypeByteaArray := make([]pgtype.Bytea, len(bytesArray))
	for i, byteSlice := range bytesArray {
		pgtypeByteaArray[i].Bytes = byteSlice
		pgtypeByteaArray[i].Status = pgtype.Present
	}
	return &pgtype.ByteaArray{
		Elements:   pgtypeByteaArray,
		Dimensions: []pgtype.ArrayDimension{{Length: int32(len(bytesArray)), LowerBound: 1}},
		Status:     pgtype.Present,
	}
}

// TextArray returns an object usable by pg drivers for passing a []string slice
// into a database as type TEXT[].
func TextArray(stringSlice []string) *pgtype.TextArray {
	pgtypeTextArray := make([]pgtype.Text, len(stringSlice))
	for i, s := range stringSlice {
		pgtypeTextArray[i].String = s
		pgtypeTextArray[i].Status = pgtype.Present
	}
	return &pgtype.TextArray{
		Elements:   pgtypeTextArray,
		Dimensions: []pgtype.ArrayDimension{{Length: int32(len(stringSlice)), LowerBound: 1}},
		Status:     pgtype.Present,
	}
}

// TimestampTZArray returns an object usable by pg drivers for passing a []time.Time
// slice into a database as type TIMESTAMPTZ[].
func TimestampTZArray(timeSlice []time.Time) *pgtype.TimestamptzArray {
	pgtypeTimestamptzArray := make([]pgtype.Timestamptz, len(timeSlice))
	for i, t := range timeSlice {
		pgtypeTimestamptzArray[i].Time = t
		pgtypeTimestamptzArray[i].Status = pgtype.Present
	}
	return &pgtype.TimestamptzArray{
		Elements:   pgtypeTimestamptzArray,
		Dimensions: []pgtype.ArrayDimension{{Length: int32(len(timeSlice)), LowerBound: 1}},
		Status:     pgtype.Present,
	}
}

// Int4Array returns an object usable by pg drivers for passing a []int32 slice
// into a database as type INT4[].
func Int4Array(ints []int32) *pgtype.Int4Array {
	pgtypeInt4Array := make([]pgtype.Int4, len(ints))
	for i, someInt := range ints {
		pgtypeInt4Array[i].Int = someInt
		pgtypeInt4Array[i].Status = pgtype.Present
	}
	return &pgtype.Int4Array{
		Elements:   pgtypeInt4Array,
		Dimensions: []pgtype.ArrayDimension{{Length: int32(len(ints)), LowerBound: 1}},
		Status:     pgtype.Present,
	}
}

// Int8Array returns an object usable by pg drivers for passing a []int64 slice
// into a database as type INT8[].
func Int8Array(bigInts []int64) *pgtype.Int8Array {
	pgtypeInt8Array := make([]pgtype.Int8, len(bigInts))
	for i, bigInt := range bigInts {
		pgtypeInt8Array[i].Int = bigInt
		pgtypeInt8Array[i].Status = pgtype.Present
	}
	return &pgtype.Int8Array{
		Elements:   pgtypeInt8Array,
		Dimensions: []pgtype.ArrayDimension{{Length: int32(len(bigInts)), LowerBound: 1}},
		Status:     pgtype.Present,
	}
}

// Float8Array returns an object usable by pg drivers for passing a []float64 slice
// into a database as type FLOAT8[].
func Float8Array(floats []float64) *pgtype.Float8Array {
	pgtypeFloat8Array := make([]pgtype.Float8, len(floats))
	for i, someFloat := range floats {
		pgtypeFloat8Array[i].Float = someFloat
		pgtypeFloat8Array[i].Status = pgtype.Present
	}
	return &pgtype.Float8Array{
		Elements:   pgtypeFloat8Array,
		Dimensions: []pgtype.ArrayDimension{{Length: int32(len(floats)), LowerBound: 1}},
		Status:     pgtype.Present,
	}
}

// NodeIDArray returns an object usable by pg drivers for passing a []storj.NodeID
// slice into a database as type BYTEA[].
func NodeIDArray(nodeIDs []storj.NodeID) *pgtype.ByteaArray {
	if nodeIDs == nil {
		return &pgtype.ByteaArray{Status: pgtype.Null}
	}
	pgtypeByteaArray := make([]pgtype.Bytea, len(nodeIDs))
	for i, nodeID := range nodeIDs {
		nodeIDCopy := nodeID
		pgtypeByteaArray[i].Bytes = nodeIDCopy[:]
		pgtypeByteaArray[i].Status = pgtype.Present
	}
	return &pgtype.ByteaArray{
		Elements:   pgtypeByteaArray,
		Dimensions: []pgtype.ArrayDimension{{Length: int32(len(nodeIDs)), LowerBound: 1}},
		Status:     pgtype.Present,
	}
}

// QuoteIdentifier quotes an identifier for use in an interpolated SQL string.
func QuoteIdentifier(ident string) string {
	return pgx.Identifier{ident}.Sanitize()
}

// UnquoteIdentifier is the analog of QuoteIdentifier.
func UnquoteIdentifier(quotedIdent string) string {
	if len(quotedIdent) >= 2 && quotedIdent[0] == '"' && quotedIdent[len(quotedIdent)-1] == '"' {
		quotedIdent = strings.ReplaceAll(quotedIdent[1:len(quotedIdent)-1], "\"\"", "\"")
	}
	return quotedIdent
}
