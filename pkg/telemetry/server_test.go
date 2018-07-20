// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package telemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListen_NilOnBadAddress(t *testing.T) {
	server, errListen := Listen("11")
	defer func() {
		if server != nil {
			server.Close()
		}
	}()

	assert.Nil(t, server)
	assert.Error(t, errListen)
}

func TestServe_ReturnErrorOnConnFail(t *testing.T) {
	server, _ := Listen("127.0.0.1:0")
	defer func() {
		if server != nil && server.conn != nil {
			server.Close()
		}
	}()

	server.conn.Close()
	server.conn = nil

	errServe := server.Serve(nil, nil)

	assert.EqualError(t, errServe, "telemetry error: invalid conn: <nil>")
}

func TestListenAndServe_ReturnErrorOnListenFails(t *testing.T) {
	err := ListenAndServe(nil, "1", nil)

	assert.Error(t, err)
}
