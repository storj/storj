// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/lib/uplink"
)

func TestParseAPIKey(t *testing.T) {
	var cErr Cchar
	apikeyString := "testapikey123"
	cAPIKeyRef := ParseAPIKey(stringToCCharPtr(apikeyString), &cErr)
	require.Empty(t, cCharToGoString(cErr))

	apikey, ok := structRefMap.Get(token(cAPIKeyRef)).(uplink.APIKey)
	require.True(t, ok)
	require.NotEmpty(t, apikey)

	assert.Equal(t, apikeyString, apikey.Serialize())
}

func TestSerialize(t *testing.T) {
	apikeyString := "testapikey123"
	apikey, err := uplink.ParseAPIKey(apikeyString)
	require.NoError(t, err)
	require.NotEmpty(t, apikey)

	cAPIKeyRef := CAPIKeyRef(structRefMap.Add(apikey))
	require.NotEmpty(t, cAPIKeyRef)

	cAPIKey := Serialize(cAPIKeyRef)

	assert.Equal(t, apikeyString, cCharToGoString(cAPIKey))
}
