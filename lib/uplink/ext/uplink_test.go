package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/lib/uplink"
)

// TODO: Start up test planet and call these from bash instead
func TestNewUplink(t *testing.T) {
	var cErr CCharPtr

	cUplinkRef := NewUplink(&cErr)
	require.Empty(t, cCharToGoString(cErr))
	require.NotEmpty(t, cUplinkRef)
}

func TestNewUplinkInsecure(t *testing.T) {
	var cErr CCharPtr

	cUplinkRef := NewUplinkInsecure(&cErr)
	require.Empty(t, cCharToGoString(cErr))
	require.NotEmpty(t, cUplinkRef)
}

func TestOpenProject(t *testing.T) {
	ctx := testcontext.New(t)
	planet := startTestPlanet(t, ctx)

	var cErr CCharPtr
	satelliteAddr := planet.Satellites[0].Addr()
	apikeyStr := "testapikey123"

	goUplink := newUplinkInsecure(t, ctx)

	apikey, err := uplink.ParseAPIKey(apikeyStr)
	require.NoError(t, err)
	require.NotEmpty(t, apikey)

	cUplinkRef := CUplinkRef(structRefMap.Add(goUplink))
	cAPIKeyRef := CAPIKeyRef(structRefMap.Add(apikey))

	cProjectRef := OpenProject(cUplinkRef, stringToCCharPtr(satelliteAddr), cAPIKeyRef, &cErr)
	assert.Empty(t, cCharToGoString(cErr))
	assert.NotEmpty(t, cProjectRef)
}

func TestCloseUplink(t *testing.T) {
	ctx := testcontext.New(t)
	var cErr CCharPtr

	// TODO: test other config values?
	goUplink, err := uplink.NewUplink(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, goUplink)

	cUplinkRef := CUplinkRef(structRefMap.Add(goUplink))

	CloseUplink(cUplinkRef, &cErr)
	require.Empty(t, cCharToGoString(cErr))
}
