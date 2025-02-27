// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/zeebo/mwc"

	"storj.io/common/memory"
)

// without preallocate
//
// nset        | [4]byte                   | [5]byte
// ------------------------------------------------------------------
// 6_319_580   | 93.7 MiB (15.5/8.05 per)  | 95.0 MiB (15.8/9.00 per)
// 75_955_551  | 1.5 GiB  (21.1/8.56 per)  | 1.5 GiB  (21.2/9.00 per)
// 400_000_000 | 8.3 GiB  (22.4/10.83 per) | 5.9 GiB  (15.8/9.01 per)
//
// with preallocate
//
// nset        | [4]byte                   | [5]byte
// ------------------------------------------------------------------
// 6_319_580   | 75.6 MiB (12.5/8.05 per)  | 80.3 MiB (13.3/9.00 per)
// 75_955_551  | 1.2 GiB  (17.6/8.56 per)  | 1.3 GiB  (17.7/9.00 per)
// 400_000_000 | 7.2 GiB  (19.3/10.8 per)  | 5.0 GiB  (13.5/9.01 per)

func TestMemoryBasicMap(t *testing.T) {
	t.SkipNow()

	type shortKey = [4]byte

	// const nset = 6_319_580
	// const nset = 75_955_551
	const nset = 400_000_000

	stats := func() (ms runtime.MemStats) {
		runtime.GC()
		runtime.ReadMemStats(&ms)
		return ms
	}

	rng := mwc.Rand()
	newKey := func() (k Key) {
		_, _ = rng.Read(k[:])
		return k
	}

	before := stats()
	data := make(map[shortKey][4]byte, nset)
	collisions := make(map[Key][4]byte)
	for i := 0; i < nset; i++ {
		key := newKey()
		short := *(*shortKey)(key[:])

		existing, ok := data[short]
		switch {
		case !ok:
			data[short] = [4]byte{0: 1}

		case existing == [4]byte{}:
			collisions[key] = [4]byte{0: 1}

		default:
			data[short] = [4]byte{}
			collisions[key] = [4]byte{0: 1}

			exKey := newKey()
			*(*shortKey)(exKey[:]) = short
			collisions[exKey] = [4]byte{0: 1}
		}
	}
	after := stats()

	runtime.KeepAlive(data)
	runtime.KeepAlive(collisions)

	used := after.HeapInuse - before.HeapInuse
	opti := len(data)*(4+len(shortKey{})) + len(collisions)*(4+len(Key{}))

	fmt.Println("used", memory.Size(used), used)
	fmt.Println("opti", memory.Size(opti), opti)
	fmt.Println("fact", float64(used)/float64(opti))
	fmt.Println("nset", nset)
	fmt.Println("ents", len(data))
	fmt.Println("cols", len(collisions))
	fmt.Println("per ", float64(used)/float64(nset))
	fmt.Println("oper", float64(opti)/float64(nset))
}
