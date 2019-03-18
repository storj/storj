// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package agreementsender

import (
	"io"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"golang.org/x/sync/errgroup"

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
		as.log.Debug("is running", zap.Duration("duration", as.checkInterval))
		agreementGroups, err := as.DB.GetBandwidthAllocations()
		if err != nil {
			as.log.Error("could not retrieve bandwidth allocations", zap.Error(err))
			continue
		}
		// send agreement payouts
		for satellite, agreements := range agreementGroups {
			as.SettleAgreements(ctx, satellite, agreements)
		}

		// Delete older payout irrespective of its status
		if err = as.DB.DeleteBandwidthAllocationPayouts(); err != nil {
			as.log.Error("failed to delete bandwidth allocation", zap.Error(err))
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// SettleAgreements uploads agreements to the satellite
func (as *AgreementSender) SettleAgreements(ctx context.Context, satelliteID storj.NodeID, agreements []*psdb.Agreement) {
	as.log.Info("sending agreements to satellite", zap.Int("number of agreements", len(agreements)), zap.String("satellite id", satelliteID.String()))

	satellite, err := as.kad.FindNode(ctx, satelliteID)
	if err != nil {
		as.log.Warn("could not find satellite", zap.Error(err))
		return
	}

	conn, err := as.transport.DialNode(ctx, &satellite)
	if err != nil {
		as.log.Warn("could not dial satellite", zap.Error(err))
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			as.log.Warn("failed to close connection", zap.Error(err))
		}
	}()

	client, err := pb.NewBandwidthClient(conn).Settlement(ctx)
	if err != nil {
		as.log.Error("failed to start settlement", zap.Error(err))
		return
	}

	var group errgroup.Group
	group.Go(func() error {
		for _, agreement := range agreements {
			err := client.Send(&pb.BandwidthSettlementRequest{
				Allocation: &agreement.Agreement,
			})
			if err != nil {
				return err
			}
		}
		return client.CloseSend()
	})

	for {
		response, err := client.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			as.log.Error("failed to recv response", zap.Error(err))
			break
		}

		switch response.Status {
		case pb.AgreementsSummary_REJECTED:
			err = as.DB.UpdateBandwidthAllocationStatus(response.SerialNumber, psdb.AgreementStatusReject)
			if err != nil {
				as.log.Error("error", zap.Error(err))
			}
		case pb.AgreementsSummary_OK:
			err = as.DB.UpdateBandwidthAllocationStatus(response.SerialNumber, psdb.AgreementStatusSent)
			if err != nil {
				as.log.Error("error", zap.Error(err))
			}
		default:
			as.log.Error("unexpected response", zap.Error(err))
		}
	}

	if err := group.Wait(); err != nil {
		as.log.Error("sending agreements returned an error", zap.Error(err))
	}
}
