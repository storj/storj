// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet_test

import (
	"context"
	"strconv"
	"testing"

	"storj.io/storj/internal/testplanet"
)

func TestBasic(t *testing.T) {
	t.Log("New")
	planet, err := testplanet.New(t, 2, 4, 1)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		t.Log("Shutdown")
		err = planet.Shutdown()
		if err != nil {
			t.Fatal(err)
		}
	}()

	t.Log("Start")
	planet.Start(context.Background())

	for _, satellite := range planet.Satellites {
		t.Log("SATELLITE", satellite.ID(), satellite.Addr())
	}
	for _, storageNode := range planet.StorageNodes {
		t.Log("STORAGE", storageNode.ID(), storageNode.Addr())
	}
	for _, uplink := range planet.Uplinks {
		t.Log("UPLINK", uplink.ID(), uplink.Addr())
	}

	// Example of using pointer db
	client, err := planet.Uplinks[0].DialPointerDB(planet.Satellites[0], "apikey")
	if err != nil {
		t.Fatal(err)
	}

	message := client.SignedMessage()
	t.Log(message)

	nodeClient, err := planet.StorageNodes[0].NewNodeClient()
	if err != nil {
		t.Fatal(err)
	}

	// ping a satellite
	_, err = nodeClient.Ping(context.Background(), planet.Satellites[0].Local())
	if err != nil {
		t.Fatal(err)
	}

	// ping a storage node
	_, err = nodeClient.Ping(context.Background(), planet.StorageNodes[1].Local())
	if err != nil {
		t.Fatal(err)
	}
}

func BenchmarkCreate(b *testing.B) {
	storageNodes := []int{4, 10, 100}
	for _, count := range storageNodes {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			ctx := context.Background()
			for i := 0; i < b.N; i++ {
				planet, err := testplanet.New(nil, 1, count, 1)
				if err != nil {
					b.Fatal(err)
				}

				planet.Start(ctx)

				err = planet.Shutdown()
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
