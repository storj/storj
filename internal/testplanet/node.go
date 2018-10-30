// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"context"
	"net"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/pkg/utils"
)

type Node struct {
	Info      pb.Node
	Identity  *provider.FullIdentity
	Transport *transport.Transport
	Listener  net.Listener
	Provider  *provider.Provider
	Kademlia  *kademlia.Kademlia
	Overlay   *overlay.Cache

	Dependencies []interface{}
}

func (node *Node) ID() string   { return node.Info.Id }
func (node *Node) Addr() string { return node.Info.Address.Address }

func (node *Node) Shutdown() error {
	var errs []error
	if node.Provider != nil {
		errs = append(errs, node.Provider.Close())
	}
	// Provider automatically closes listener
	// if node.Listener != nil {
	//    errs = append(errs, node.Listener.Close())
	// }
	if node.Kademlia != nil {
		errs = append(errs, node.Kademlia.Disconnect())
	}
	for _, dep := range node.Dependencies {
		if closer, ok := dep.(interface{ Close() error }); ok {
			errs = append(errs, closer.Close())
		} else if closer, ok := dep.(interface{ Close(context.Context) error }); ok {
			errs = append(errs, closer.Close(context.Background()))
		} else if stopper, ok := dep.(interface{ Stop() error }); ok {
			errs = append(errs, stopper.Stop())
		} else if stopper, ok := dep.(interface{ Stop(context.Context) error }); ok {
			errs = append(errs, stopper.Stop(context.Background()))
		} else if disconnect, ok := dep.(interface{ Disconnect() error }); ok {
			errs = append(errs, disconnect.Disconnect())
		} else if disconnect, ok := dep.(interface{ Disconnect(context.Context) error }); ok {
			errs = append(errs, disconnect.Disconnect(context.Background()))
		}
	}
	return utils.CombineErrors(errs...)
}

func (node *Node) DialPointerDB(destination *Node, apikey string) (pdbclient.Client, error) {
	/*
		// TODO: use node.Transport
			conn, err := node.Transport.DialNode(context.Background(), &destination.Info)
			if err != nil {
				return nil, err
			}
			return piececlient.NewPSClient
	*/
	return pdbclient.NewClient(node.Identity, destination.Addr(), apikey)
}
