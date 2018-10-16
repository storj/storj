// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"

	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/transport"
)

// Service helps coordinate Cursor and Verifier to run the audit process continuously
type Service struct {
	Cursor   *Cursor
	Verifier *Verifier
	Reporter reporter
}

// Config contains configurable values for audit service
type Config struct {
	StatDBPort       string `help:"port to contact statDB client" default:":9090"`
	MaxRetriesStatDB int    `help:"max number of times to attempt updating a statdb batch" default:"3"`
}

// NewService instantiates a Service with access to a Cursor and Verifier
func NewService(ctx context.Context, statDBPort string, maxRetries int, pointers pdbclient.Client, transport transport.Client, overlay overlay.Client,
	id provider.FullIdentity) (service *Service, err error) {
	cursor := NewCursor(pointers)
	verifier := NewVerifier(transport, overlay, id)
	reporter, err := NewReporter(ctx, statDBPort, maxRetries)
	if err != nil {
		return nil, err
	}
	return &Service{Cursor: cursor, Verifier: verifier, Reporter: reporter}, nil
}

// Run calls Cursor and Verifier to continuously request random pointers, then verify data correctness at
// a random stripe within a segment
func (service *Service) Run(ctx context.Context) (err error) {
	// TODO(James): make this function run indefinitely instead of once
	stripe, err := service.Cursor.NextStripe(ctx)
	if err != nil {
		return err
	}
	verifiedNodes, err := service.Verifier.verify(ctx, stripe.Index, stripe.Segment)
	if err != nil {
		return err
	}
	err = service.Reporter.RecordAudits(ctx, verifiedNodes)
	// TODO: if Error.Has(err) then log the error because it means not all node stats updated
	if !Error.Has(err) && err != nil {
		return err
	}
	return nil
}
