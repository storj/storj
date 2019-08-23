// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"math"
	"reflect"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/audit"
)

func TestRandomPaths(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		type pathCount struct {
			path  storj.Path
			count int
		}

		satellite := planet.Satellites[0]
		service := satellite.Audit.ReservoirService

		// create 100 reservoirs generated from random
		service.Reservoirs = make(map[storj.NodeID]*audit.Reservoir, 100)
		for i := 0; i < 100; i++ {
			path := strconv.Itoa(i)
			nodeID := storj.NodeID{byte(i)}
			service.Reservoirs[nodeID] = &audit.Reservoir{Paths: [2]storj.Path{path}}
			i++
		}

		// get count of items picked at random
		nodesToSelect := 100
		uniquePathCounted := []pathCount{}
		pathCounter := []pathCount{}

		for i := 0; i < nodesToSelect; i++ {
			randomReservoir, err := service.GetRandomReservoir()
			require.NoError(t, err)

			if randomReservoir == nil {
				continue
			}
			randomPath := audit.GetRandomPath(randomReservoir)
			val := pathCount{path: randomPath, count: 1}
			pathCounter = append(pathCounter, val)
		}

		// get a count for paths in PathsToAudit
		for _, pc := range pathCounter {
			skip := false
			for i, up := range uniquePathCounted {
				if reflect.DeepEqual(pc.path, up.path) {
					up.count++
					uniquePathCounted[i] = up
					skip = true
					break
				}
			}
			if !skip {
				uniquePathCounted = append(uniquePathCounted, pc)
			}
		}

		// Section: binomial test for randomness
		n := float64(100) // events
		p := float64(.10) // theoretical probability of getting  1/10 paths
		m := n * p
		s := math.Sqrt(m * (1 - p)) // binomial distribution

		// if values fall outside of the critical values of test statistics (ie Z value)
		// in a 2-tail test
		// we can assume, 95% confidence, it's not sampling according to a 10% probability
		for _, v := range uniquePathCounted {
			z := (float64(v.count) - m) / s
			if z <= -1.96 || z >= 1.96 {
				t.Log(false)
			} else {
				t.Log(true)
			}
		}

	})
}
