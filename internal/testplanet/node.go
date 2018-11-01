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
	"storj.io/storj/storage/teststore"
)

// Node is a general purpose
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

// ID returns node id
// TODO: switch to storj.NodeID
func (node *Node) ID() string { return node.Info.Id }

// Addr retursn node address
func (node *Node) Addr() string { return node.Info.Address.Address }

// Shutdown shuts down all node dependencies
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
		err := tryClose(dep)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return utils.CombineErrors(errs...)
}

// DialPointerDB dials destination with apikey and returns pointerdb Client
func (node *Node) DialPointerDB(destination *Node, apikey string) (pdbclient.Client, error) {
	// TODO: use node.Transport instead of pdbclient.NewClient
	/*
		conn, err := node.Transport.DialNode(context.Background(), &destination.Info)
		if err != nil {
			return nil, err
		}
		return piececlient.NewPSClient
	*/
	return pdbclient.NewClient(node.Identity, destination.Addr(), apikey)
}

// initOverlay creates overlay for a given planet
func (node *Node) initOverlay(planet *Planet) error {
	routing, err := kademlia.NewRoutingTable(node.Info, teststore.New(), teststore.New())
	if err != nil {
		return err
	}

	kad, err := kademlia.NewKademliaWithRoutingTable(node.Info, planet.nodeInfos, node.Identity, 5, routing)
	if err != nil {
		return utils.CombineErrors(err, routing.Close())
	}

	node.Kademlia = kad
	node.Overlay = overlay.NewOverlayCache(teststore.New(), node.Kademlia)

	return nil
}

// tryClose tries to guess the closing method and stop/close/shutdown the dependency
func tryClose(dep interface{}) error {
	if closer, ok := dep.(interface{ Close() error }); ok {
		return closer.Close()
	} else if closer, ok := dep.(interface{ Close(context.Context) error }); ok {
		return closer.Close(context.Background())
	} else if stopper, ok := dep.(interface{ Stop() error }); ok {
		return stopper.Stop()
	} else if stopper, ok := dep.(interface{ Stop(context.Context) error }); ok {
		return stopper.Stop(context.Background())
	} else if disconnect, ok := dep.(interface{ Disconnect() error }); ok {
		return disconnect.Disconnect()
	} else if disconnect, ok := dep.(interface{ Disconnect(context.Context) error }); ok {
		return disconnect.Disconnect(context.Background())
	} else if closer, ok := dep.(interface{ Close() }); ok {
		closer.Close()
		return nil
	} else if closer, ok := dep.(interface{ Close(context.Context) }); ok {
		closer.Close(context.Background())
		return nil
	} else if stopper, ok := dep.(interface{ Stop() }); ok {
		stopper.Stop()
		return nil
	} else if stopper, ok := dep.(interface{ Stop(context.Context) }); ok {
		stopper.Stop(context.Background())
		return nil
	} else if disconnect, ok := dep.(interface{ Disconnect() }); ok {
		disconnect.Disconnect()
		return nil
	} else if disconnect, ok := dep.(interface{ Disconnect(context.Context) }); ok {
		disconnect.Disconnect(context.Background())
		return nil
	}
	return nil
}
