// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package agreementsender

import (
	"flag"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/piecestore/psserver/psdb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/pkg/utils"
)

var (
	defaultCheckInterval = flag.Duration("piecestore.agreementsender.check-interval", time.Hour, "number of seconds to sleep between agreement checks")
	defaultOverlayAddr   = flag.String("piecestore.agreementsender.overlay-addr", "127.0.0.1:7777", "Overlay Address")

	// ASError wraps errors returned from agreementsender package
	ASError = errs.Class("agreement sender error")
)

// AgreementSender maintains variables required for reading bandwidth agreements from a DB and sending them to a Payers
type AgreementSender struct {
	DB        *psdb.DB
	log       *zap.Logger
	overlay   overlay.Client
	transport transport.Client
	identity  *provider.FullIdentity
	errs      []error
}

// Initialize the Agreement Sender
func Initialize(log *zap.Logger, DB *psdb.DB, identity *provider.FullIdentity) (*AgreementSender, error) {
	overlay, err := overlay.NewClient(identity, *defaultOverlayAddr)
	if err != nil {
		return nil, err
	}
	return &AgreementSender{DB: DB, log: log, transport: transport.NewClient(identity), identity: identity, overlay: overlay}, nil
}

// Run the afreement sender with a context to cehck for cancel
func (as *AgreementSender) Run(ctx context.Context) error {
	as.log.Info("AgreementSender is starting up")

	type agreementGroup struct {
		satellite  storj.NodeID
		agreements []*psdb.Agreement
	}

	c := make(chan *agreementGroup, 1)

	ticker := time.NewTicker(*defaultCheckInterval)
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			agreementGroups, err := as.DB.GetBandwidthAllocations()
			if err != nil {
				as.log.Error("Agreementsender could not retrieve bandwidth allocations", zap.Error(err))
				continue
			}
			// Send agreements in groups by satellite id to open less connections
			for satellite, agreements := range agreementGroups {
				c <- &agreementGroup{satellite, agreements}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return utils.CombineErrors(as.errs...)
		case agreementGroup := <-c:
			go func() {
				as.log.Info("Sending agreements to satellite", zap.Int("number of agreements", len(agreementGroup.agreements)), zap.String("sat node id", agreementGroup.satellite.String()))

				// Get satellite ip from overlay by Lookup agreementGroup.satellite
				satellite, err := as.overlay.Lookup(ctx, agreementGroup.satellite)
				if err != nil {
					as.log.Error("Agreementsender could not find satellite", zap.Error(err))
					return
				}

				// Create client from satellite ip
				//conn, err := as.transport.DialNode(ctx, satellite)
				identOpt, err := as.identity.DialOption(storj.NodeID{})
				if err != nil {
					zap.S().Error(err)
					return
				}
				conn, err := grpc.Dial(satellite.GetAddress().Address, identOpt)

				if err != nil {
					as.log.Error("Agreementsender could not dial satellite", zap.Error(err))
					return
				}

				client := pb.NewBandwidthClient(conn)
				for _, agreement := range agreementGroup.agreements {
					msg := &pb.RenterBandwidthAllocation{
						Data:      agreement.Agreement,
						Signature: agreement.Signature,
					}
					// Send agreement to satellite
					r, err := client.BandwidthAgreements(ctx, msg)
					if err != nil || r.GetStatus() != pb.AgreementsSummary_OK {
						as.log.Error("Agreementsender failed to send agreement to satellite", zap.Error(err))
						return
					}
					// Delete from PSDB by signature
					if err = as.DB.DeleteBandwidthAllocationBySignature(agreement.Signature); err != nil {
						as.log.Error("Agreementsender failed to delete bandwidth allocation", zap.Error(err))
						return
					}
				}
				//as.log.Error("Agreementsender failed to close connection", zap.Error(conn.Close()))
			}()
		}
	}
}
