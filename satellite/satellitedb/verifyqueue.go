// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"storj.io/private/dbutil"
	"storj.io/private/dbutil/pgutil"
	"storj.io/storj/satellite/audit"
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

func (vq *verifyQueue) Push(ctx context.Context, segments []audit.Segment) (err error) {
	defer mon.Task()(&ctx)(&err)

	streamIDSlice := make([][]byte, len(segments))
	positionSlice := make([]int64, len(segments))
	expirationSlice := make([]*time.Time, len(segments))
	encryptedSizeSlice := make([]int32, len(segments))

	for i, seg := range segments {
		streamIDSlice[i] = seg.StreamID.Bytes()
		positionSlice[i] = int64(seg.Position.Encode())
		expirationSlice[i] = seg.ExpiresAt
		encryptedSizeSlice[i] = seg.EncryptedSize
	}
	_, err = vq.db.DB.ExecContext(ctx, `
		INSERT INTO verification_audits (stream_id, position, expires_at, encrypted_size)
		SELECT unnest($1::bytea[]), unnest($2::int8[]), unnest($3::timestamptz[]), unnest($4::int4[])
	`,
		pgutil.ByteaArray(streamIDSlice),
		pgutil.Int8Array(positionSlice),
		pgutil.NullTimestampTZArray(expirationSlice),
		pgutil.Int4Array(encryptedSizeSlice),
	)
	return Error.Wrap(err)
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
	}

	err = vq.db.DB.QueryRowContext(ctx, getQuery).Scan(&seg.StreamID, &seg.Position, &seg.ExpiresAt, &seg.EncryptedSize)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return audit.Segment{}, audit.ErrEmptyQueue.Wrap(err)
		}
		return audit.Segment{}, Error.Wrap(err)
	}
	return seg, nil
}
