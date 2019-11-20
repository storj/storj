// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build grpc

package rpc

import (
	"google.golang.org/grpc"

	"storj.io/storj/pkg/pb"
)

// RawConn is a type alias to a grpc client connection
type RawConn = grpc.ClientConn

type (
	// CertificatesClient is an alias to the grpc client interface
	CertificatesClient = pb.CertificatesClient

	// ContactClient is an alias to the grpc client interface
	ContactClient = pb.ContactClient

	// HealthInspectorClient is an alias to the grpc client interface
	HealthInspectorClient = pb.HealthInspectorClient

	// IrreparableInspectorClient is an alias to the grpc client interface
	IrreparableInspectorClient = pb.IrreparableInspectorClient

	// MetainfoClient is an alias to the grpc client interface
	MetainfoClient = pb.MetainfoClient

	// NodeClient is an alias to the grpc client interface
	NodeClient = pb.NodeClient

	// NodeGracefulExitClient is an alias to the grpc client interface
	NodeGracefulExitClient = pb.NodeGracefulExitClient

	// NodeStatsClient is an alias to the grpc client interface
	NodeStatsClient = pb.NodeStatsClient

	// OrdersClient is an alias to the grpc client interface
	OrdersClient = pb.OrdersClient

	// OverlayInspectorClient is an alias to the grpc client interface
	OverlayInspectorClient = pb.OverlayInspectorClient

	// PaymentsClient is an alias to the grpc client interface
	PaymentsClient = pb.PaymentsClient

	// PieceStoreInspectorClient is an alias to the grpc client interface
	PieceStoreInspectorClient = pb.PieceStoreInspectorClient

	// PiecestoreClient is an alias to the grpc client interface
	PiecestoreClient = pb.PiecestoreClient

	// ReferralManagerClient is an alias to the grpc client interface
	ReferralManagerClient = pb.ReferralManagerClient

	// SatelliteGracefulExitClient is an alias to the grpc client interface
	SatelliteGracefulExitClient = pb.SatelliteGracefulExitClient

	// VouchersClient is an alias to the grpc client interface
	VouchersClient = pb.VouchersClient
)

// NewCertificatesClient returns the grpc version of a CertificatesClient
func NewCertificatesClient(rc *RawConn) CertificatesClient {
	return pb.NewCertificatesClient(rc)
}

// CertificatesClient returns a CertificatesClient for this connection
func (c *Conn) CertificatesClient() CertificatesClient {
	return NewCertificatesClient(c.raw)
}

// NewContactClient returns the grpc version of a ContactClient
func NewContactClient(rc *RawConn) ContactClient {
	return pb.NewContactClient(rc)
}

// ContactClient returns a ContactClient for this connection
func (c *Conn) ContactClient() ContactClient {
	return NewContactClient(c.raw)
}

// NewHealthInspectorClient returns the grpc version of a HealthInspectorClient
func NewHealthInspectorClient(rc *RawConn) HealthInspectorClient {
	return pb.NewHealthInspectorClient(rc)
}

// HealthInspectorClient returns a HealthInspectorClient for this connection
func (c *Conn) HealthInspectorClient() HealthInspectorClient {
	return NewHealthInspectorClient(c.raw)
}

// NewIrreparableInspectorClient returns the grpc version of a IrreparableInspectorClient
func NewIrreparableInspectorClient(rc *RawConn) IrreparableInspectorClient {
	return pb.NewIrreparableInspectorClient(rc)
}

// IrreparableInspectorClient returns a IrreparableInspectorClient for this connection
func (c *Conn) IrreparableInspectorClient() IrreparableInspectorClient {
	return NewIrreparableInspectorClient(c.raw)
}

// NewMetainfoClient returns the grpc version of a MetainfoClient
func NewMetainfoClient(rc *RawConn) MetainfoClient {
	return pb.NewMetainfoClient(rc)
}

// MetainfoClient returns a MetainfoClient for this connection
func (c *Conn) MetainfoClient() MetainfoClient {
	return NewMetainfoClient(c.raw)
}

// NewNodeClient returns the grpc version of a NodeClient
func NewNodeClient(rc *RawConn) NodeClient {
	return pb.NewNodeClient(rc)
}

