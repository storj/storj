package main

import (
	"github.com/stretchr/testify/require"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/lib/uplink"
	"testing"
)


// TODO: Start up test planet and call these from bash instead
func TestCProjectTests(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)

	project := newProject(t, planet)
	apikey := newAPIKey(t, ctx, planet, project.ID)
	satelliteAddr := planet.Satellites[0].Addr()

	envVars := []string{
		"SATELLITE_ADDR=" + satelliteAddr,
		"APIKEY=" + apikey,
	}

	runCTest(t, ctx, "project_test.c", envVars...)
}

func TestCreateBucket(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)

	consoleProject := newProject(t, planet)
	consoleAPIKey := newAPIKey(t, ctx, planet, consoleProject.ID)
	satelliteAddr := planet.Satellites[0].Addr()

	var cErr Cchar

	goUplink := newUplinkInsecure(t, ctx)
	defer ctx.Check(goUplink.Close)

	apikey, err := uplink.ParseAPIKey(consoleAPIKey)
	require.NoError(t, err)
	require.NotEmpty(t, apikey)

	// TODO: test options
	project, err := goUplink.OpenProject(ctx, satelliteAddr, apikey, nil)
	require.NoError(t, err)
	require.NotNil(t, project)

	cProjectRef := CProjectRef(structRefMap.Add(project))

	{
		t.Log("nil config")
		_ = CreateBucket(cProjectRef, stringToCCharPtr("TestBucket"), nil, &cErr)
		require.Empty(t, cCharToGoString(cErr))
	}
	// TODO: test more config values
}

func TestOpenBucket(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)
}

func TestDeleteBucket(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)
}

func TestListBuckets(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)
}

func TestGetBucketInfo(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)
}

func TestCloseProject(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)
}