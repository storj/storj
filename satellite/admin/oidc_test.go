// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/satellite/admin"
)

func TestOIDCMiddleware(t *testing.T) {
	log := zaptest.NewLogger(t)
	handler := admin.NewOIDCHandler(log, admin.OIDCConfig{
		Enabled:       true,
		ProviderURL:   "https://provider.example.com",
		ClientID:      "test-client",
		ClientSecret:  "test-secret",
		GroupsClaim:   "roles",
		SessionSecret: "a-secret-that-is-at-least-32-chars-x",
	}, false, "http://localhost")

	t.Run("InjectsHeaders", func(t *testing.T) {
		email := "admin@example.com"
		groups := []string{"admins", "ops"}

		w := httptest.NewRecorder()
		require.NoError(t, handler.TestSetSession(w, httptest.NewRequest(http.MethodGet, "/", nil), email, groups, time.Now().Add(time.Hour)))

		var sessionCookie *http.Cookie
		for _, c := range w.Result().Cookies() {
			if c.Name == "admin_session" {
				sessionCookie = c
				break
			}
		}
		require.NotNil(t, sessionCookie)

		var gotEmail, gotGroups string
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotEmail = r.Header.Get("X-Forwarded-Email")
			gotGroups = r.Header.Get("X-Forwarded-Groups")
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
		req.AddCookie(sessionCookie)
		rec := httptest.NewRecorder()
		handler.OIDCMiddleware(inner).ServeHTTP(rec, req)

		require.Equal(t, email, gotEmail)
		require.Equal(t, "admins,ops", gotGroups)
		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("RedirectsWithoutSession", func(t *testing.T) {
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
		rec := httptest.NewRecorder()
		handler.OIDCMiddleware(inner).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		require.Equal(t, http.StatusFound, rec.Code)
		require.Equal(t, "/auth/login", rec.Header().Get("Location"))
	})

	t.Run("Returns401ForAPIWithoutSession", func(t *testing.T) {
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
		rec := httptest.NewRecorder()
		handler.OIDCMiddleware(inner).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/users", nil))
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("SkipsAuthAndStaticRoutes", func(t *testing.T) {
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
		for _, path := range []string{"/auth/login", "/auth/callback", "/auth/logout", "/auth/front-channel-logout", "/static/build/app.js"} {
			rec := httptest.NewRecorder()
			handler.OIDCMiddleware(inner).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			require.Equal(t, http.StatusOK, rec.Code, "path %s should pass through without auth", path)
		}
	})
}

func TestLogout(t *testing.T) {
	log := zaptest.NewLogger(t)
	handler := admin.NewOIDCHandler(log, admin.OIDCConfig{
		Enabled:       true,
		ProviderURL:   "https://provider.example.com",
		ClientID:      "test-client",
		ClientSecret:  "test-secret",
		GroupsClaim:   "roles",
		SessionSecret: "a-secret-that-is-at-least-32-chars-x",
	}, false, "http://localhost")

	newSessionCookie := func(t *testing.T) *http.Cookie {
		t.Helper()
		w := httptest.NewRecorder()
		require.NoError(t, handler.TestSetSession(w, httptest.NewRequest(http.MethodGet, "/", nil), "admin@example.com", []string{"admins"}, time.Now().Add(time.Hour)))
		for _, c := range w.Result().Cookies() {
			if c.Name == "admin_session" {
				return c
			}
		}
		t.Fatal("session cookie not set")
		return nil
	}

	assertSessionCleared := func(t *testing.T, rec *httptest.ResponseRecorder) {
		t.Helper()
		require.Equal(t, http.StatusFound, rec.Code)
		require.Equal(t, "/auth/login", rec.Header().Get("Location"))
		for _, c := range rec.Result().Cookies() {
			if c.Name == "admin_session" {
				require.Equal(t, -1, c.MaxAge)
				return
			}
		}
		t.Fatal("session cookie not cleared")
	}

	t.Run("Logout", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/logout", nil)
		req.AddCookie(newSessionCookie(t))
		rec := httptest.NewRecorder()
		handler.Logout(rec, req)
		assertSessionCleared(t, rec)
	})

	t.Run("FrontChannelLogout", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/front-channel-logout", nil)
		req.AddCookie(newSessionCookie(t))
		rec := httptest.NewRecorder()
		handler.FrontChannelLogout(rec, req)
		assertSessionCleared(t, rec)
	})
}
