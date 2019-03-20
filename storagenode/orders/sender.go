// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"context"
	"io"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
)

// Info contains full information about an order.
type Info struct {
	Limit  *pb.OrderLimit2
	Order  *pb.Order2
	Uplink *identity.PeerIdentity
}

type OrderStatus byte

const (
	StatusInvalid OrderStatus = iota
	StatusAccepted
	StatusRejected
)

// DB implements storing orders for sending to the satellite.
type DB interface {
	// Enqueue inserts order to the list of orders needing to be sent to the satellite.
	Enqueue(ctx context.Context, info *Info) error
	// ListUnsent returns orders that haven't been sent yet.
	ListUnsent(ctx context.Context, limit int) ([]*Info, error)
	// ListUnsentBySatellite returns orders that haven't been sent yet grouped by satellite.
	ListUnsentBySatellite(ctx context.Context) (map[storj.NodeID][]*Info, error)
	// Archive marks order as being handled.
	Archive(ctx context.Context, satellite storj.NodeID, serial storj.SerialNumber, status OrderStatus) error
}

// SenderConfig defines configuration for sending orders.
type SenderConfig struct {
	Interval      time.Duration
	SettleTimeout time.Duration
}

// Sender sends every interval unsent orders to the satellite.
type Sender struct {
	log    *zap.Logger
	config SenderConfig

	transport transport.Client
	kademlia  *kademlia.Kademlia
	orders    DB

	Loop sync2.Cycle
}

// NewSender creates an order sender.
func NewSender(log *zap.Logger, transport transport.Client, kademlia *kademlia.Kademlia, orders DB, config SenderConfig) *Sender {
	return &Sender{
		log:       log,
		transport: transport,
		kademlia:  kademlia,
		orders:    orders,

		Loop: *sync2.NewCycle(config.Interval),
	}
}

// Run sends orders on every interval to the appropriate satellites.
func (sender *Sender) Run(ctx context.Context) error {
	return sender.Loop.Run(ctx, func(ctx context.Context) error {
		sender.log.Debug("sending")

		ordersBySatellite, err := sender.orders.ListUnsentBySatellite(ctx)
		if err != nil {
			sender.log.Error("listing orders", zap.Error(err))
			return nil
		}

		if len(ordersBySatellite) > 0 {
			var group errgroup.Group
			ctx, cancel := context.WithTimeout(ctx, sender.config.SettleTimeout)
			defer cancel()

			for satelliteID, orders := range ordersBySatellite {
				satelliteID, orders := satelliteID, orders
				group.Go(func() error {
					sender.Settle(ctx, satelliteID, orders)
					return nil
				})
			}
		} else {
			sender.log.Debug("no orders to send")
		}

		return nil
	})
}

// Settle uploads agreements to the satellite.
func (sender *Sender) Settle(ctx context.Context, satelliteID storj.NodeID, orders []*Info) {
	log := sender.log.Named(satelliteID.String())

	log.Info("sending", zap.Int("count", len(orders)))
	defer log.Info("finished")

	satellite, err := sender.kademlia.FindNode(ctx, satelliteID)
	if err != nil {
		log.Error("unable to find satellite on the network", zap.Error(err))
		return
	}

	conn, err := sender.transport.DialNode(ctx, &satellite)
	if err != nil {
		log.Error("unable to connect to the satellite", zap.Error(err))
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Warn("failed to close connection", zap.Error(err))
		}
	}()

	client, err := pb.NewOrdersClient(conn).Settlement(ctx)
	if err != nil {
		log.Error("failed to start settlement", zap.Error(err))
		return
	}

	var group errgroup.Group
	group.Go(func() error {
		for _, order := range orders {
			err := client.Send(&pb.SettlementRequest{
				Limit: order.Limit,
				Order: order.Order,
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

			log.Error("failed to receive response", zap.Error(err))
			break
		}

		switch response.Status {
		case pb.SettlementResponse_ACCEPTED:
			err = sender.orders.Archive(ctx, satelliteID, response.SerialNumber, StatusAccepted)
			if err != nil {
				log.Error("failed to archive order as accepted", zap.Stringer("serial", response.SerialNumber), zap.Error(err))
			}
		case pb.SettlementResponse_REJECTED:
			err = sender.orders.Archive(ctx, satelliteID, response.SerialNumber, StatusRejected)
			if err != nil {
				log.Error("failed to archive order as rejected", zap.Stringer("serial", response.SerialNumber), zap.Error(err))
			}
		default:
			log.Error("unexpected response", zap.Error(err))
		}
	}

	if err := group.Wait(); err != nil {
		log.Error("sending agreements returned an error", zap.Error(err))
	}
}

// Close stops the sending service.
func (sender *Sender) Close() error {
	sender.Loop.Stop()
	return nil
}
