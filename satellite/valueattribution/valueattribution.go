// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package valueattribution

import (
	"context"
	"time"
)

// PartnerInfo describing connector/partner key info in the database
type PartnerInfo struct {
	PartnerID  []byte
	BucketName []byte
	CreatedAt  time.Time
}

// DB implements the database for value attribution table
type DB interface {
	// Get retrieves partner id using bucket name
	Get(ctx context.Context, buckname []byte) (*PartnerInfo, error)
	// Insert creates and stores new ConnectorKeyInfo
	Insert(ctx context.Context, info *PartnerInfo) (*PartnerInfo, error)
}
