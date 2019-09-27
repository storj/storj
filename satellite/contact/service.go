// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"
	"sync"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/rpc"
	"storj.io/storj/satellite/overlay"
)

// Error is the default error class for contact package.
var Error = errs.Class("contact")

var mon = monkit.Package()

// Config contains configurable values for contact service
type Config struct {
	ExternalAddress string `user:"true" help:"the public address of the node, useful for nodes behind NAT" default:""`
}

// Conn represents a connection
type Conn struct {
	conn   *rpc.Conn
	client rpc.NodesClient
}

// Service is the contact service between storage nodes and satellites.
// It is responsible for updating general node information like address, capacity, and uptime.
// It is also responsible for updating peer identity information for verifying signatures from that node.
//
// architecture: Service
type Service struct {
	log *zap.Logger

	mutex sync.Mutex
	self  *overlay.NodeDossier

	overlay *overlay.Service
	peerIDs overlay.PeerIdentities
	dialer  rpc.Dialer
}

// NewService creates a new contact service.
func NewService(log *zap.Logger, self *overlay.NodeDossier, overlay *overlay.Service, peerIDs overlay.PeerIdentities, dialer rpc.Dialer) *Service {
	return &Service{
		log:     log,
		self:    self,
		overlay: overlay,
		peerIDs: peerIDs,
		dialer:  dialer,
	}
}

// Local returns the satellite node dossier
func (service *Service) Local() overlay.NodeDossier {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	return *service.self
}

// FetchInfo connects to a node and returns its node info.
func (service *Service) FetchInfo(ctx context.Context, target pb.Node) (_ *pb.InfoResponse, err error) {
	conn, err := service.dialNode(ctx, target)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, conn.Close()) }()

	resp, err := conn.client.RequestInfo(ctx, &pb.InfoRequest{})
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// dialNode dials the specified node.
func (service *Service) dialNode(ctx context.Context, target pb.Node) (_ *Conn, err error) {
	defer mon.Task()(&ctx)(&err)

	conn, err := service.dialer.DialNode(ctx, &target)
	if err != nil {
		return nil, err
	}

	return &Conn{
		conn:   conn,
		client: conn.NodesClient(),
	}, err
}

// Close disconnects this connection.
func (conn *Conn) Close() error {
	return conn.conn.Close()
}

// Close closes resources
func (service *Service) Close() error { return nil }
