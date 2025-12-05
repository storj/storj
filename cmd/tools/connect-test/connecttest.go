// Copyright (C) 2021 Storj, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"os"

	"golang.org/x/sync/errgroup"

	"storj.io/common/identity"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/rpc/quic"
	"storj.io/common/storj"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("needs a node address like <ip-address>:<port-number>, but got none")
	}

	destAddr := os.Args[1]
	ctx := context.Background()

	ident, err := identity.NewFullIdentity(ctx, identity.NewCAOptions{
		Difficulty:  0,
		Concurrency: 1,
	})
	if err != nil {
		log.Fatalf("could not generate an identity: %v", err)
	}
	tlsOptions, err := tlsopts.NewOptions(ident, tlsopts.Config{
		PeerIDVersions: "*",
	}, nil)
	if err != nil {
		log.Fatalf("could not get tls options: %v", err)
	}
	unverifiedClientConfig := tlsOptions.UnverifiedClientTLSConfig()

	var (
		group      errgroup.Group
		quicNodeID storj.NodeID
		quicErr    error
		tcpNodeID  storj.NodeID
		tcpErr     error
	)
	group.Go(func() error {
		quicNodeID, quicErr = tryConnect(ctx, unverifiedClientConfig, quic.NewDefaultConnector(nil), destAddr)
		return nil
	})
	group.Go(func() error {
		//lint:ignore SA1019 deprecated is fine here.
		//nolint:staticcheck // deprecated is fine here.
		connector := rpc.NewDefaultTCPConnector(nil)

		tcpNodeID, tcpErr = tryConnect(ctx, unverifiedClientConfig, connector, destAddr)
		return nil
	})
	err = group.Wait()
	if err != nil {
		log.Fatalf("failed to perform checks: %v", err)
	}

	if quicErr != nil {
		fmt.Printf("QUIC\tfail\t%v\n", quicErr)
	} else {
		fmt.Printf("QUIC\tsuccess\t%s\n", quicNodeID.String())
	}
	if tcpErr != nil {
		fmt.Printf("TCP\tfail\t%v\n", tcpErr)
	} else {
		fmt.Printf("TCP\tsuccess\t%s\n", tcpNodeID.String())
	}
	if quicErr == nil && tcpErr == nil && quicNodeID.Compare(tcpNodeID) != 0 {
		fmt.Printf("(warning: node IDs do not match)\n")
	}
}

func tryConnect(ctx context.Context, tlsConfig *tls.Config, dialer rpc.Connector, destAddr string) (storj.NodeID, error) {
	conn, err := dialer.DialContext(ctx, tlsConfig, destAddr)
	if err != nil {
		return storj.NodeID{}, err
	}
	defer func() { _ = conn.Close() }()
	nodeID, err := identity.PeerIdentityFromChain(conn.ConnectionState().PeerCertificates)
	if err != nil {
		return storj.NodeID{}, fmt.Errorf("could not get node ID from peer certificates: %w", err)
	}
	return nodeID.ID, nil
}
