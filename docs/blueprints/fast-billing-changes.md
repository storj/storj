# Fast Billing Changes

## Problem Statement

Cockroach has some interesting performance characteristics.

1. Doing a blind write is fast
2. Batching writes into single transactions is fast
3. Doing reads and writes in the same transaction is slow
4. Doing reads at a given timestamp is non-blocking and fast

We hit some of these performance issues with the `used_serials` table: in order to check if we should include some serial into some bandwidth rollup, we had to issue a read in the same transaction as the eventual write. Also, since we had multiple APIs responsible for this, there was contention.

In order to address those performance issues, we started using the `reported_serials` table that was only ever blindly written to. In order to do blind writes and avoid double spending issues (a node submitting the same order multiple times), a reported order is not added into a bandwidth rollup until after it expires. Unfortunately, this causes a lot of confusion for a number of reasons.

1. End of month payouts do not include the last 7-8 days of bandwidth
2. Display of used bandwidth lags behind actual usage 7-8 days

Some constraints on the solution space happen due to our payment processing system having a number of steps to it. After orders are received, we eventually insert them into the bandwidth rollups tables, which are eventually read to be inserted into an accounting table. That accounting table is used for storagenode payments at the end of the month. The accounting table only includes entries that it has never included before based on creation timestamp, so you cannot insert into the past or update existing rows of the bandwidth rollups tables without also updating how the accounting table works.

## Proposed Solution

We create a new `pending_serial_queue` table to function as a queue of serials to process sent by storage nodes. Any order processing just blindly upserts into this table. The primary key will be on `( storage_node_id, bucket_id, serial_number )` which means that we don't necessarily consume them in the order they have been inserted, but we do get good prefix compression with cockroach.

We bring back a `consumed_serials` table which functions much like the older `used_serials` table to record which serials have been added into a rollup. It has a primary key of `( storage_node_id, serial_number )` to allow for quick point lookups and has an index on `expires_at` in order to allow for efficient deletes.

The core process consumes `pending_serial_queue` to do inserts into `consumed_serial`. Since there is only ever one core process doing this, we are given much flexibility in how to do the transactions (for example, we can do any size of transaction, or partition them into read-only and write-only transactions.) It first queries `pending_serial_queue` in pages (each page in its own transaction) for any values. While batching up the pages into a values to write, it has a read-only transaction open querying `consumed_serials` to ensure it does not double account, building a batch of writes into `storagenode_bandwidth_rollups`, `bucket_bandwidth_rollups`, and `reported_consumed_serials`.

At the end of a page, if the batches are large enough, a new transaction issues the blind upserts. It then issues a delete to the `pending_serial_queue` table for all entries that were part of the batch. Note that this does not need to be in the same transaction: since it was inserted into `consumed_serial`, we know that it will not be accounted for again.

Eventually, some process deletes from `consumed_serials` when they are definitely expired.

The essence of this solution is to go back to how we were doing it with `used_serials` except asynchronously and with a single worker so that we can do it with high performance and nearly the same characteristics with respect to when values are included for billing. It allows full control over transaction size and the read/write characteristics of each transaction.

## Benefits

- APIs can blindly upsert into `pending_serial_queue`, allowing high scalability
- Full control over transactions means we can tune sizes without code changes
- Using smaller transactions allows getting better monitoring data on rate of progress
- The core consumer will quickly include data that has been reported again
- Does not require changes to any of the other systems (accounting, dashboards, etc.) to have their behavior restored
- Does not modify existing tables, just adds new ones.

## Downsides

- It is racy if we need to have more than one consumer to keep up, but this can be fixed at some complexity cost with sharding the `pending_serial_queue` table if necessary.
- More temporary space used with `consumed_serials`, but hopefully this is offset by the prefix compression.

## Q/A

- Why not use kafka or some other queue for `pending_serial_queue`?

That'd be fine, and whatever the consumer of the queue is should be agnostic to how the queue is implemented. The fastest implementation will be one that uses the current database we have, and if we like the design of the system, changing where the serials get inserted to is an easy detail to change.

If the database can handle the load, I'd prefer not to have to spin up and maintain a new service and learn the operation challenges involved as we head into production. If the database cannot handle the load, the current system, while flawed, does not lose payment.

## Appendix

### dbx models

	model pending_serial_queue (
		table pending_serial_queue

		key   storage_node_id bucket_id serial_number
		field storage_node_id blob
		field bucket_id       blob
		field serial_number   blob

		field action     uint
		field settled    uint64
		field expires_at timestamp
	)

	model consumed_serial (
		key storage_node_id serial_number
		index ( fields expires_at )

		field storage_node_id blob
		field serial_number   blob
		field expires_at      timestamp
	)
