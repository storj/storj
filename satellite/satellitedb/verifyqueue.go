// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"time"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/pgutil"
)

const (
	// VerifyRetryInterval defines a limit on how frequently we retry
	// verification audits. At least this long should elapse between
	// attempts.
	VerifyRetryInterval = 4 * time.Hour
)

// verifyQueue implements storj.io/storj/satellite/audit.VerifyQueue.
type verifyQueue struct {
	db *satelliteDB
}

var _ audit.VerifyQueue = (*verifyQueue)(nil)

func (vq *verifyQueue) Push(ctx context.Context, segments []audit.Segment, maxBatchSize int) (err error) {
	defer mon.Task()(&ctx)(&err)

	positionSlice := make([]int64, maxBatchSize)
	expirationSlice := make([]*time.Time, maxBatchSize)

	// sort segments in the order of the primary key before inserting, for performance reasons
	sort.Sort(audit.ByStreamIDAndPosition(segments))

	segmentsCounter := mon.Counter("audit_verify_queue_segments_inserted")
	segmentsIndex := 0

	switch vq.db.impl {
	case dbutil.Postgres, dbutil.Cockroach:
		streamIDSlice := make([]uuid.UUID, maxBatchSize)
		encryptedSizeSlice := make([]int32, maxBatchSize)
		var insertFunc func(streamIDSlice []uuid.UUID, positionSlice []int64, expirationSlice []*time.Time, encryptedSizeSlice []int32) (sql.Result, error)

		insertFunc = func(streamIDSlice []uuid.UUID, positionSlice []int64, expirationSlice []*time.Time, encryptedSizeSlice []int32) (sql.Result, error) {
			res, err := vq.db.DB.ExecContext(ctx, `
					INSERT INTO verification_audits (stream_id, position, expires_at, encrypted_size)
					SELECT unnest($1::bytea[]), unnest($2::int8[]), unnest($3::timestamptz[]), unnest($4::int4[])`,
				pgutil.UUIDArray(streamIDSlice),
				pgutil.Int8Array(positionSlice),
				pgutil.NullTimestampTZArray(expirationSlice),
				pgutil.Int4Array(encryptedSizeSlice),
			)
			return res, err
		}
		for segmentsIndex < len(segments) {
			batchIndex := 0
			for batchIndex < maxBatchSize && segmentsIndex < len(segments) {
				streamIDSlice[batchIndex] = segments[segmentsIndex].StreamID
				positionSlice[batchIndex] = int64(segments[segmentsIndex].Position.Encode())
				expirationSlice[batchIndex] = segments[segmentsIndex].ExpiresAt
				encryptedSizeSlice[batchIndex] = segments[segmentsIndex].EncryptedSize
				batchIndex++
				segmentsIndex++
			}

			res, err := insertFunc((streamIDSlice[:batchIndex]), positionSlice[:batchIndex],
				expirationSlice[:batchIndex], encryptedSizeSlice[:batchIndex])

			if err != nil {
				return Error.Wrap(err)
			}

			if n, err := res.RowsAffected(); err == nil {
				segmentsCounter.Inc(n)
			} else {
				segmentsCounter.Inc(int64(batchIndex))
			}
		}
		return nil

	case dbutil.Spanner:
		encryptedSizeSliceSpanner := make([]int, maxBatchSize)
		streamIDSliceSpanner := make([][]byte, maxBatchSize)
		var insertFuncSpanner func(streamIDSlice [][]byte, positionSlice []int64, expirationSlice []*time.Time, encryptedSizeSlice []int) (sql.Result, error)

		insertFuncSpanner = func(streamIDSlice [][]byte, positionSlice []int64, expirationSlice []*time.Time, encryptedSizeSlice []int) (sql.Result, error) {

			type verificationAudit struct {
				StreamID      []byte
				Position      int64
				Expiration    *time.Time
				EncryptedSize int
			}

			audits := make([]verificationAudit, len(streamIDSlice))
			for i := range audits {
				audits[i] = verificationAudit{
					StreamID:      streamIDSlice[i],
					Position:      positionSlice[i],
					Expiration:    expirationSlice[i],
					EncryptedSize: encryptedSizeSlice[i],
				}
			}

			res, err := vq.db.DB.ExecContext(ctx, `
				INSERT INTO verification_audits (stream_id, position, expires_at, encrypted_size)
				(SELECT StreamID, Position, Expiration, EncryptedSize FROM UNNEST(?))`, audits,
			)
			return res, err
		}
		for segmentsIndex < len(segments) {
			batchIndex := 0
			for batchIndex < maxBatchSize && segmentsIndex < len(segments) {
				streamIDSliceSpanner[batchIndex] = segments[segmentsIndex].StreamID.Bytes()
				positionSlice[batchIndex] = int64(segments[segmentsIndex].Position.Encode())
				expirationSlice[batchIndex] = segments[segmentsIndex].ExpiresAt
				encryptedSizeSliceSpanner[batchIndex] = int(segments[segmentsIndex].EncryptedSize)
				batchIndex++
				segmentsIndex++
			}

			res, err := insertFuncSpanner((streamIDSliceSpanner[:batchIndex]), positionSlice[:batchIndex],
				expirationSlice[:batchIndex], encryptedSizeSliceSpanner[:batchIndex])

			if err != nil {
				return Error.Wrap(err)
			}

			if n, err := res.RowsAffected(); err == nil {
				segmentsCounter.Inc(n)
			} else {
				segmentsCounter.Inc(int64(batchIndex))
			}
		}
		return nil

	default:
		return Error.New("unsupported database: %v", vq.db.impl)
	}
}

