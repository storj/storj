package testplanet_test

import (
	"context"
	"strconv"
	"testing"

	"storj.io/storj/internal/testplanet"
)

func TestBasic(t *testing.T) {
	t.Log("New")
	planet, err := testplanet.New(2, 4)
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

	for i := 0; i < planet.SatelliteCount(); i++ {
		t.Log("SATELLITE", planet.Satellite(i).ID(), planet.Satellite(i).Addr())
	}

	for i := 0; i < planet.StorageNodeCount(); i++ {
		t.Log("STORAGE", planet.StorageNode(i).ID(), planet.StorageNode(i).Addr())
	}
}

func BenchmarkCreate(b *testing.B) {
	storageNodes := []int{4, 10, 100}
	for _, count := range storageNodes {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			ctx := context.Background()
			for i := 0; i < b.N; i++ {
				planet, err := testplanet.New(1, 100)
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
