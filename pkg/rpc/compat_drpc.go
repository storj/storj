// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build drpc

package rpc

import (
	"storj.io/drpc/drpcconn"
	"storj.io/storj/pkg/pb"
)

// RawConn is a type alias to a drpc client connection
type RawConn = drpcconn.Conn

type (
	// CertificatesClient is an alias to the drpc client interface
	CertificatesClient = pb.DRPCCertificatesClient

	// ContactClient is an alias to the drpc client interface
	ContactClient = pb.DRPCContactClient

	// HealthInspectorClient is an alias to the drpc client interface
	HealthInspectorClient = pb.DRPCHealthInspectorClient

	// IrreparableInspectorClient is an alias to the drpc client interface
	IrreparableInspectorClient = pb.DRPCIrreparableInspectorClient

	// MetainfoClient is an alias to the drpc client interface
	MetainfoClient = pb.DRPCMetainfoClient

	// NodeClient is an alias to the drpc client interface
	NodeClient = pb.DRPCNodeClient

	// NodeGracefulExitClient is an alias to the drpc client interface
	NodeGracefulExitClient = pb.DRPCNodeGracefulExitClient

	// NodeStatsClient is an alias to the drpc client interface
	NodeStatsClient = pb.DRPCNodeStatsClient

	// OrdersClient is an alias to the drpc client interface
	OrdersClient = pb.DRPCOrdersClient

	// OverlayInspectorClient is an alias to the drpc client interface
	OverlayInspectorClient = pb.DRPCOverlayInspectorClient

	// PieceStoreInspectorClient is an alias to the drpc client interface
	PieceStoreInspectorClient = pb.DRPCPieceStoreInspectorClient

	// PiecestoreClient is an alias to the drpc client interface
	PiecestoreClient = pb.DRPCPiecestoreClient

	// SatelliteGracefulExitClient is an alias to the drpc client interface
	SatelliteGracefulExitClient = pb.DRPCSatelliteGracefulExitClient

	// VouchersClient is an alias to the drpc client interface
	VouchersClient = pb.DRPCVouchersClient
)

// NewCertificatesClient returns the drpc version of a CertificatesClient
func NewCertificatesClient(rc *RawConn) CertificatesClient {
	return pb.NewDRPCCertificatesClient(rc)
}

// CertificatesClient returns a CertificatesClient for this connection
func (c *Conn) CertificatesClient() CertificatesClient {
	return NewCertificatesClient(c.raw)
}

// NewContactClient returns the drpc version of a ContactClient
func NewContactClient(rc *RawConn) ContactClient {
	return pb.NewDRPCContactClient(rc)
}

// ContactClient returns a ContactClient for this connection
func (c *Conn) ContactClient() ContactClient {
	return NewContactClient(c.raw)
}

// NewHealthInspectorClient returns the drpc version of a HealthInspectorClient
func NewHealthInspectorClient(rc *RawConn) HealthInspectorClient {
	return pb.NewDRPCHealthInspectorClient(rc)
}

// HealthInspectorClient returns a HealthInspectorClient for this connection
func (c *Conn) HealthInspectorClient() HealthInspectorClient {
	return NewHealthInspectorClient(c.raw)
}

// NewIrreparableInspectorClient returns the drpc version of a IrreparableInspectorClient
func NewIrreparableInspectorClient(rc *RawConn) IrreparableInspectorClient {
	return pb.NewDRPCIrreparableInspectorClient(rc)
}

// IrreparableInspectorClient returns a IrreparableInspectorClient for this connection
func (c *Conn) IrreparableInspectorClient() IrreparableInspectorClient {
	return NewIrreparableInspectorClient(c.raw)
}

// NewMetainfoClient returns the drpc version of a MetainfoClient
func NewMetainfoClient(rc *RawConn) MetainfoClient {
	return pb.NewDRPCMetainfoClient(rc)
}

