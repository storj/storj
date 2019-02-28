// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package ecclient

import (
	"github.com/zeebo/errs"
)

//go:generate mockgen -destination=psclient_mock_test.go -package=ecclient storj.io/storj/pkg/piecestore/psclient Client
//go:generate mockgen -destination=transportclient_mock_test.go -package=ecclient storj.io/storj/pkg/transport Client
//go:generate mockgen -destination=mocks/mock_client.go -package=mocks storj.io/storj/pkg/storage/ec Client

// Error is the errs class of standard Ranger errors
var Error = errs.Class("ecclient error")
