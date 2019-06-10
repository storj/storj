// +build ignore

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/lib/uplink"
)

import "C"

func TestParseAPIKey(t *testing.T) {
	var cerr *C.char
	apikey := "testapikey123"
	apikeyref := ParseAPIKey(C.CString(apikey), &cerr)
	require.Empty(t, C.GoString(cerr))

	apikey, ok := universe.Get(Ref(apikeyref)).(uplink.APIKey)
	require.True(t, ok)
	require.NotEmpty(t, apikey)

	assert.Equal(t, apikey, apikey.Serialize())
}

func TestSerialize(t *testing.T) {
	apikey := "testapikey123"
	apikey, err := uplink.ParseAPIKey(apikey)
	require.NoError(t, err)
	require.NotEmpty(t, apikey)

	apikeyref := CAPIKeyRef(universe.Add(apikey))
	require.NotEmpty(t, apikeyref)

	cAPIKey := Serialize(apikeyref)

	assert.Equal(t, apikey, cCharToGoString(cAPIKey))
}
