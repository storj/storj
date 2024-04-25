// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/shared/dbutil/pgutil"
)

// ensures that *webappSessions implements consoleauth.WebappSessions.
var _ consoleauth.WebappSessions = (*webappSessions)(nil)

type webappSessions struct {
	db *satelliteDB
}

// Create creates a webapp session and returns the session info.
func (db *webappSessions) Create(ctx context.Context, id, userID uuid.UUID, address, userAgent string, expiresAt time.Time) (session consoleauth.WebappSession, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxSession, err := db.db.Create_WebappSession(ctx, dbx.WebappSession_Id(id.Bytes()), dbx.WebappSession_UserId(userID.Bytes()),
		dbx.WebappSession_IpAddress(address), dbx.WebappSession_UserAgent(userAgent), dbx.WebappSession_ExpiresAt(expiresAt))
	if err != nil {
		return session, err
	}

	return getSessionFromDBX(dbxSession)
}

// UpdateExpiration updates the expiration time of the session.
func (db *webappSessions) UpdateExpiration(ctx context.Context, sessionID uuid.UUID, expiresAt time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.db.Update_WebappSession_By_Id(
		ctx,
		dbx.WebappSession_Id(sessionID.Bytes()),
		dbx.WebappSession_Update_Fields{
			ExpiresAt: dbx.WebappSession_ExpiresAt(expiresAt),
		},
	)

	return err
}

// GetBySessionID gets the session info from the session ID.
func (db *webappSessions) GetBySessionID(ctx context.Context, sessionID uuid.UUID) (session consoleauth.WebappSession, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxSession, err := db.db.Get_WebappSession_By_Id(ctx, dbx.WebappSession_Id(sessionID.Bytes()))
	if err != nil {
		return session, err
	}

	return getSessionFromDBX(dbxSession)
}

// GetAllByUserID gets all webapp sessions with userID.
func (db *webappSessions) GetAllByUserID(ctx context.Context, userID uuid.UUID) (sessions []consoleauth.WebappSession, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxSessions, err := db.db.All_WebappSession_By_UserId(ctx, dbx.WebappSession_UserId(userID.Bytes()))
	for _, dbxs := range dbxSessions {
		s, err := getSessionFromDBX(dbxs)
		if err != nil {
			return sessions, err
		}
		sessions = append(sessions, s)
	}

	return sessions, nil
}

// DeleteBySessionID deletes a webapp session by ID.
func (db *webappSessions) DeleteBySessionID(ctx context.Context, sessionID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.db.Delete_WebappSession_By_Id(ctx, dbx.WebappSession_Id(sessionID.Bytes()))

	return err
}

// DeleteAllByUserID deletes all webapp sessions by user ID.
func (db *webappSessions) DeleteAllByUserID(ctx context.Context, userID uuid.UUID) (deleted int64, err error) {
	defer mon.Task()(&ctx)(&err)

	return db.db.Delete_WebappSession_By_UserId(ctx, dbx.WebappSession_UserId(userID.Bytes()))
}

// DeleteExpired deletes all sessions that have expired before the provided timestamp.
func (db *webappSessions) DeleteExpired(ctx context.Context, now time.Time, asOfSystemTimeInterval time.Duration, pageSize int) (err error) {
	defer mon.Task()(&ctx)(&err)

	if pageSize <= 0 {
		return Error.New("expected page size to be positive; got %d", pageSize)
	}

	var pageCursor uuid.UUID
	selected := make([]uuid.UUID, pageSize)
	aost := db.db.impl.AsOfSystemInterval(asOfSystemTimeInterval)
	for {
		// Select the ID beginning this page of records
		err := db.db.QueryRowContext(ctx, `
			SELECT id FROM webapp_sessions
			`+aost+`
			WHERE id > $1 AND expires_at < $2
			ORDER BY id LIMIT 1
		`, pageCursor, now).Scan(&pageCursor)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			return Error.Wrap(err)
		}

		// Select page of records
		rows, err := db.db.QueryContext(ctx, `
			SELECT id FROM webapp_sessions
			`+aost+`
			WHERE id >= $1 ORDER BY id LIMIT $2
		`, pageCursor, pageSize)
		if err != nil {
			return Error.Wrap(err)
		}

		var i int
		for i = 0; rows.Next(); i++ {
			if err = rows.Scan(&selected[i]); err != nil {
				return Error.Wrap(err)
			}
		}
		if err = errs.Combine(rows.Err(), rows.Close()); err != nil {
			return Error.Wrap(err)
		}

		// Delete all expired records in the page
		_, err = db.db.ExecContext(ctx, `
			DELETE FROM webapp_sessions
			WHERE id = ANY($1)
			AND expires_at < $2
		`, pgutil.UUIDArray(selected[:i]), now)
		if err != nil {
			return Error.Wrap(err)
		}

		if i < pageSize {
			return nil
		}

		// Advance the cursor to the next page
		pageCursor = selected[i-1]
	}
}

func getSessionFromDBX(dbxSession *dbx.WebappSession) (consoleauth.WebappSession, error) {
	id, err := uuid.FromBytes(dbxSession.Id)
	if err != nil {
		return consoleauth.WebappSession{}, err
	}
	userID, err := uuid.FromBytes(dbxSession.UserId)
	if err != nil {
		return consoleauth.WebappSession{}, err
	}
	return consoleauth.WebappSession{
		ID:        id,
		UserID:    userID,
		Address:   dbxSession.IpAddress,
		UserAgent: dbxSession.UserAgent,
		Status:    dbxSession.Status,
		ExpiresAt: dbxSession.ExpiresAt,
	}, nil
}
