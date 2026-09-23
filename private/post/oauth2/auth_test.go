// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package oauth2

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/smtp"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAuthWithContextBoundsTokenRefresh(t *testing.T) {
	blocked := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-blocked
	}))
	defer func() {
		close(blocked)
		server.Close()
	}()

	auth := &Auth{
		UserEmail: "sender@example.com",
		// an already expired token forces Start to refresh.
		Storage: NewTokenStore(Credentials{TokenURI: server.URL}, Token{Expiry: time.Now().Add(-time.Hour)}),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, _, err := auth.WithContext(ctx).Start(&smtp.ServerInfo{TLS: true})
		done <- err
	}()

	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("token refresh was not bounded by the context")
	}
}
