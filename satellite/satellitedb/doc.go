// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

/*
Package satellitedb implements database layer for satellite operations.

# Notes about Spanner

# UPDATE OR INSERT is not supported

Spanner does not support `INSERT INTO ... ON CONFLICT DO UPDATE SET`.
It also does not support `UPDATE` with joins.

One way to get the same effect is to use two statements:

1. UPDATE table SET value = value + @value WHERE id = @id
2. INSERT OR IGNORE INTO table (id, value) VALUES (@id, @value)

As an example:

	err := spannerutil.UnderlyingClient(ctx, db.db, func(client *spanner.Client) (err error) {
		defer mon.Task()(&ctx)(&err)

		_, err = client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, txn *spanner.ReadWriteTransaction) error {
			params := map[string]any{
				"id":    id,
				"count": count,
			}
			_, err := txn.BatchUpdateWithOptions(ctx, []spanner.Statement{
					{
						SQL: `UPDATE table SET count = count + @count WHERE id = @id`,
						Params: params,
					},
					{
						SQL: `INSERT OR IGNORE INTO table (id, count) VALUES (@id, @count)`,
						Params: params,
					},
				}, spanner.QueryOptions{RequestTag: "example"})
			return err
		}, spanner.TransactionOptions{TransactionTag: "example"})
		return err
	})

It's guaranteed that only one of those statements succeed, this approach also works in a single roundtrip.
*/
package satellitedb