// MetainfoClient returns a MetainfoClient for this connection
func (c *Conn) MetainfoClient() MetainfoClient {
	return NewMetainfoClient(c.raw)
}

// NewNodeClient returns the drpc version of a NodeClient
func NewNodeClient(rc *RawConn) NodeClient {
	return pb.NewDRPCNodeClient(rc)
}

// NodeClient returns a NodeClient for this connection
func (c *Conn) NodeClient() NodeClient {
	return NewNodeClient(c.raw)
}

// NewNodeGracefulExitClient returns the drpc version of a NodeGracefulExitClient
func NewNodeGracefulExitClient(rc *RawConn) NodeGracefulExitClient {
	return pb.NewDRPCNodeGracefulExitClient(rc)
}

// NodeGracefulExitClient returns a NodeGracefulExitClient for this connection
func (c *Conn) NodeGracefulExitClient() NodeGracefulExitClient {
	return NewNodeGracefulExitClient(c.raw)
}

// NewNodeStatsClient returns the drpc version of a NodeStatsClient
func NewNodeStatsClient(rc *RawConn) NodeStatsClient {
	return pb.NewDRPCNodeStatsClient(rc)
}

// NodeStatsClient returns a NodeStatsClient for this connection
func (c *Conn) NodeStatsClient() NodeStatsClient {
	return NewNodeStatsClient(c.raw)
}

// NewOrdersClient returns the drpc version of a OrdersClient
func NewOrdersClient(rc *RawConn) OrdersClient {
	return pb.NewDRPCOrdersClient(rc)
}

// OrdersClient returns a OrdersClient for this connection
func (c *Conn) OrdersClient() OrdersClient {
	return NewOrdersClient(c.raw)
}

// NewOverlayInspectorClient returns the drpc version of a OverlayInspectorClient
func NewOverlayInspectorClient(rc *RawConn) OverlayInspectorClient {
	return pb.NewDRPCOverlayInspectorClient(rc)
}

// OverlayInspectorClient returns a OverlayInspectorClient for this connection
func (c *Conn) OverlayInspectorClient() OverlayInspectorClient {
	return NewOverlayInspectorClient(c.raw)
}

// NewPieceStoreInspectorClient returns the drpc version of a PieceStoreInspectorClient
func NewPieceStoreInspectorClient(rc *RawConn) PieceStoreInspectorClient {
	return pb.NewDRPCPieceStoreInspectorClient(rc)
}

// PieceStoreInspectorClient returns a PieceStoreInspectorClient for this connection
func (c *Conn) PieceStoreInspectorClient() PieceStoreInspectorClient {
	return NewPieceStoreInspectorClient(c.raw)
}

// NewPiecestoreClient returns the drpc version of a PiecestoreClient
func NewPiecestoreClient(rc *RawConn) PiecestoreClient {
	return pb.NewDRPCPiecestoreClient(rc)
}

// PiecestoreClient returns a PiecestoreClient for this connection
func (c *Conn) PiecestoreClient() PiecestoreClient {
	return NewPiecestoreClient(c.raw)
}

// NewSatelliteGracefulExitClient returns the drpc version of a SatelliteGracefulExitClient
func NewSatelliteGracefulExitClient(rc *RawConn) SatelliteGracefulExitClient {
	return pb.NewDRPCSatelliteGracefulExitClient(rc)
}

// SatelliteGracefulExitClient returns a SatelliteGracefulExitClient for this connection
func (c *Conn) SatelliteGracefulExitClient() SatelliteGracefulExitClient {
	return NewSatelliteGracefulExitClient(c.raw)
}

// NewVouchersClient returns the drpc version of a VouchersClient
func NewVouchersClient(rc *RawConn) VouchersClient {
	return pb.NewDRPCVouchersClient(rc)
}

// VouchersClient returns a VouchersClient for this connection
func (c *Conn) VouchersClient() VouchersClient {
	return NewVouchersClient(c.raw)
}
