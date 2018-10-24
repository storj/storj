package testplanet

import (
	"context"
	"strconv"
	"testing"

	"storj.io/storj/pkg/utils"
)

func TestBasic(t *testing.T) {
	t.Log("New")
	planet, err := New(1, 4)
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
	err = planet.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}

func BenchmarkCreate(b *testing.B) {
	storageNodes := []int{4, 10, 100}
	for _, count := range storageNodes {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				planet, err := New(1, 100)
				if err != nil {
					b.Fatal(err)
				}

				err = utils.CombineErrors(
					planet.Start(context.Background()),
					planet.Shutdown(),
				)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
