// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package agreementsender

import (
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/net/context"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/piecestore/psserver/psdb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
)

var (
	// ASError wraps errors returned from agreementsender package
	ASError = errs.Class("agreement sender error")
)

// AgreementSender maintains variables required for reading bandwidth agreements from a DB and sending them to a Payers
type AgreementSender struct { // TODO: rename to service
	DB            *psdb.DB
	log           *zap.Logger
	transport     transport.Client
	kad           *kademlia.Kademlia
	checkInterval time.Duration
}

// TODO: take transport instead of identity as argument

// New creates an Agreement Sender
func New(log *zap.Logger, DB *psdb.DB, tc transport.Client, kad *kademlia.Kademlia, checkInterval time.Duration) *AgreementSender {
	return &AgreementSender{DB: DB, log: log, transport: tc, kad: kad, checkInterval: checkInterval}
}

// Run the agreement sender with a context to check for cancel
func (as *AgreementSender) Run(ctx context.Context) error {
	//todo:  we likely don't want to stop on err, but consider returning errors via a channel
	ticker := time.NewTicker(as.checkInterval)
	defer ticker.Stop()
	for {
		as.log.Debug("AgreementSender is running", zap.Duration("duration", as.checkInterval))
		agreementGroups, err := as.DB.GetBandwidthAllocations()
		if err != nil {
			as.log.Error("Agreementsender could not retrieve bandwidth allocations", zap.Error(err))
			continue
		}
		// send agreement payouts
		for satellite, agreements := range agreementGroups {
			as.SendAgreementsToSatellite(ctx, satellite, agreements)
		}

		// Delete older payout irrespective of its status
		if err = as.DB.DeleteBandwidthAllocationPayouts(); err != nil {
			as.log.Error("Agreementsender failed to delete bandwidth allocation", zap.Error(err))
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

//SendAgreementsToSatellite uploads agreements to the satellite
func (as *AgreementSender) SendAgreementsToSatellite(ctx context.Context, satID storj.NodeID, agreements []*psdb.Agreement) {
	as.log.Info("Sending agreements to satellite", zap.Int("number of agreements", len(agreements)), zap.String("satellite id", satID.String()))
	// todo: cache kad responses if this interval is very small
	// Get satellite ip from kademlia
	satellite, err := as.kad.FindNode(ctx, satID)
	if err != nil {
		as.log.Warn("Agreementsender could not find satellite", zap.Error(err))
		return
	}
	// Create client from satellite ip
	conn, err := as.transport.DialNode(ctx, &satellite)
	if err != nil {
		as.log.Warn("Agreementsender could not dial satellite", zap.Error(err))
		return
	}
	client := pb.NewBandwidthClient(conn)
	defer func() {
		if err := conn.Close(); err != nil {
			as.log.Warn("Agreementsender failed to close connection", zap.Error(err))
		}
	}()

	//todo:  stop sending these one-by-one, send all at once
	for _, agreement := range agreements {
		rba := agreement.Agreement
		// Send agreement to satellite
		sat, err := client.BandwidthAgreements(ctx, &rba)
		if err != nil {
			switch sat.GetStatus() {
			case pb.AgreementsSummary_FAIL:
				// CASE FAILED: connection with sat couldnt be established or connection
				// is established but lost before receiving response from satellite
				// no updates to the bwa status is done, so it remains "UNSENT"
				as.log.Warn("Agreementsender lost connection", zap.Error(err))
			default:
				// CASE REJECTED: successful connection with sat established but either failed or rejected received
				as.log.Warn("Agreementsender had agreement explicitly rejected/failed by satellite")
				err = as.DB.UpdateBandwidthAllocationStatus(rba.PayerAllocation.SerialNumber, psdb.BwaStatusREJECT)
				if err != nil {
					as.log.Error("Agreementsender error", zap.Error(err))
				}
			}
		} else {
			// updates the status to "SENT"
			err = as.DB.UpdateBandwidthAllocationStatus(rba.PayerAllocation.SerialNumber, psdb.BwaStatusSENT)
			if err != nil {
				as.log.Error("Agreementsender error", zap.Error(err))
			}
		}
	}
}
