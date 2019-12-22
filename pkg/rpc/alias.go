// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rpc

import (
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/rpc/rpcpool"
)

// RawConn is a type alias to a drpc client connection
type RawConn = rpcpool.Conn

// CertificatesClient returns a CertificatesClient for this connection
func (c *Conn) CertificatesClient() pb.DRPCCertificatesClient {
	return pb.NewDRPCCertificatesClient(c.raw)
}

// ContactClient returns a ContactClient for this connection
func (c *Conn) ContactClient() pb.DRPCContactClient {
	return pb.NewDRPCContactClient(c.raw)
}

// HealthInspectorClient returns a HealthInspectorClient for this connection
func (c *Conn) HealthInspectorClient() pb.DRPCHealthInspectorClient {
	return pb.NewDRPCHealthInspectorClient(c.raw)
}

// IrreparableInspectorClient returns a IrreparableInspectorClient for this connection
func (c *Conn) IrreparableInspectorClient() pb.DRPCIrreparableInspectorClient {
	return pb.NewDRPCIrreparableInspectorClient(c.raw)
}

// MetainfoClient returns a MetainfoClient for this connection
func (c *Conn) MetainfoClient() pb.DRPCMetainfoClient {
	return pb.NewDRPCMetainfoClient(c.raw)
}

// NodeClient returns a NodeClient for this connection
func (c *Conn) NodeClient() pb.DRPCNodeClient {
	return pb.NewDRPCNodeClient(c.raw)
}

// NodeGracefulExitClient returns a NodeGracefulExitClient for this connection
func (c *Conn) NodeGracefulExitClient() pb.DRPCNodeGracefulExitClient {
	return pb.NewDRPCNodeGracefulExitClient(c.raw)
}

// NodeStatsClient returns a NodeStatsClient for this connection
func (c *Conn) NodeStatsClient() pb.DRPCNodeStatsClient {
	return pb.NewDRPCNodeStatsClient(c.raw)
}

// OrdersClient returns a OrdersClient for this connection
func (c *Conn) OrdersClient() pb.DRPCOrdersClient {
	return pb.NewDRPCOrdersClient(c.raw)
}

// OverlayInspectorClient returns a OverlayInspectorClient for this connection
func (c *Conn) OverlayInspectorClient() pb.DRPCOverlayInspectorClient {
	return pb.NewDRPCOverlayInspectorClient(c.raw)
}

// PaymentsClient returns a PaymentsClient for this connection
func (c *Conn) PaymentsClient() pb.DRPCPaymentsClient {
	return pb.NewDRPCPaymentsClient(c.raw)
}

// PieceStoreInspectorClient returns a PieceStoreInspectorClient for this connection
func (c *Conn) PieceStoreInspectorClient() pb.DRPCPieceStoreInspectorClient {
	return pb.NewDRPCPieceStoreInspectorClient(c.raw)
}

// PiecestoreClient returns a PiecestoreClient for this connection
func (c *Conn) PiecestoreClient() pb.DRPCPiecestoreClient {
	return pb.NewDRPCPiecestoreClient(c.raw)
}

// ReferralManagerClient returns a ReferralManagerClient for this connection
func (c *Conn) ReferralManagerClient() pb.DRPCReferralManagerClient {
	return pb.NewDRPCReferralManagerClient(c.raw)
}

// SatelliteGracefulExitClient returns a SatelliteGracefulExitClient for this connection
func (c *Conn) SatelliteGracefulExitClient() pb.DRPCSatelliteGracefulExitClient {
	return pb.NewDRPCSatelliteGracefulExitClient(c.raw)
}

// VouchersClient returns a VouchersClient for this connection
func (c *Conn) VouchersClient() pb.DRPCVouchersClient {
	return pb.NewDRPCVouchersClient(c.raw)
}
