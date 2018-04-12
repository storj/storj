// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/julienschmidt/httprouter"
)

const (
	mockBridgeAddr = "localhost:8091"
	mockBridgeURL  = "http://" + mockBridgeAddr
	mockBridgeUser = "testuser@storj.io"
	mockBridgePass = "secret"
)

var (
	mockGetBucketsResponse string
)

func TestMain(m *testing.M) {
	mockBridge()
	os.Exit(m.Run())
}

func NewMockEnv() Env {
	return Env{
		URL:      mockBridgeURL,
		User:     mockBridgeUser,
		Password: mockBridgePass,
		Mnemonic: mockMnemonic,
	}
}

func NewMockNoAuthEnv() Env {
	return Env{
		URL: mockBridgeURL,
	}
}

func NewMockBadPassEnv() Env {
	return Env{
		URL:      mockBridgeURL,
		User:     mockBridgeUser,
		Password: "bad password",
		Mnemonic: mockMnemonic,
	}
}

func NewMockNoMnemonicEnv() Env {
	return Env{
		URL:      mockBridgeURL,
		User:     mockBridgeUser,
		Password: mockBridgePass,
	}
}

func mockBridge() {
	router := httprouter.New()
	router.GET("/", getInfo)
	router.GET("/buckets", basicAuth(getBuckets, mockBridgeUser, mockBridgePass))
	go http.ListenAndServe(mockBridgeAddr, router)
	// TODO better way to wait for the mock server to start listening
	time.Sleep(1 * time.Second)
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

func getInfo(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	fmt.Fprintf(w, `{"info":{"title":"%s","description":"%s","version":"%s"},"host":"%s"}`,
		mockTitle, mockDescription, mockVersion, mockHost)
}

func getBuckets(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	fmt.Fprintf(w, mockGetBucketsResponse)
}
