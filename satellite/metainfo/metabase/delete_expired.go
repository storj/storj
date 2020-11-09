// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"bytes"
	"context"
	"sort"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/private/dbutil/pgutil"
	"storj.io/storj/private/dbutil/txutil"
	"storj.io/storj/private/tagsql"
)

// DeleteExpiredObjects deletes all objects that expired before expiredBefore.
func (db *DB) DeleteExpiredObjects(ctx context.Context, expiredBefore time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	var startAfter Object
	for {
		lastDeleted, err := db.deleteExpiredObjectsBatch(ctx, startAfter, expiredBefore)
		if err != nil {
			return err
		}
		if lastDeleted.StreamID.IsZero() {
			return nil
		}
		startAfter = lastDeleted
	}
}

func (db *DB) deleteExpiredObjectsBatch(ctx context.Context, startAfter Object, expiredBefore time.Time) (lastDeleted Object, err error) {
	defer mon.Task()(&ctx)(&err)

	err = txutil.WithTx(ctx, db.db, nil, func(ctx context.Context, tx tagsql.Tx) error {
		var streamIDs [][]byte
		// TODO: Consider adding an index like "CREATE INDEX ON objects (expires_at) WHERE expires_at IS NOT NULL".
		// It would let the database go immediately to the relevant rows instead of scanning through the table for
		// them. This would save a lot of time if a very small percent of all rows have expiration time, which is
		// what we actually expect.
		err = withRows(tx.Query(ctx, `
			DELETE FROM objects
				WHERE stream_id IN (
					SELECT stream_id FROM objects
						WHERE
							(project_id, bucket_name, object_key, version) > ($1, $2, $3, $4)
							AND expires_at < $5
						ORDER BY project_id, bucket_name, object_key, version
						LIMIT $6
				)
				RETURNING
					project_id, bucket_name,
					object_key, version, stream_id,
					expires_at;
			`, lastDeleted.ProjectID, lastDeleted.BucketName, []byte(lastDeleted.ObjectKey), lastDeleted.Version,
			expiredBefore,
			batchsizeLimit),
		)(func(rows tagsql.Rows) error {
			for rows.Next() {
				err = rows.Scan(&lastDeleted.ProjectID, &lastDeleted.BucketName,
					&lastDeleted.ObjectKey, &lastDeleted.Version, &lastDeleted.StreamID,
					&lastDeleted.ExpiresAt)
				if err != nil {
					return Error.New("unable to delete expired objects: %w", err)
				}

				db.log.Info("Deleting expired object",
					zap.Stringer("Project", lastDeleted.ProjectID),
					zap.String("Bucket", lastDeleted.BucketName),
					zap.String("Object Key", string(lastDeleted.ObjectKey)),
					zap.Int64("Version", int64(lastDeleted.Version)),
					zap.Time("Expired At", *lastDeleted.ExpiresAt),
				)

				streamIDs = append(streamIDs, lastDeleted.StreamID[:])
			}
			return nil
		})
		if err != nil {
			return Error.New("unable to delete expired objects: %w", err)
		}

		err = deleteExpiredSegments(ctx, tx, streamIDs)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return Object{}, err
	}

	return lastDeleted, nil
}

func deleteExpiredSegments(ctx context.Context, tx tagsql.Tx, streamIDs [][]byte) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(streamIDs) == 0 {
		return nil
	}

	// Sorting the stream IDs isn't strictly necessary, but it may help the query
	// be more serializable with respect to other transactions, particularly if
	// there are any others that touch multiple segments rows. That is, without
	// this sorting, it might happen that this query will need to be retried more
	// times than it would have been otherwise.
	sort.Slice(streamIDs, func(i, j int) bool {
		return bytes.Compare(streamIDs[i], streamIDs[j]) < 0
	})

	_, err = tx.ExecContext(ctx, `
			DELETE FROM segments
			WHERE stream_id = ANY ($1);
		`, pgutil.ByteaArray(streamIDs))
	if err != nil {
		return Error.New("unable to delete expired segments: %w", err)
	}

	return nil
}
