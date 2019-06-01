package main

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"storj.io/storj/lib/uplink"
	"testing"

	"storj.io/storj/internal/testcontext"
)

// TODO: Start up test planet and call these from bash instead
func TestCUplinkTests(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)

	project := newProject(t, planet)
	apikeyStr := newAPIKey(t, ctx, planet, project.ID)
	satelliteAddr := planet.Satellites[0].Addr()

	envVars := []string{
		"SATELLITE_ADDR=" + satelliteAddr,
		"APIKEY=" + apikeyStr,
	}

	runCTest(t, ctx, "uplink_test.c", envVars...)
}

func TestNewUplink(t *testing.T) {
	var cErr Cchar

	cUplinkRef := NewUplink(&cErr)
	require.Empty(t, cCharToGoString(cErr))
	require.NotEmpty(t, cUplinkRef)
}

func TestNewUplinkInsecure(t *testing.T) {
	var cErr Cchar

	cUplinkRef := NewUplinkInsecure(&cErr)
	require.Empty(t, cCharToGoString(cErr))
	require.NotEmpty(t, cUplinkRef)
}

func TestOpenProject(t *testing.T) {
	ctx := testcontext.New(t)
	planet := startTestPlanet(t, ctx)

	var cErr Cchar
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
	var cErr Cchar

	// TODO: test other config values?
	goUplink, err := uplink.NewUplink(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, goUplink)

	cUplinkRef := CUplinkRef(structRefMap.Add(goUplink))

	CloseUplink(cUplinkRef, &cErr)
	require.Empty(t, cCharToGoString(cErr))
}
