// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"fmt"
	pb "github.com/golang/protobuf/proto"

	proto "storj.io/storj/protos/overlay"
	"storj.io/storj/storage"
)

func (rt *RoutingTable) updateReplacementCache(kadBucketID storage.Key, nodes *proto.NodeSlice) error {
	n, err := pb.Marshal(nodes)
	if err != nil {
		return RoutingErr.New("could not marshal node slice %s", err)
	}
	err = rt.replacementCache.Put(kadBucketID, n)
	if err != nil {
		return RoutingErr.New("could not add node slice %s", err)
	}
	return nil
}

func (rt *RoutingTable) getReplacementCacheBucket(kadBucketID storage.Key) (*proto.NodeSlice, error){
	nodes := &proto.NodeSlice{}
	fmt.Print("IM IN GET REPLACEMENT CACHE")
	val, err := rt.replacementCache.Get(kadBucketID)
	if err != nil {
		return nodes, RoutingErr.New("could not get node slice %s", err)
	}
	if val != nil {
		err = pb.Unmarshal(val, nodes)
		if err != nil {
			return nodes, RoutingErr.New("could not unmarshal node slice %s", err)
		}
	}
	return nodes, nil
}

func (rt *RoutingTable) addToReplacementCache(kadBucketID storage.Key, node *proto.Node) error {
	nodes, err := rt.getReplacementCacheBucket(kadBucketID)
	if err != nil {
		return err
	}
	nodes.Nodes = append(nodes.Nodes, node)
	return rt.updateReplacementCache(kadBucketID, nodes)
}