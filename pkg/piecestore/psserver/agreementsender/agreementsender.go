// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package agreementsender

import (
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/net/context"

	"storj.io/storj/internal/sync2"
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
	DB        *psdb.DB
	log       *zap.Logger
	transport transport.Client
	kad       *kademlia.Kademlia
	Loop      sync2.Cycle
}

// TODO: take transport instead of identity as argument

// New creates an Agreement Sender
func New(log *zap.Logger, DB *psdb.DB, tc transport.Client, kad *kademlia.Kademlia, checkInterval time.Duration) *AgreementSender {
	return &AgreementSender{DB: DB, log: log, transport: tc, kad: kad, Loop: *sync2.NewCycle(checkInterval)}
}

// Close halts the agreement sender loop
func (as *AgreementSender) Close() error {
	as.Loop.Close()
	return nil
}

// Run the agreement sender with a context to check for cancel
func (as *AgreementSender) Run(ctx context.Context) error {
	as.log.Debug("AgreementSender is running")

	return as.Loop.Run(ctx, func(ctx context.Context) error {
		agreementGroups, err := as.DB.GetBandwidthAllocations()
		if err != nil {
			as.log.Error("Agreementsender could not retrieve bandwidth allocations", zap.Error(err))
			return nil
		}
		for satellite, agreements := range agreementGroups {
			as.SendAgreementsToSatellite(ctx, satellite, agreements)
		}
		return err
	})
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
		err := conn.Close()
		if err != nil {
			as.log.Warn("Agreementsender failed to close connection", zap.Error(err))
		}
	}()

	//todo:  stop sending these one-by-one, send all at once
	for _, agreement := range agreements {
		rba := agreement.Agreement
		if err != nil {
			as.log.Warn("Agreementsender failed to deserialize agreement : will delete", zap.Error(err))
		} else {
			// Send agreement to satellite
			r, err := client.BandwidthAgreements(ctx, &rba)
			if err != nil || r.GetStatus() == pb.AgreementsSummary_FAIL {
				as.log.Warn("Agreementsender failed to send agreement to satellite : will retry", zap.Error(err))
				continue
			} else if r.GetStatus() == pb.AgreementsSummary_REJECTED {
				//todo: something better than a delete here?
				as.log.Error("Agreementsender had agreement explicitly rejected by satellite : will delete", zap.Error(err))
			}
		}
		// Delete from PSDB by signature
		if err = as.DB.DeleteBandwidthAllocationBySerialnum(rba.PayerAllocation.SerialNumber); err != nil {
			as.log.Error("Agreementsender failed to delete bandwidth allocation", zap.Error(err))
		}
	}
}
