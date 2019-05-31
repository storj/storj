package main

import (
	"storj.io/storj/internal/testcontext"
	"testing"
)


// TODO: Call these from bash
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

//func TestCreateBucket(t *testing.T) {
//	ctx := testcontext.New(t)
//	defer ctx.Cleanup()
//
//	planet := startTestPlanet(t, ctx)
//	defer ctx.Check(planet.Shutdown)
//
//	project := newProject(t, planet)
//	apikey := newAPIKey(t, ctx, planet, project.ID)
//	satelliteAddr := planet.Satellites[0].Addr()
//
//	var cErr Cchar
//
//	cUplinkRef := NewUplinkInsecure(&cErr)
//	require.Empty(t, cCharToGoString(cErr))
//
//	defer CloseUplink(cUplinkRef, &cErr)
//	require.Empty(t, cCharToGoString(cErr))
//
//	cAPIKeyRef := ParseAPIKey(stringToCCharPtr(apikey), &cErr)
//	require.Empty(t, cCharToGoString(cErr))
//
//
//	cProjectRef := OpenProject(cUplinkRef, stringToCCharPtr(satelliteAddr), cAPIKeyRef, &cErr)
//	require.Empty(t, cCharToGoString(cErr))
//
//	cBucketCfg := CBucketConfig{}
//
//	_ = CreateBucket(cProjectRef, stringToCCharPtr("TestBucket"), cBucketCfg, &cErr)
//	require.Empty(t, cCharToGoString(cErr))
//}

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