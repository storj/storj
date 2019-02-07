// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package uplinkdb

import (
	"context"
	"crypto"
	"crypto/ecdsa"

	"storj.io/storj/pkg/storj"
)

// DB stores uplink public keys.
type DB interface {
	// SavePublicKey adds a new bandwidth agreement.
	SavePublicKey(context.Context, storj.NodeID, crypto.PublicKey) error
	// GetPublicKey gets the public key of uplink corresponding to uplink id
	GetPublicKey(context.Context, storj.NodeID) (*ecdsa.PublicKey, error)
}

// // Server is an implementation of the pb.BandwidthServer interface
// type Server struct {
// 	db     DB
// 	pkey   crypto.PublicKey
// 	logger *zap.Logger
// }

// // Agreement is a struct that contains a uplinks agreement info
// type Agreement struct {
// 	ID        storj.NodeID // uplink id
// 	PublicKey crypto.PublicKey
// }

// // NewServer creates instance of Server
// func NewServer(db DB, logger *zap.Logger, pkey crypto.PublicKey) *Server {
// 	return &Server{
// 		db:     db,
// 		logger: logger,
// 		pkey:   pkey,
// 	}
// }
