// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

/*
Package piecetransfer contains code meant to deal with transferring pieces
from one node to another. This does not happen under typical circumstances,
but may happen when a node wants to become unavailable in a "clean" way.
(Graceful exit, planned downtime)
*/
package piecetransfer
