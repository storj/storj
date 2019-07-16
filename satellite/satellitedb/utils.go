// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"database/sql/driver"
	"time"

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

// null time helps convert nil to time.Time
type nullTime struct {
	time.Time
	Valid bool
}

// Scan implements the Scanner interface.
func (nt *nullTime) Scan(value interface{}) error {
	nt.Time, nt.Valid = value.(time.Time)
	return nil
}

// Value implements the driver Valuer interface.
func (nt nullTime) Value() (driver.Value, error) {
	if !nt.Valid {
		return nil, nil
	}
	return nt.Time, nil
}

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
