package storage

import (
	"path"
	"strconv"
	"testing"
)

func RunBenchmarks(b *testing.B, store KeyValueStore) {
	var words = []string{
		"alpha", "beta", "gamma", "delta", "iota", "kappa", "lambda", "mu",
		"άλφα", "βήτα", "γάμμα", "δέλτα", "έψιλον", "ζήτα", "ήτα", "θήτα", "ιώτα", "κάππα", "λάμδα", "μυ",
		"nu", "xi", "omicron", "pi", "rho", "sigma", "tau", "upsilon", "phi", "chi", "psi", "omega",
		"νυ", "ξι", "όμικρον", "πι", "ρώ", "σίγμα", "ταυ", "ύψιλον", "φι", "χι", "ψι", "ωμέγα",
	}

	words = words[:20] // branching factor

	var items Items

	k := 0
	for _, a := range words {
		for _, b := range words {
			for _, c := range words {
				items = append(items, ListItem{
					Key:   Key(path.Join(a, b, c)),
					Value: Value(strconv.Itoa(k)),
				})
				k++
			}
		}
	}

	defer cleanupItems(store, items)

	b.Run("Put", func(b *testing.B) {
		b.SetBytes(int64(len(items)))
		for k := 0; k < b.N; k++ {
			for _, item := range items {
				err := store.Put(item.Key, item.Value)
				if err != nil {
					b.Fatal(err)
				}
			}
		}
	})

	b.Run("Get", func(b *testing.B) {
		b.SetBytes(int64(len(items)))
		for k := 0; k < b.N; k++ {
			for _, item := range items {
				_, err := store.Get(item.Key)
				if err != nil {
					b.Fatal(err)
				}
			}
		}
	})

	b.Run("ListV2 5", func(b *testing.B) {
		b.SetBytes(int64(len(items)))
		for k := 0; k < b.N; k++ {
			_, _, err := ListV2(store, ListOptions{
				StartAfter: Key("gamma"),
				Limit:      5,
			})
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
