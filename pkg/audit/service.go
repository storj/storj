// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/pkg/utils"
)

// Service helps coordinate Cursor and Verifier to run the audit process continuously
type Service struct {
	Cursor   *Cursor
	Verifier *Verifier
	Reporter reporter
<<<<<<< HEAD
=======
	errs     []error
>>>>>>> rm redundant ctx
}

// Config contains configurable values for audit service
type Config struct {
	StatDBPort          string        `help:"port to contact statDB client" default:":9090"`
	MaxRetriesStatDB    int           `help:"max number of times to attempt updating a statdb batch" default:"3"`
	PointerDBPort       string        `help:"Pointers for a instantiation of a new service"`
	TransportClientPort string        `help:"Transport for a instantiation of a new service"`
	OverlayClientPort   string        `help:"Overlay for a instantiation of a new service"`
	ID                  string        `help:"ID for a instantiation of a new service"`
	Interval            time.Duration `help:"how frequently segements should audited" default:"30s"`
}

// Run runs the repairer with the configured values
func (c Config) Run(ctx context.Context, server *provider.Provider) (err error) {
	var pdb pdbclient.Client
	var tc transport.Client
	var oc overlay.Client
	var id provider.FullIdentity
	service, err := NewService(ctx, c.StatDBPort, c.MaxRetriesStatDB, pdb, tc, oc, id)
	if err != nil {
		return err
	}
	return service.Run(ctx, c.Interval)
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

	return &Service{Cursor: cursor,
		Verifier: verifier,
		Reporter: reporter,
<<<<<<< HEAD
=======
		errs:     []error{},
>>>>>>> rm redundant ctx
	}, nil
}

// Run calls Cursor and Verifier to continuously request random pointers, then verify data correctness at
// a random stripe within a segment
func (service *Service) Run(ctx context.Context, interval time.Duration) (err error) {
	defer mon.Task()(&ctx)(&err)

	zap.S().Info("Audit cron is starting up")
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errch := make(chan error)

	go func() {
		for {
			select {
			case <-ticker.C:
				stripe, err := service.Cursor.NextStripe(ctx)
				if err != nil {
<<<<<<< HEAD
					errch <- err
=======
					service.errs = append(service.errs, err)
>>>>>>> rm redundant ctx
					cancel()
				}

				authorization, err := service.Cursor.pointers.SignedMessage()
				if err != nil {
<<<<<<< HEAD
					errch <- err
					cancel()
				}

				verifiedNodes, err := service.Verifier.verify(ctx, stripe.Index, stripe.Segment, authorization)
				if err != nil {
					errch <- err
=======
					service.errs = append(service.errs, err)
>>>>>>> rm redundant ctx
					cancel()
				}
				err = service.Reporter.RecordAudits(ctx, verifiedNodes)
				// TODO: if Error.Has(err) then log the error because it means not all node stats updated
				if err != nil {
<<<<<<< HEAD
					errch <- err
=======
					service.errs = append(service.errs, err)
>>>>>>> rm redundant ctx
					cancel()
				}
			case <-ctx.Done():
				return
			}
		}
	}()

<<<<<<< HEAD
	// TODO(James): convert to collectErrors
	return utils.CollectErrors(errch, 5*time.Second)
=======
	return utils.CombineErrors(service.errs...)
>>>>>>> rm redundant ctx
}