func (vq *verifyQueue) Next(ctx context.Context) (seg audit.Segment, err error) {
	defer mon.Task()(&ctx)(&err)

	var getQuery string
	switch vq.db.impl {
	case dbutil.Postgres:
		getQuery = `
			WITH next_row AS (
				SELECT inserted_at, stream_id, position
				FROM verification_audits
				ORDER BY inserted_at, stream_id, position
				FOR UPDATE SKIP LOCKED
				LIMIT 1
			)
			DELETE FROM verification_audits v
				USING next_row
			WHERE v.inserted_at = next_row.inserted_at
				AND v.stream_id = next_row.stream_id
				AND v.position = next_row.position
			RETURNING v.stream_id, v.position, v.expires_at, v.encrypted_size
		`
		err = vq.db.DB.QueryRowContext(ctx, getQuery).Scan(&seg.StreamID, &seg.Position, &seg.ExpiresAt, &seg.EncryptedSize)

	case dbutil.Cockroach:
		// Note: because Cockroach does not support SKIP LOCKED, this implementation
		// is likely much less performant under any amount of contention.
		getQuery = `
			WITH next_row AS (
				SELECT inserted_at, stream_id, position
				FROM verification_audits
				ORDER BY inserted_at, stream_id, position
				FOR UPDATE
				LIMIT 1
			)
			DELETE FROM verification_audits v
			WHERE v.inserted_at = (SELECT inserted_at FROM next_row)
				AND v.stream_id = (SELECT stream_id FROM next_row)
				AND v.position = (SELECT position FROM next_row)
			RETURNING v.stream_id, v.position, v.expires_at, v.encrypted_size
		`
		err = vq.db.DB.QueryRowContext(ctx, getQuery).Scan(&seg.StreamID, &seg.Position, &seg.ExpiresAt, &seg.EncryptedSize)

	case dbutil.Spanner:
		err = vq.db.DB.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
			// Note - Spanner does not support a with clause when deleting rows.
			// For which two queries are used to accomplish this
			getQuery = `SELECT inserted_at, stream_id, position, expires_at, encrypted_size
			FROM verification_audits
			ORDER BY inserted_at, stream_id, position
			LIMIT 1`

			row := tx.Tx.QueryRowContext(ctx, getQuery)

			var insertedAt time.Time

			err = row.Scan(&insertedAt, &seg.StreamID, &seg.Position, &seg.ExpiresAt, &seg.EncryptedSize)

			if err == nil {
				query := `DELETE FROM verification_audits
				WHERE inserted_at = ?
				 AND stream_id = ?
				 AND position = ?`
				_, err = tx.Tx.ExecContext(ctx, query, insertedAt, seg.StreamID, seg.Position)
			}
			return err
		})
	default:
		err = errors.New("unsupported database implementation")
	}

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return audit.Segment{}, audit.ErrEmptyQueue.Wrap(err)
		}
		return audit.Segment{}, Error.Wrap(err)
	}

	mon.Counter("audit_verify_queue_segments_deleted").Dec(1)
	return seg, nil
}
