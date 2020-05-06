// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"database/sql/driver"

	"storj.io/common/storj"
)

type postgresNodeIDList storj.NodeIDList

// Value converts a NodeIDList to a postgres array
func (nodes postgresNodeIDList) Value() (driver.Value, error) {
	const hextable = "0123456789abcdef"

	if nodes == nil {
		return nil, nil
	}
	if len(nodes) == 0 {
		return []byte("{}"), nil
	}

	var wp, x int
	out := make([]byte, 2+len(nodes)*(6+storj.NodeIDSize*2)-1)

	x = copy(out[wp:], []byte(`{"\\x`))
	wp += x

	for i := range nodes {
		for _, v := range nodes[i] {
			out[wp] = hextable[v>>4]
			out[wp+1] = hextable[v&0xf]
			wp += 2
		}

		if i+1 < len(nodes) {
			x = copy(out[wp:], []byte(`","\\x`))
			wp += x
		}
	}

	x = copy(out[wp:], `"}`)
	wp += x

	if wp != len(out) {
		panic("unreachable")
	}

	return out, nil
}
