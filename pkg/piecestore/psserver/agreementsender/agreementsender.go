// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package agreementsender

import (
	"flag"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/net/context"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/piecestore/psserver/psdb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
)

var (
	//todo: cache kad responses if this interval is very small
	defaultCheckInterval = flag.Duration("piecestore.agreementsender.check-interval", time.Hour, "duration to sleep between agreement checks")
	// ASError wraps errors returned from agreementsender package
	ASError = errs.Class("agreement sender error")
)

// AgreementSender maintains variables required for reading bandwidth agreements from a DB and sending them to a Payers
type AgreementSender struct {
	DB        *psdb.DB
	log       *zap.Logger
	transport transport.Client
	kad       *kademlia.Kademlia
}

// New creates an Agreement Sender
func New(log *zap.Logger, DB *psdb.DB, identity *provider.FullIdentity, kad *kademlia.Kademlia) *AgreementSender {
	return &AgreementSender{DB: DB, log: log, transport: transport.NewClient(identity), kad: kad}
}

// Run the agreement sender with a context to check for cancel
func (as *AgreementSender) Run(ctx context.Context) {
	//todo:  we likely don't want to stop on err, but consider returning errors via a channel
	ticker := time.NewTicker(*defaultCheckInterval)
	defer ticker.Stop()
	for {
		as.log.Debug("AgreementSender is running", zap.Duration("duration", *defaultCheckInterval))
		agreementGroups, err := as.DB.GetBandwidthAllocations()
		if err != nil {
			as.log.Error("Agreementsender could not retrieve bandwidth allocations", zap.Error(err))
			continue
		}
		for satellite, agreements := range agreementGroups {
			as.sendAgreementsToSatellite(ctx, satellite, agreements)
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			as.log.Debug("AgreementSender is shutting down", zap.Error(ctx.Err()))
			return
		}
	}
}

func (as *AgreementSender) sendAgreementsToSatellite(ctx context.Context, satID storj.NodeID, agreements []*psdb.Agreement) {
	as.log.Info("Sending agreements to satellite", zap.Int("number of agreements", len(agreements)), zap.String("satellite id", satID.String()))
	// Get satellite ip from kademlia
	satellite, err := as.kad.FindNode(ctx, satID)
	if err != nil {
		as.log.Error("Agreementsender could not find satellite", zap.Error(err))
		return
	}
	// Create client from satellite ip
	conn, err := as.transport.DialNode(ctx, &satellite)
	if err != nil {
		as.log.Error("Agreementsender could not dial satellite", zap.Error(err))
		return
	}
	client := pb.NewBandwidthClient(conn)
	defer func() {
		err := conn.Close()
		if err != nil {
			as.log.Error("Agreementsender failed to close connection", zap.Error(err))
		}
	}()

	for _, agreement := range agreements {
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
}
