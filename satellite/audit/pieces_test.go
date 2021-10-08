// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/common/rpc"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/storj/satellite/audit"
)

func TestPieceAuditFromErr(t *testing.T) {
	cases := []struct {
		Error error
		Audit audit.PieceAudit
	}{
		{
			Error: nil,
			Audit: audit.PieceAuditSuccess,
		},
		{
			Error: rpc.Error.Wrap(context.DeadlineExceeded),
			Audit: audit.PieceAuditOffline,
		},
		{
			Error: rpc.Error.New("unknown rpc error"),
			Audit: audit.PieceAuditOffline,
		},
		{
			Error: rpc.Error.Wrap(rpcstatus.Error(rpcstatus.InvalidArgument, "rpc wrapped rpcstatus invalid arg error")),
			Audit: audit.PieceAuditOffline,
		},
		{
			Error: rpc.Error.Wrap(rpcstatus.Error(rpcstatus.NotFound, "rpc wrapped rpcstatus not found error")),
			// TODO: should not this be failure?
			Audit: audit.PieceAuditOffline,
		},
		{
			Error: rpcstatus.Error(rpcstatus.NotFound, "rpcstatus not found error"),
			Audit: audit.PieceAuditFailure,
		},
		{
			Error: context.DeadlineExceeded,
			Audit: audit.PieceAuditContained,
		},
		{
			Error: rpcstatus.Error(rpcstatus.DeadlineExceeded, "deadline exceeded rpcstatus error"),
			Audit: audit.PieceAuditContained,
		},
		{
			Error: errs.New("unknown error"),
			Audit: audit.PieceAuditUnknown,
		},
		{
			Error: rpcstatus.Error(rpcstatus.Unknown, "unknown rpcstatus error"),
			Audit: audit.PieceAuditUnknown,
		},
	}
	for _, c := range cases {
		pieceAudit := audit.PieceAuditFromErr(c.Error)
		require.Equal(t, c.Audit, pieceAudit, c.Error)
	}
}
