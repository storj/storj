package main_test

import (
	"testing"

	"storj.io/storj/internal/testcontext"
)

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
