// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testsuite

import (
	"path"
	"strconv"
	"testing"

	"golang.org/x/sync/errgroup"

	"storj.io/common/testcontext"
	"storj.io/storj/storage"
)

// RunBenchmarks runs common storage.KeyValueStore benchmarks
func RunBenchmarks(b *testing.B, store storage.KeyValueStore) {
	var words = []string{
		"alpha", "beta", "gamma", "delta", "iota", "kappa", "lambda", "mu",
		"άλφα", "βήτα", "γάμμα", "δέλτα", "έψιλον", "ζήτα", "ήτα", "θήτα", "ιώτα", "κάππα", "λάμδα", "μυ",
		"nu", "xi", "omicron", "pi", "rho", "sigma", "tau", "upsilon", "phi", "chi", "psi", "omega",
		"νυ", "ξι", "όμικρον", "πι", "ρώ", "σίγμα", "ταυ", "ύψιλον", "φι", "χι", "ψι", "ωμέγα",
	}

	words = words[:20] // branching factor

	var items storage.Items

	k := 0
	for _, a := range words {
		for _, b := range words {
			for _, c := range words {
				items = append(items, storage.ListItem{
					Key:   storage.Key(path.Join(a, b, c)),
					Value: storage.Value(strconv.Itoa(k)),
				})
				k++
			}
		}
	}

	ctx := testcontext.New(b)
	defer ctx.Cleanup()

	defer cleanupItems(b, ctx, store, items)

	b.Run("Put", func(b *testing.B) {
		b.SetBytes(int64(len(items)))
		for k := 0; k < b.N; k++ {
			var group errgroup.Group
			for _, item := range items {
				key := item.Key
				value := item.Value

				group.Go(func() error {
					return store.Put(ctx, key, value)
				})
			}

			if err := group.Wait(); err != nil {
				b.Fatalf("Put: %v", err)
			}
		}
	})

	b.Run("Get", func(b *testing.B) {
		b.SetBytes(int64(len(items)))
		for k := 0; k < b.N; k++ {
			for _, item := range items {
				_, err := store.Get(ctx, item.Key)
				if err != nil {
					b.Fatal(err)
				}
			}
		}
	})

	b.Run("ListV2 5", func(b *testing.B) {
		b.SetBytes(int64(len(items)))
		for k := 0; k < b.N; k++ {
			_, _, err := storage.ListV2(ctx, store, storage.ListOptions{
				StartAfter: storage.Key("gamma"),
				Limit:      5,
			})
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
