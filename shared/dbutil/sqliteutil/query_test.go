// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package sqliteutil_test

import (
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/shared/dbutil/dbschema"
	"storj.io/storj/shared/dbutil/sqliteutil"
	"storj.io/storj/shared/tagsql"
)

func TestQuery(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	db, err := tagsql.Open(ctx, "sqlite3", ":memory:", nil)
	require.NoError(t, err)

	defer ctx.Check(db.Close)

	emptySchema, err := sqliteutil.QuerySchema(ctx, db)
	assert.NoError(t, err)
	assert.Equal(t, &dbschema.Schema{}, emptySchema)

	_, err = db.ExecContext(ctx, `
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

	schema, err := sqliteutil.QuerySchema(ctx, db)
	assert.NoError(t, err)

	expected := &dbschema.Schema{
		Tables: []*dbschema.Table{
			{
				Name: "users",
				Columns: []*dbschema.Column{
					{Name: "a", Type: "INTEGER", IsNullable: false},
					{Name: "b", Type: "INTEGER", IsNullable: false},
					{Name: "c", Type: "TEXT", IsNullable: true},
				},
				PrimaryKey: []string{"a"},
				Unique: [][]string{
					{"c"},
				},
			},
			{
				Name: "names",
				Columns: []*dbschema.Column{
					{Name: "users_a", Type: "INTEGER", IsNullable: true},
					{Name: "a", Type: "TEXT", IsNullable: false},
					{Name: "x", Type: "TEXT", IsNullable: false}, // not null, because primary key
					{Name: "b", Type: "TEXT", IsNullable: true},
				},
				PrimaryKey: []string{"a", "x"},
				Unique: [][]string{
					{"a", "b"},
					{"x"},
				},
				ForeignKeys: []*dbschema.ForeignKey{
					{
						Name:           "fk_0",
						LocalColumns:   []string{"users_a"},
						ForeignTable:   "users",
						ForeignColumns: []string{"a"},
						OnDelete:       "CASCADE",
						OnUpdate:       "",
					},
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
