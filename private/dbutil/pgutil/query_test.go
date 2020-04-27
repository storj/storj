// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pgutil_test

import (
	"testing"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/dbutil"
	"storj.io/storj/private/dbutil/dbschema"
	"storj.io/storj/private/dbutil/pgtest"
	"storj.io/storj/private/dbutil/pgutil"
	"storj.io/storj/private/dbutil/tempdb"
)

func TestQuery(t *testing.T) {
	pgtest.Run(t, func(ctx *testcontext.Context, t *testing.T, connstr string) {
		db, err := tempdb.OpenUnique(ctx, connstr, "pgutil-query")
		require.NoError(t, err)
		defer ctx.Check(db.Close)

		emptySchema, err := pgutil.QuerySchema(ctx, db)
		require.NoError(t, err)
		assert.Equal(t, &dbschema.Schema{}, emptySchema)

		_, err = db.ExecContext(ctx, `
			CREATE TABLE users (
				a bigint NOT NULL,
				b bigint NOT NULL,
				c text,
				UNIQUE (c),
				PRIMARY KEY (a)
			);
			CREATE TABLE names (
				users_a bigint REFERENCES users( a ) ON DELETE CASCADE,
				a text NOT NULL,
				x text,
				b text,
				PRIMARY KEY (a, x),
				UNIQUE ( x ),
				UNIQUE ( a, b )
			);
		`)
		require.NoError(t, err)

		schema, err := pgutil.QuerySchema(ctx, db)
		require.NoError(t, err)

		expected := &dbschema.Schema{
			Tables: []*dbschema.Table{
				{
					Name: "users",
					Columns: []*dbschema.Column{
						{Name: "a", Type: "bigint", IsNullable: false, Reference: nil},
						{Name: "b", Type: "bigint", IsNullable: false, Reference: nil},
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
						{Name: "users_a", Type: "bigint", IsNullable: true,
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
				{Name: "names_a_b_key", Table: "names", Columns: []string{"a", "b"}, Unique: true, Partial: ""},
				{Name: "names_pkey", Table: "names", Columns: []string{"a", "x"}, Unique: true, Partial: ""},
				{Name: "names_x_key", Table: "names", Columns: []string{"x"}, Unique: true, Partial: ""},
				{Name: "users_c_key", Table: "users", Columns: []string{"c"}, Unique: true, Partial: ""},
				{Name: "users_pkey", Table: "users", Columns: []string{"a"}, Unique: true, Partial: ""},
			},
		}

		if db.Implementation == dbutil.Cockroach {
			expected.Indexes = append(expected.Indexes, &dbschema.Index{
				Name:    "names_auto_index_fk_users_a_ref_users",
				Table:   "names",
				Columns: []string{"users_a"},
			})
		}

		expected.Sort()
		schema.Sort()
		assert.Equal(t, expected, schema)
	})
}
