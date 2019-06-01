package main

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"storj.io/storj/lib/uplink"
	"testing"

	"storj.io/storj/internal/testcontext"
)

// TODO: Start up test planet and call these from bash instead
func TestCCommonTests(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)

	runCTest(t, ctx, "common_test.c")
}

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
