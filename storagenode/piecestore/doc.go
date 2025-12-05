// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

// Package piecestore contains the endpoint for responding to requests from the uplinks and satellites.
// It implements the upload and download protocol, where the counterpart is in uplink.
// It uses trust packages to establish trusted satellites.
package piecestore
