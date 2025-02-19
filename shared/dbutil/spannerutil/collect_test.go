// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package spannerutil_test

import (
	"testing"

	"cloud.google.com/go/spanner"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/common/testcontext"
	"storj.io/storj/shared/dbutil/spannertest"
	"storj.io/storj/shared/dbutil/spannerutil"
)

var Error = errs.Class("spannerutil")

const testSchema = `
	CREATE TABLE users (
		username STRING(64) NOT NULL
	) PRIMARY KEY (username)
`

func TestCollectRows(t *testing.T) {
	ctx := testcontext.New(t)
	spannertest.RunClient(ctx, t, testSchema, func(t *testing.T, client *spanner.Client) {
		{
			items, err := spannerutil.CollectRows(
				client.Single().Query(ctx, spanner.Statement{
					SQL: "SELECT username FROM users",
				}),
				func(row *spanner.Row, item *string) error {
					return row.Columns(item)
				})
			require.Empty(t, items)
			require.NoError(t, err)
		}

		{
			_, err := client.Apply(ctx, []*spanner.Mutation{
				spanner.Insert("users",
					[]string{"username"},
					[]any{"alice"},
				),
				spanner.Insert("users",
					[]string{"username"},
					[]any{"bob"},
				),
			})
			require.NoError(t, err)
		}

		{
			items, err := spannerutil.CollectRows(
				client.Single().Query(ctx, spanner.Statement{
					SQL: "SELECT username FROM users ORDER BY username",
				}),
				func(row *spanner.Row, item *string) error {
					return row.Columns(item)
				})
			require.EqualValues(t, []string{"alice", "bob"}, items)
			require.NoError(t, err)
		}
	})
}

func TestCollectRow(t *testing.T) {
	ctx := testcontext.New(t)
	spannertest.RunClient(ctx, t, testSchema, func(t *testing.T, client *spanner.Client) {
		{
			item, err := spannerutil.CollectRow(
				client.Single().Query(ctx, spanner.Statement{
					SQL: "SELECT username FROM users",
				}),
				func(row *spanner.Row, item *string) error {
					return row.Columns(item)
				})
			require.Empty(t, item)
			require.Error(t, err)
		}

		{
			_, err := client.Apply(ctx, []*spanner.Mutation{
				spanner.Insert("users",
					[]string{"username"},
					[]any{"alice"},
				),
				spanner.Insert("users",
					[]string{"username"},
					[]any{"bob"},
				),
			})
			require.NoError(t, err)
		}

		{
			item, err := spannerutil.CollectRow(
				client.Single().Query(ctx, spanner.Statement{
					SQL: "SELECT username FROM users ORDER BY username LIMIT 1",
				}),
				func(row *spanner.Row, item *string) error {
					return row.Columns(item)
				})
			require.Equal(t, "alice", item)
			require.NoError(t, err)
		}

		{
			_, err := spannerutil.CollectRow(
				client.Single().Query(ctx, spanner.Statement{
					SQL: "SELECT username FROM users ORDER BY username",
				}),
				func(row *spanner.Row, item *string) error {
					return row.Columns(item)
				})
			require.Error(t, err)
			require.True(t, spannerutil.ErrMultipleRows.Has(err))
		}
	})
}
