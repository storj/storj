// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"testing"

	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/storj/satellite/overlay"
)

func TestEmitEventkitEvent(t *testing.T) {
	ctx := testcontext.New(t)
	emitEventkitEvent(ctx, &pb.CheckInRequest{
		Address: "127.0.0.1:234",
	}, false, false, overlay.NodeCheckInInfo{})
}
