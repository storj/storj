// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"
)

const (
	testBridgeUser = "testuser@storj.io"
	testBridgePass = "secret"
)

func NewTestEnv(ts *httptest.Server) Env {
	return Env{
		URL:      ts.URL,
		User:     testBridgeUser,
		Password: testBridgePass,
		Mnemonic: testMnemonic,
	}
}

func NewNoAuthTestEnv(ts *httptest.Server) Env {
	return Env{
		URL: ts.URL,
	}
}

func NewBadPassTestEnv(ts *httptest.Server) Env {
	return Env{
		URL:      ts.URL,
		User:     testBridgeUser,
		Password: "bad password",
		Mnemonic: testMnemonic,
	}
}

func NewNoMnemonicTestEnv(ts *httptest.Server) Env {
	return Env{
		URL:      ts.URL,
		User:     testBridgeUser,
		Password: testBridgePass,
	}
}

func basicAuth(h httprouter.Handle, requiredUser, requiredPassword string) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// Get the Basic Authentication credentials
		user, password, hasAuth := r.BasicAuth()

		if hasAuth && user == requiredUser && password == requiredPassword {
			// Delegate request to the given handle
			h(w, r, ps)
		} else {
			// Request Basic Authentication otherwise
			w.Header().Set("WWW-Authenticate", "Basic realm=Restricted")
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		}
	}
}

func TestNewEnv(t *testing.T) {
	for _, tt := range []struct {
		env Env
		url string
	}{
		{Env{}, ""},
		{NewEnv(), DefaultURL},
		{Env{URL: "http://example.com"}, "http://example.com"},
	} {
		assert.Equal(t, tt.url, tt.env.URL)
	}
}
