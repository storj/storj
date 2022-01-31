# Migrations and You

This document is best read in the voice of one of those 50s instructional videos. You know the ones, like [this one](https://www.youtube.com/watch?v=NjduGaMZvR4). If you want a sound track to go with it, I would recommend [playing this in the background](https://www.youtube.com/watch?v=IvB4ZO3e61A0).

---

## Mechanics

In order to understand the right way to add a new migration with associated tests, a basic understanding of how the tests run and what they check is required. The migration tests work by having

1. A single database kept for the duration of the test that the individual migrations are run on one at a time in order
2. An expected snapshot for the database for every single step
3. A sql file for each snapshot to generate it

For a given step, what happens is

1. The `-- OLD DATA --` section is just run on the database being migrated
2. The main section and `-- NEW DATA --` sections are run on the snapshot
3. The migration is run against the main database
4. The `-- NEW DATA --` section is run on the database

Then, the snapshot and current database state are compared for schema and data differences.

## How to Create a Migration

With that basic overview out of the way, the steps to create a new migration are

1. Create an empty `postgres.vN.sql` file in this folder.
2. Copy the `satellitedb.dbx.pgx.sql` file from the `satellitedb/dbx` folder. This ensures that the snapshot does not drift from the dbx sql file. We bootstrap tests from the dbx output, so the correctness of our tests depends on them matching.
3. Copy the `INSERT` statements from the end of the previous migration. These lines are after the `CREATE INDEX` lines but before any `-- NEW DATA --` or `-- OLD DATA --` section.
4. Copy the `-- NEW DATA --` statements from the end of the previous migration into the main section. They are no longer `-- NEW DATA --`. You should only need to copy `INSERT` statements.

Depending on what your migration is doing, you should then do one of these:

1. If your migration is creating new tables, add an `INSERT` for it in the `-- NEW DATA --` section. This ensures that future migrations don't break data in that table. Our test coverage depends on these `INSERT`s existing and being complete. Help avoid problems in production, now. Smokey the Bear says only you can prevent production fires.
2. If you are updating data in old tables or changing the table schema
    a. and there is previous data (which there should be, by point 1), change the existing row in the main section to be the new updated value.
    b. and there is no previous data, add an insert into the `-- OLD DATA --` section as well as a corresponding updated row like in point a. Unfortunately, someone got away with adding a table without adding a row, but no longer. The tests must forever grow.
3. Anything more complicated, look at the mechanics section again to see if there's a way to solve your problem. If it is unclear (it probably is), just ask for help. Maybe the brains of multiple people can figure out something to do.

## Best Practices and Common Mistakes

1. Don't do in two migrations what can be done in one. The more migrations we have, the longer test times are, and the more often we may have to collapse them. Unless you have a good reason to do two migrations, just do one.

2. There is almost no reason to have an `UPDATE` statement in a sql snapshot file. Each snapshot is run independently, so you should just adjust the `INSERT` statement to reflect the rows you expect to exist, and leave the `UPDATE` to the migration. This helps test that the migration runs and does what we expect. One exception is when the migration does not have deterministic output. For example,
    ```
    {
    	DB:          db.DB,
    	Description: "Backfill vetted_at with time.now for nodes that have been vetted already (aka nodes that have been audited 100 times)",
    	Version:     99,
    	Action: migrate.SQL{
    		`UPDATE nodes SET vetted_at = date_trunc('day', now() at time zone 'utc') at time zone 'utc' WHERE total_audit_count >= 100;`,
    	},
    },
    ```
    The above migration sets a value to the current day. This can be solved in one of two ways:
    1. Have each of the `INSERT` statements for the matching nodes in the main section use `date_trunc('day', now() at time zone 'utc') at time zone 'utc'` for the `vetted_at` column
    2. Have an `UPDATE "nodes" SET vetted_at = 'fixed date' where id = 'expected id'` in the `-- NEW DATA --` section (so that it runs in both the snapshot, and after the migration has run) and update the main section's `INSERT` for that node. Future snapshot files do not need to retain the `UPDATE` in that case, and the `INSERT` statements can just use the fixed date for the future.

    See migration 99 for the specifics, where it chose option 2.

3. Cockroach does schema changes asynchronously with regards to a transaction. This means if you need to add a column and fill it with some data, then these need to have them in separate migrations steps with using `SeparateTx`:
    ```
    {
    	DB:          db.DB,
    	Description: "Add project bandwidth limit",
    	Version:     999,
    	Action: migrate.SQL{
    		`ALTER TABLE projects ADD COLUMN bandwidth_limit`,
    		`UPDATE projects SET bandwidth_limit = usage_limit`,
    	},
    },
    ```
    Will fail with column "bandwidth_limit" is missing. To make it work, it needs to be written as:
    ```
    {
    	DB:          db.DB,
    	Description: "add separate bandwidth column",
    	Version:     107,
    	Action: migrate.SQL{
    		`ALTER TABLE projects ADD COLUMN bandwidth_limit bigint NOT NULL DEFAULT 0;`,
    	},
    },
    {
    	DB:          db.DB,
    	Description: "backfill bandwidth column with previous limits",
    	Version:     108,
    	SeparateTx:  true,
    	Action: migrate.SQL{
    			`UPDATE projects SET bandwidth_limit = usage_limit;`,
    	},
    },
    ```

4. Removing a DEFAULT value for a column can be tricky. Old values inserted in the snapshot files may not specify every column and relying on those defaults. In the migration that removes the DEFAULT value, you must also change any INSERT statements to include any unspecified columns, setting them to the dropped DEFAULT. This is because the main database will have inserted them while they had the DEFAULT, but the new snapshot will not be inserting while the columns have the DEFAULT.

