// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pdbclient

import (
	"github.com/zeebo/errs"
)

//go:generate mockgen -destination=pdbclient_mock_test.go -package=pdbclient storj.io/storj/pkg/pb PointerDBClient
//go:generate mockgen -destination=mocks/mock_client.go -package=mock_pointerdb storj.io/storj/pkg/pointerdb/pdbclient Client

// Error is the pdbclient error class
var Error = errs.Class("pointerdb client error")
