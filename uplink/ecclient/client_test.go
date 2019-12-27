// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package ecclient

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/common/pb"
	"storj.io/storj/private/teststorj"
)

func TestUnique(t *testing.T) {
	limits := make([]*pb.AddressedOrderLimit, 4)
	for i := 0; i < len(limits); i++ {
		limits[i] = &pb.AddressedOrderLimit{
			Limit: &pb.OrderLimit{
				StorageNodeId: teststorj.NodeIDFromString(fmt.Sprintf("node-%d", i)),
			},
		}
	}

	for i, tt := range []struct {
		limits []*pb.AddressedOrderLimit
		unique bool
	}{
		{nil, true},
		{[]*pb.AddressedOrderLimit{}, true},
		{[]*pb.AddressedOrderLimit{limits[0]}, true},
		{[]*pb.AddressedOrderLimit{limits[0], limits[1]}, true},
		{[]*pb.AddressedOrderLimit{limits[0], limits[0]}, false},
		{[]*pb.AddressedOrderLimit{limits[0], limits[1], limits[0]}, false},
		{[]*pb.AddressedOrderLimit{limits[1], limits[0], limits[0]}, false},
		{[]*pb.AddressedOrderLimit{limits[0], limits[0], limits[1]}, false},
		{[]*pb.AddressedOrderLimit{limits[2], limits[0], limits[1]}, true},
		{[]*pb.AddressedOrderLimit{limits[2], limits[0], limits[3], limits[1]}, true},
		{[]*pb.AddressedOrderLimit{limits[2], limits[0], limits[2], limits[1]}, false},
		{[]*pb.AddressedOrderLimit{limits[1], limits[0], limits[3], limits[1]}, false},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		assert.Equal(t, tt.unique, unique(tt.limits), errTag)
	}
}
