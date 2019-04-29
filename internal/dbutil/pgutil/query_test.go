// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pgutil_test

import (
	"testing"

	_ "github.com/jackc/pgx/stdlib"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/dbutil/dbschema"
	"storj.io/storj/internal/dbutil/pgutil"
	"storj.io/storj/internal/dbutil/pgutil/pgtest"
	"storj.io/storj/internal/testcontext"
)

const (
	// DefaultPostgresConn is a connstring that works with docker-compose
	DefaultPostgresConn = "postgres://storj:storj-pass@test-postgres/teststorj?sslmode=disable"
)

func TestQuery(t *testing.T) {
	if *pgtest.ConnStr == "" {
		t.Skip("Postgres flag missing, example: -postgres-test-db=" + DefaultPostgresConn)
	}

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	db, err := pgutil.Open(*pgtest.ConnStr, "pgutil-query")
	require.NoError(t, err)
	defer ctx.Check(db.Close)

	emptySchema, err := pgutil.QuerySchema(db)
	require.NoError(t, err)
	assert.Equal(t, &dbschema.Schema{}, emptySchema)

	_, err = db.Exec(`
		CREATE TABLE users (
			a integer NOT NULL,
			b integer NOT NULL,
			c text,
			UNIQUE (c),
			PRIMARY KEY (a)
		);
		CREATE TABLE names (
			users_a integer REFERENCES users( a ) ON DELETE CASCADE,
			a text NOT NULL,
			x text,
			b text,
			PRIMARY KEY (a, x),
			UNIQUE ( x ),
			UNIQUE ( a, b )
		);
	`)
	require.NoError(t, err)

	schema, err := pgutil.QuerySchema(db)
	require.NoError(t, err)

	expected := &dbschema.Schema{
		Tables: []*dbschema.Table{
			{
				Name: "users",
				Columns: []*dbschema.Column{
					{Name: "a", Type: "integer", IsNullable: false, Reference: nil},
					{Name: "b", Type: "integer", IsNullable: false, Reference: nil},
					{Name: "c", Type: "text", IsNullable: true, Reference: nil},
				},
				PrimaryKey: []string{"a"},
				Unique: [][]string{
					{"c"},
				},
			},
			{
				Name: "names",
				Columns: []*dbschema.Column{
					{Name: "users_a", Type: "integer", IsNullable: true,
						Reference: &dbschema.Reference{
							Table:    "users",
							Column:   "a",
							OnDelete: "CASCADE",
						}},
					{Name: "a", Type: "text", IsNullable: false, Reference: nil},
					{Name: "x", Type: "text", IsNullable: false, Reference: nil}, // not null, because primary key
					{Name: "b", Type: "text", IsNullable: true, Reference: nil},
				},
				PrimaryKey: []string{"a", "x"},
				Unique: [][]string{
					{"a", "b"},
					{"x"},
				},
			},
		},
	}

	expected.Sort()
	schema.Sort()
	assert.Equal(t, expected, schema)
}
