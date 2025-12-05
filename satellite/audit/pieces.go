// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/common/errs2"
	"storj.io/common/rpc"
	"storj.io/common/rpc/rpcstatus"
)

// PieceAudit is piece audit status.
type PieceAudit int

const (
	// PieceAuditUnknown is unknown piece audit.
	PieceAuditUnknown PieceAudit = iota
	// PieceAuditFailure is failed piece audit.
	PieceAuditFailure
	// PieceAuditOffline is offline node piece audit.
	PieceAuditOffline
	// PieceAuditContained is online but unresponsive node piece audit.
	PieceAuditContained
	// PieceAuditSuccess is successful piece audit.
	PieceAuditSuccess
)

// PieceAuditFromErr returns piece audit based on error.
func PieceAuditFromErr(err error) PieceAudit {
	if err == nil {
		return PieceAuditSuccess
	}

	if rpc.Error.Has(err) {
		switch {
		case errs.Is(err, context.DeadlineExceeded), errs2.IsRPC(err, rpcstatus.Unknown):
			return PieceAuditOffline
		default:
			// TODO: is this path not reachable?
			return PieceAuditUnknown
		}
	}

	switch {
	case errs2.IsRPC(err, rpcstatus.NotFound):
		return PieceAuditFailure
	case errs2.IsRPC(err, rpcstatus.DeadlineExceeded):
		return PieceAuditContained
	default:
		return PieceAuditUnknown
	}
}
