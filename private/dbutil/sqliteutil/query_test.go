// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package sqliteutil_test

import (
	"database/sql"
	"testing"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/dbutil/dbschema"
	"storj.io/storj/private/dbutil/sqliteutil"
)

func TestQuery(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	defer ctx.Check(db.Close)

	emptySchema, err := sqliteutil.QuerySchema(db)
	assert.NoError(t, err)
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
		CREATE INDEX names_a ON names (a, b);
	`)

	require.NoError(t, err)

	schema, err := sqliteutil.QuerySchema(db)
	assert.NoError(t, err)

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
		Indexes: []*dbschema.Index{
			{
				Name:    "names_a",
				Table:   "names",
				Columns: []string{"a", "b"},
			},
		},
	}

	expected.Sort()
	schema.Sort()
	assert.Equal(t, expected, schema)
}
