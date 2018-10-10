// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"

	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/transport"
)

// Service helps coordinate Cursor and Verifier to run the audit process continuously
type Service struct {
	Cursor   *Cursor
	Verifier *Verifier
	statdb   *statdb
}

// NewService instantiates a Service with access to a Cursor and Verifier
func NewService(pointers pdbclient.Client, transport transport.Client, overlay overlay.Client, id provider.FullIdentity) *Service {
	cursor := NewCursor(pointers)
	verifier := NewVerifier(transport, overlay, id)
	return &Service{Cursor: cursor, Verifier: verifier}
}

// Run calls Cursor and Verifier to continuously request random pointers, then verify data correctness at
// a random stripe within a segment
func (service *Service) Run(ctx context.Context) (err error) {
	// TODO(James): make this function run indefinitely instead of once
	stripe, err := service.Cursor.NextStripe(ctx)
	if err != nil {
		return err
	}
	failedNodes, err := service.Verifier.verify(ctx, stripe.Index, stripe.Segment)
	if err != nil {
		return err
	}
	for _, fail := range failedNodes {
		service.statdb.RecordFailedAudit(fail)
	}
	return nil
}

type statdb struct{}

func (db *statdb) RecordFailedAudit(*pb.Node) {}
