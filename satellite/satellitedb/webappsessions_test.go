// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestWebappSessionsCreate(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		sessions := db.Console().WebappSessions()

		id := testrand.UUID()
		userID := testrand.UUID()
		address := "127.0.0.1"
		userAgent := "test_user_agent"
		expiresAt := time.Now().Add(24 * time.Hour).Truncate(time.Second)

		session, err := sessions.Create(ctx, id, userID, address, userAgent, expiresAt)
		require.NoError(t, err)

		require.Equal(t, id, session.ID)
		require.Equal(t, userID, session.UserID)
		require.Equal(t, address, session.Address)
		require.Equal(t, userAgent, session.UserAgent)
		require.Equal(t, expiresAt, session.ExpiresAt)

		_, err = sessions.Create(ctx, id, userID, address, userAgent, expiresAt)
		require.Error(t, err)
	})
}

func TestWebappSessionsGetBySessionID(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		sessions := db.Console().WebappSessions()

		id := testrand.UUID()
		userID := testrand.UUID()
		address := "127.0.0.1"
		userAgent := "test_user_agent"
		expiresAt := time.Now().Add(24 * time.Hour).Truncate(time.Second)

		session, err := sessions.Create(ctx, id, userID, address, userAgent, expiresAt)
		require.NoError(t, err)

		require.Equal(t, id, session.ID)
		require.Equal(t, userID, session.UserID)
		require.Equal(t, address, session.Address)
		require.Equal(t, userAgent, session.UserAgent)
		require.Equal(t, expiresAt, session.ExpiresAt)

		session, err = sessions.GetBySessionID(ctx, session.ID)
		require.NoError(t, err)

		require.Equal(t, id, session.ID)
		require.Equal(t, userID, session.UserID)
		require.Equal(t, address, session.Address)
		require.Equal(t, userAgent, session.UserAgent)
		require.Equal(t, expiresAt, session.ExpiresAt)
	})
}

func TestWebappSessionsGetAllByUserID(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		sessions := db.Console().WebappSessions()

		id1 := testrand.UUID()
		userID := testrand.UUID()
		address := "127.0.0.1"
		userAgent := "test_user_agent"
		expiresAt := time.Now().Add(24 * time.Hour).Truncate(time.Second)

		session1, err := sessions.Create(ctx, id1, userID, address, userAgent, expiresAt)
		require.NoError(t, err)
		require.Equal(t, id1, session1.ID)
		require.Equal(t, userID, session1.UserID)
		require.Equal(t, address, session1.Address)
		require.Equal(t, userAgent, session1.UserAgent)
		require.Equal(t, expiresAt, session1.ExpiresAt)

		id2 := testrand.UUID()

		session2, err := sessions.Create(ctx, id2, userID, address, userAgent, expiresAt)
		require.NoError(t, err)
		require.Equal(t, id2, session2.ID)
		require.Equal(t, userID, session2.UserID)
		require.Equal(t, address, session2.Address)
		require.Equal(t, userAgent, session2.UserAgent)
		require.Equal(t, expiresAt, session2.ExpiresAt)

		allSessions, err := sessions.GetAllByUserID(ctx, userID)
		require.NoError(t, err)
		require.Len(t, allSessions, 2)

		var foundSession1, foundSession2 bool
		if id1 == allSessions[0].ID || id1 == allSessions[1].ID {
			foundSession1 = true
		}
		if id2 == allSessions[0].ID || id2 == allSessions[1].ID {
			foundSession2 = true
		}
		require.True(t, foundSession1)
		require.True(t, foundSession2)
		require.Equal(t, userID, allSessions[0].UserID)
		require.Equal(t, address, allSessions[0].Address)
		require.Equal(t, userAgent, allSessions[0].UserAgent)
		require.Equal(t, expiresAt, allSessions[0].ExpiresAt)
		require.Equal(t, userID, allSessions[1].UserID)
		require.Equal(t, address, allSessions[1].Address)
		require.Equal(t, userAgent, allSessions[1].UserAgent)
		require.Equal(t, expiresAt, allSessions[1].ExpiresAt)
	})
}

func TestWebappSessionsDeleteBySessionID(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		sessions := db.Console().WebappSessions()

		id1 := testrand.UUID()
		userID := testrand.UUID()
		address := "127.0.0.1"
		userAgent := "test_user_agent"
		expiresAt := time.Now().Add(24 * time.Hour).Truncate(time.Second)

		session1, err := sessions.Create(ctx, id1, userID, address, userAgent, expiresAt)
		require.NoError(t, err)

		id2 := testrand.UUID()

		session2, err := sessions.Create(ctx, id2, userID, address, userAgent, expiresAt)
		require.NoError(t, err)

		require.NoError(t, sessions.DeleteBySessionID(ctx, session1.ID))

		_, err = sessions.GetBySessionID(ctx, session1.ID)
		require.Error(t, err)

		_, err = sessions.GetBySessionID(ctx, session2.ID)
		require.NoError(t, err)
	})
}

func TestWebappSessionsDeleteAllByUserID(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		sessions := db.Console().WebappSessions()

		id1 := testrand.UUID()
		userID := testrand.UUID()
		address := "127.0.0.1"
		userAgent := "test_user_agent"
		expiresAt := time.Now().Add(24 * time.Hour).Truncate(time.Second)

		session1, err := sessions.Create(ctx, id1, userID, address, userAgent, expiresAt)
		require.NoError(t, err)
		require.Equal(t, id1, session1.ID)
		require.Equal(t, userID, session1.UserID)
		require.Equal(t, address, session1.Address)
		require.Equal(t, userAgent, session1.UserAgent)
		require.Equal(t, expiresAt, session1.ExpiresAt)

		id2 := testrand.UUID()

		session2, err := sessions.Create(ctx, id2, userID, address, userAgent, expiresAt)
		require.NoError(t, err)
		require.Equal(t, id2, session2.ID)
		require.Equal(t, userID, session2.UserID)
		require.Equal(t, address, session2.Address)
		require.Equal(t, userAgent, session2.UserAgent)
		require.Equal(t, expiresAt, session2.ExpiresAt)

		deleted, err := sessions.DeleteAllByUserID(ctx, userID)
		require.NoError(t, err)
		require.Equal(t, deleted, int64(2))

		allSessions, err := sessions.GetAllByUserID(ctx, userID)
		require.NoError(t, err)
		require.Len(t, allSessions, 0)
	})
}

func TestDeleteExpired(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		sessionsDB := db.Console().WebappSessions()
		now := time.Now()

		// Only positive page sizes should be allowed.
		require.Error(t, sessionsDB.DeleteExpired(ctx, time.Time{}, 0, 0))
		require.Error(t, sessionsDB.DeleteExpired(ctx, time.Time{}, 0, -1))

		newSession, err := sessionsDB.Create(ctx, testrand.UUID(), testrand.UUID(), "", "", now.Add(time.Second))
		require.NoError(t, err)
		oldSession, err := sessionsDB.Create(ctx, testrand.UUID(), testrand.UUID(), "", "", now.Add(-time.Second))
		require.NoError(t, err)
		require.NoError(t, sessionsDB.DeleteExpired(ctx, now, 0, 1))

		// Ensure that the old session record was deleted and the other remains.
		_, err = sessionsDB.GetBySessionID(ctx, oldSession.ID)
		require.ErrorIs(t, err, sql.ErrNoRows)
		_, err = sessionsDB.GetBySessionID(ctx, newSession.ID)
		require.NoError(t, err)
	})
}
