package testplanet

import (
	"context"
	"strconv"
	"testing"
)

func TestBasic(t *testing.T) {
	planet, err := New(ctx, 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = planet.Shutdown()
		if err != nil {
			t.Fatal(err)
		}
	}()

	err := planet.Start()
	if err != nil {
		t.Fatal(err)
	}
}

func BenchmarkCreate(b *testing.B) {
	storageNodes := []int{4, 10, 100}
	for _, count := range storageNodes {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			ctx := context.Context
			for i := 0; i < b.N; i++ {
				planet, err := New(ctx, 1, 100)
				if err != nil {
					b.Fatal(err)
				}

				planet.Start()

				err = planet.Shutdown()
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
