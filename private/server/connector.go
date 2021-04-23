// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"
	"crypto/tls"

	"github.com/zeebo/errs"

	"storj.io/common/memory"
	"storj.io/common/rpc"
	"storj.io/storj/private/quic"
)

// HybridConnector implements a dialer that creates a connection using either
// quic or tcp.
type HybridConnector struct {
	quic *quic.Connector
	tcp  *rpc.TCPConnector
}

// NewDefaultHybridConnector instantiates a new instance of HybridConnector with
// provided quic and tcp connectors.
// If a nil value is provided for either connector, a default connector will be
// created instead.
// See func DialContext for more details.
func NewDefaultHybridConnector(qc *quic.Connector, tc *rpc.TCPConnector) HybridConnector {
	if qc == nil {
		connector := quic.NewDefaultConnector(nil)
		qc = &connector
	}
	if tc == nil {
		connector := rpc.NewDefaultTCPConnector(nil)
		tc = &connector
	}

	return HybridConnector{
		quic: qc,
		tcp:  tc,
	}
}

// DialContext creates a connection using either quic or tcp.
// It tries to dial through both connector and returns the first established
// connection. If both connections are established, it will return quic connection.
// An error is returned if both connector failed.
func (c HybridConnector) DialContext(ctx context.Context, tlsConfig *tls.Config, address string) (_ rpc.ConnectorConn, err error) {
	defer mon.Task()(&ctx)(&err)

	if tlsConfig == nil {
		return nil, Error.New("tls config is not set")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var tcpConn, quicConn rpc.ConnectorConn
	errChan := make(chan error)
	readyChan := make(chan struct{})

	go func() {
		var err error
		quicConn, err = c.quic.DialContext(ctx, tlsConfig.Clone(), address)
		if err != nil {
			errChan <- err
			return
		}

		readyChan <- struct{}{}
	}()
	go func() {
		var err error
		tcpConn, err = c.tcp.DialContext(ctx, tlsConfig.Clone(), address)
		if err != nil {
			errChan <- err
			return
		}

		readyChan <- struct{}{}
	}()

	var errors []error
	var numFinished int
	// makre sure both dial is finished either with an established connection or
	// an error. It allows us to appropriately close tcp connection if both
	// connections are ready around the same time
	for numFinished < 2 {
		select {
		case <-readyChan:
			numFinished++
			// if one connection is ready, we want to cancel the other dial if
			// the connection isn't ready
			cancel()
		case err := <-errChan:
			numFinished++
			errors = append(errors, err)
		}
	}

	// we want to prioritize quic conn if both connections are available
	if quicConn != nil {
		if tcpConn != nil {
			_ = tcpConn.Close()
		}

		mon.Event("hybrid_connector_established_quic_connection")
		return quicConn, nil
	}

	if tcpConn != nil {
		mon.Event("hybrid_connector_established_tcp_connection")
		return tcpConn, nil
	}

	mon.Event("hybrid_connector_established_no_connection")

	return nil, errs.Combine(errors...)
}

// SetQUICTransferRate returns a connector with the given transfer rate.
func (c *HybridConnector) SetQUICTransferRate(rate memory.Size) {
	updated := c.quic.SetTransferRate(rate)
	c.quic = &updated
}

// SetTCPTransferRate returns a connector with the given transfer rate.
func (c *HybridConnector) SetTCPTransferRate(rate memory.Size) {
	c.tcp.TransferRate = rate
}