// NodeClient returns a NodeClient for this connection
func (c *Conn) NodeClient() NodeClient {
	return NewNodeClient(c.raw)
}

// NewNodeGracefulExitClient returns the grpc version of a NodeGracefulExitClient
func NewNodeGracefulExitClient(rc *RawConn) NodeGracefulExitClient {
	return pb.NewNodeGracefulExitClient(rc)
}

// NodeGracefulExitClient returns a NodeGracefulExitClient for this connection
func (c *Conn) NodeGracefulExitClient() NodeGracefulExitClient {
	return NewNodeGracefulExitClient(c.raw)
}

// NewNodeStatsClient returns the grpc version of a NodeStatsClient
func NewNodeStatsClient(rc *RawConn) NodeStatsClient {
	return pb.NewNodeStatsClient(rc)
}

// NodeStatsClient returns a NodeStatsClient for this connection
func (c *Conn) NodeStatsClient() NodeStatsClient {
	return NewNodeStatsClient(c.raw)
}

// NewOrdersClient returns the grpc version of a OrdersClient
func NewOrdersClient(rc *RawConn) OrdersClient {
	return pb.NewOrdersClient(rc)
}

// OrdersClient returns a OrdersClient for this connection
func (c *Conn) OrdersClient() OrdersClient {
	return NewOrdersClient(c.raw)
}

// NewOverlayInspectorClient returns the grpc version of a OverlayInspectorClient
func NewOverlayInspectorClient(rc *RawConn) OverlayInspectorClient {
	return pb.NewOverlayInspectorClient(rc)
}

// OverlayInspectorClient returns a OverlayInspectorClient for this connection
func (c *Conn) OverlayInspectorClient() OverlayInspectorClient {
	return NewOverlayInspectorClient(c.raw)
}

// NewPaymentsClient returns the grpc version of a PaymentsClient
func NewPaymentsClient(rc *RawConn) PaymentsClient {
	return pb.NewPaymentsClient(rc)
}

// PaymentsClient returns a PaymentsClient for this connection
func (c *Conn) PaymentsClient() PaymentsClient {
	return NewPaymentsClient(c.raw)
}

// NewPieceStoreInspectorClient returns the grpc version of a PieceStoreInspectorClient
func NewPieceStoreInspectorClient(rc *RawConn) PieceStoreInspectorClient {
	return pb.NewPieceStoreInspectorClient(rc)
}

// PieceStoreInspectorClient returns a PieceStoreInspectorClient for this connection
func (c *Conn) PieceStoreInspectorClient() PieceStoreInspectorClient {
	return NewPieceStoreInspectorClient(c.raw)
}

// NewPiecestoreClient returns the grpc version of a PiecestoreClient
func NewPiecestoreClient(rc *RawConn) PiecestoreClient {
	return pb.NewPiecestoreClient(rc)
}

// PiecestoreClient returns a PiecestoreClient for this connection
func (c *Conn) PiecestoreClient() PiecestoreClient {
	return NewPiecestoreClient(c.raw)
}

// NewReferralManagerClient returns the grpc version of a ReferralManagerClient
func NewReferralManagerClient(rc *RawConn) ReferralManagerClient {
	return pb.NewReferralManagerClient(rc)
}

// ReferralManagerClient returns a ReferralManagerClient for this connection
func (c *Conn) ReferralManagerClient() ReferralManagerClient {
	return NewReferralManagerClient(c.raw)
}

// NewSatelliteGracefulExitClient returns the grpc version of a SatelliteGracefulExitClient
func NewSatelliteGracefulExitClient(rc *RawConn) SatelliteGracefulExitClient {
	return pb.NewSatelliteGracefulExitClient(rc)
}

// SatelliteGracefulExitClient returns a SatelliteGracefulExitClient for this connection
func (c *Conn) SatelliteGracefulExitClient() SatelliteGracefulExitClient {
	return NewSatelliteGracefulExitClient(c.raw)
}

// NewVouchersClient returns the grpc version of a VouchersClient
func NewVouchersClient(rc *RawConn) VouchersClient {
	return pb.NewVouchersClient(rc)
}

// VouchersClient returns a VouchersClient for this connection
func (c *Conn) VouchersClient() VouchersClient {
	return NewVouchersClient(c.raw)
}
