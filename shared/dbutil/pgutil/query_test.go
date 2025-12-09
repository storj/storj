// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pgutil_test

import (
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/dbschema"
	"storj.io/storj/shared/dbutil/dbtest"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/dbutil/tempdb"
)

func TestQuery(t *testing.T) {
	dbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, connstr string) {
		db, err := tempdb.OpenUnique(ctx, zaptest.NewLogger(t), connstr, "pgutil-query", nil)
		require.NoError(t, err)
		defer ctx.Check(db.Close)

		emptySchema, err := pgutil.QuerySchema(ctx, db)
		require.NoError(t, err)
		assert.Equal(t, &dbschema.Schema{}, emptySchema)

		if db.Implementation != dbutil.Cockroach {
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
				c bigint,
				PRIMARY KEY (a, x),
				UNIQUE ( x ),
				UNIQUE ( a, b ),
				CONSTRAINT x_not_b CHECK (x != b)
			);
			CREATE INDEX users_a_b_c ON users (a, b DESC, c NULLS LAST) WHERE b > 10 AND c != '';
			CREATE INDEX names_a_x ON names (a ASC, x NULLS FIRST) WHERE b != '';
			CREATE INDEX names_b ON names (b);
			CREATE SEQUENCE node_alias_seq
				INCREMENT BY 1
				MINVALUE 1 MAXVALUE 2147483647 -- MaxInt32
				START WITH 1;
			`)
		} else {
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
				c bigint,
				PRIMARY KEY (a, x),
				UNIQUE ( x ),
				UNIQUE ( a, b ),
				CONSTRAINT x_not_b CHECK (x <> b)
			);
			CREATE INDEX users_a_b_c ON users (a, b DESC, c NULLS FIRST) WHERE b > 10 AND c != '';
			CREATE INDEX names_a_x ON names (a ASC, x NULLS FIRST) STORING(b, c) WHERE b != '';
			CREATE INDEX names_b ON names (b) STORING (c);
			CREATE SEQUENCE node_alias_seq
				INCREMENT BY 1
				MINVALUE 1 MAXVALUE 2147483647 -- MaxInt32
				START WITH 1;
			`)
		}

		require.NoError(t, err)

		schema, err := pgutil.QuerySchema(ctx, db)
		require.NoError(t, err)

		expected := &dbschema.Schema{
			Tables: []*dbschema.Table{
				{
					Name: "users",
					Columns: []*dbschema.Column{
						{Name: "a", Type: "bigint", IsNullable: false},
						{Name: "b", Type: "bigint", IsNullable: false},
						{Name: "c", Type: "text", IsNullable: true},
					},
					PrimaryKey: []string{"a"},
					Unique: [][]string{
						{"c"},
					},
				},
				{
					Name: "names",
					Columns: []*dbschema.Column{
						{
							Name: "users_a", Type: "bigint", IsNullable: true,
						},
						{Name: "a", Type: "text", IsNullable: false},
						{Name: "x", Type: "text", IsNullable: false}, // not null, because primary key
						{Name: "b", Type: "text", IsNullable: true},
						{Name: "c", Type: "bigint", IsNullable: true},
					},
					PrimaryKey: []string{"a", "x"},
					Unique: [][]string{
						{"a", "b"},
						{"x"},
					},
					ForeignKeys: []*dbschema.ForeignKey{
						{
							Name:           "names_users_a_fkey",
							LocalColumns:   []string{"users_a"},
							ForeignTable:   "users",
							ForeignColumns: []string{"a"},
							OnDelete:       "CASCADE",
							OnUpdate:       "",
						},
					},
					Checks: []string{
						"CHECK ((x <> b))",
					},
				},
			},
			Indexes: []*dbschema.Index{
				{Name: "names_a_b_key", Table: "names", Columns: []string{"a", "b"}, Unique: true, Partial: ""},
				{Name: "names_pkey", Table: "names", Columns: []string{"a", "x"}, Unique: true, Partial: ""},
				{Name: "names_x_key", Table: "names", Columns: []string{"x"}, Unique: true, Partial: ""},
				{Name: "users_c_key", Table: "users", Columns: []string{"c"}, Unique: true, Partial: ""},
				{Name: "users_pkey", Table: "users", Columns: []string{"a"}, Unique: true, Partial: ""},
				{Name: "names_b", Table: "names", Columns: []string{"b"}, Unique: false, Partial: ""},
			},
			Sequences: []string{"node_alias_seq"},
		}

		if db.Implementation != dbutil.Cockroach {
			expected.Indexes = append(expected.Indexes,
				&dbschema.Index{Name: "users_a_b_c", Table: "users", Columns: []string{"a", "b", "c"}, Unique: false, Partial: "((b > 10) AND (c <> ''::text))"},
				&dbschema.Index{Name: "names_a_x", Table: "names", Columns: []string{"a", "x"}, Unique: false, Partial: "(b <> ''::text)"},
			)
		} else {
			expected.Indexes = append(expected.Indexes,
				&dbschema.Index{Name: "users_a_b_c", Table: "users", Columns: []string{"a", "b", "c"}, Unique: false, Partial: "((b > 10) AND (c != ''::STRING))"},
				&dbschema.Index{Name: "names_a_x", Table: "names", Columns: []string{"a", "x"}, Unique: false, Partial: "(b != ''::STRING)"},
			)
		}

		expected.Sort()
		schema.Sort()
		assert.Equal(t, expected, schema)
	})
}
