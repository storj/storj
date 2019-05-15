// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"database/sql/driver"
	"encoding/hex"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/storj"
)

// bytesToUUID is used to convert []byte to UUID
func bytesToUUID(data []byte) (uuid.UUID, error) {
	var id uuid.UUID

	copy(id[:], data)
	if len(id) != len(data) {
		return uuid.UUID{}, errs.New("Invalid uuid")
	}

	return id, nil
}

type nodeIDsArray storj.NodeIDList

// Value converts a NodeIDList to a postgres array
func (nodes nodeIDsArray) Value() (driver.Value, error) {
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
		x = hex.Encode(out[wp:], nodes[i].Bytes())
		wp += x
		if i+1 < len(nodes) {
			x = copy(out[wp:], []byte(`","\\x`))
			wp += x
		}
	}

	x = copy(out[wp:], `"}`)
	wp += x

	if wp != len(out) {
		println(wp, len(out))
		panic("unreachable")
	}

	return out, nil
}
