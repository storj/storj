// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package consoledb_test

import (
	"database/sql"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console/consoleauth"
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
		require.WithinDuration(t, expiresAt, session.ExpiresAt, 0)

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
		require.WithinDuration(t, expiresAt, session.ExpiresAt, 0)

		session, err = sessions.GetBySessionID(ctx, session.ID)
		require.NoError(t, err)

		require.Equal(t, id, session.ID)
		require.Equal(t, userID, session.UserID)
		require.Equal(t, address, session.Address)
		require.Equal(t, userAgent, session.UserAgent)
		require.WithinDuration(t, expiresAt, session.ExpiresAt, 0)
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
		require.WithinDuration(t, expiresAt, session1.ExpiresAt, 0)

		id2 := testrand.UUID()

		session2, err := sessions.Create(ctx, id2, userID, address, userAgent, expiresAt)
		require.NoError(t, err)
		require.Equal(t, id2, session2.ID)
		require.Equal(t, userID, session2.UserID)
		require.Equal(t, address, session2.Address)
		require.Equal(t, userAgent, session2.UserAgent)
		require.WithinDuration(t, expiresAt, session2.ExpiresAt, 0)

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
		require.WithinDuration(t, expiresAt, allSessions[0].ExpiresAt, 0)
		require.Equal(t, userID, allSessions[1].UserID)
		require.Equal(t, address, allSessions[1].Address)
		require.Equal(t, userAgent, allSessions[1].UserAgent)
		require.WithinDuration(t, expiresAt, allSessions[1].ExpiresAt, 0)
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

		createSessions := func() {
			session1, err := sessions.Create(ctx, id1, userID, address, userAgent, expiresAt)
			require.NoError(t, err)
			require.Equal(t, id1, session1.ID)
			require.Equal(t, userID, session1.UserID)
			require.Equal(t, address, session1.Address)
			require.Equal(t, userAgent, session1.UserAgent)
			require.WithinDuration(t, expiresAt, session1.ExpiresAt, 0)

			id2 := testrand.UUID()

			session2, err := sessions.Create(ctx, id2, userID, address, userAgent, expiresAt)
			require.NoError(t, err)
			require.Equal(t, id2, session2.ID)
			require.Equal(t, userID, session2.UserID)
			require.Equal(t, address, session2.Address)
			require.Equal(t, userAgent, session2.UserAgent)
			require.WithinDuration(t, expiresAt, session2.ExpiresAt, 0)
		}

		createSessions()

		deleted, err := sessions.DeleteAllByUserID(ctx, userID)
		require.NoError(t, err)
		require.Equal(t, deleted, int64(2))

		allSessions, err := sessions.GetAllByUserID(ctx, userID)
		require.NoError(t, err)
		require.Len(t, allSessions, 0)

		createSessions()

		allSessions, err = sessions.GetAllByUserID(ctx, userID)
		require.NoError(t, err)
		require.Len(t, allSessions, 2)

		excluded := allSessions[0].ID

		deleted, err = sessions.DeleteAllByUserIDExcept(ctx, userID, excluded)
		require.NoError(t, err)
		require.Equal(t, deleted, int64(1))

		allSessions, err = sessions.GetAllByUserID(ctx, userID)
		require.NoError(t, err)
		require.Len(t, allSessions, 1)
		require.Equal(t, excluded, allSessions[0].ID)
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

func TestGetActivePagedByUserID(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		sessionsDB := db.Console().WebappSessions()
		now := time.Now()
		ownerID := testrand.UUID()
		traitorID := testrand.UUID()

		_, err := sessionsDB.GetPagedActiveByUserID(ctx, ownerID, time.Time{}, consoleauth.WebappSessionsCursor{
			Limit: 0,
			Page:  1,
		})
		require.Error(t, err)
		_, err = sessionsDB.GetPagedActiveByUserID(ctx, ownerID, time.Time{}, consoleauth.WebappSessionsCursor{
			Limit: 10,
			Page:  0,
		})
		require.Error(t, err)

		userSessionsCount := 5
		for i := 0; i < userSessionsCount; i++ {
			_, err = sessionsDB.Create(ctx, testrand.UUID(), ownerID, "", strconv.Itoa(i), now.Add(time.Duration(i+1)*time.Hour))
			require.NoError(t, err)
		}
		// Add a session for another user.
		_, err = sessionsDB.Create(ctx, testrand.UUID(), traitorID, "", "", now.Add(time.Hour))
		require.NoError(t, err)
		// Add an expired session for the owner.
		_, err = sessionsDB.Create(ctx, testrand.UUID(), ownerID, "", "", now.Add(-time.Hour))
		require.NoError(t, err)

		page, err := sessionsDB.GetPagedActiveByUserID(ctx, ownerID, now, consoleauth.WebappSessionsCursor{
			Limit: 10,
			Page:  1,
		})
		require.NoError(t, err)
		require.Len(t, page.Sessions, userSessionsCount)

		page, err = sessionsDB.GetPagedActiveByUserID(ctx, ownerID, now, consoleauth.WebappSessionsCursor{
			Limit: uint(userSessionsCount - 1),
			Page:  2,
		})
		require.NoError(t, err)
		require.Len(t, page.Sessions, 1)

		page, err = sessionsDB.GetPagedActiveByUserID(ctx, ownerID, now, consoleauth.WebappSessionsCursor{
			Limit:          10,
			Page:           1,
			Order:          consoleauth.UserAgent,
			OrderDirection: consoleauth.Ascending,
		})
		require.NoError(t, err)
		for i, session := range page.Sessions {
			require.Equal(t, strconv.Itoa(i), session.UserAgent)
		}

		page, err = sessionsDB.GetPagedActiveByUserID(ctx, ownerID, now, consoleauth.WebappSessionsCursor{
			Limit:          10,
			Page:           1,
			Order:          consoleauth.ExpiresAt,
			OrderDirection: consoleauth.Descending,
		})
		require.NoError(t, err)
		for i, session := range page.Sessions {
			hours := len(page.Sessions) - i
			require.WithinDuration(t, now.Add(time.Duration(hours)*time.Hour), session.ExpiresAt, time.Minute)
		}

		for _, session := range page.Sessions {
			err = sessionsDB.UpdateExpiration(ctx, session.ID, now.Add(-2*time.Hour))
			require.NoError(t, err)
		}

		page, err = sessionsDB.GetPagedActiveByUserID(ctx, ownerID, now, consoleauth.WebappSessionsCursor{
			Limit: 10,
			Page:  1,
		})
		require.NoError(t, err)
		require.Len(t, page.Sessions, 0)
	})
}
