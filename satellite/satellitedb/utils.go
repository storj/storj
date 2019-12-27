// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"database/sql/driver"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/common/storj"
	"storj.io/storj/private/dbutil"
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

// uuidScan used to represent uuid scan struct
type uuidScan struct {
	uuid *uuid.UUID
}

// Scan is used to wrap logic of db scan with uuid conversion
func (s *uuidScan) Scan(src interface{}) (err error) {
	b, ok := src.([]byte)

	if !ok {
		return Error.New("unexpected type %T for uuid", src)
	}

	*s.uuid, err = dbutil.BytesToUUID(b)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}
