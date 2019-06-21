// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"bytes"
	"database/sql/driver"

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

// splitBucketID takes a bucketID, splits on /, and returns a projectID and bucketName
func splitBucketID(bucketID []byte) (projectID *uuid.UUID, bucketName []byte, err error) {
	pathElements := bytes.Split(bucketID, []byte("/"))
	bucketName = pathElements[1]
	projectID, err = uuid.Parse(string(pathElements[0]))
	if err != nil {
		return nil, nil, err
	}
	return projectID, bucketName, nil
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
