// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"testing"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/teststorj"
)

func TestAntechamberAddNode(t *testing.T){
	ctx := testcontext.New(t)
	defer ctx.Cleanup()
	rt := createRoutingTable(ctx, teststorj.NodeIDFromString("OO"))
	defer ctx.Check(rt.Close)
	// 1. not in neighborhood
	// 2. in neighborhood
}

func TestAntechamberRemoveNode(t *testing.T){
	ctx := testcontext.New(t)
	defer ctx.Cleanup()
	rt := createRoutingTable(ctx, teststorj.NodeIDFromString("OO"))
	defer ctx.Check(rt.Close)
	// remove node, check if gone
}

func TestAntechamberFindNear(t *testing.T){
	ctx := testcontext.New(t)
	defer ctx.Cleanup()
	rt := createRoutingTable(ctx, teststorj.NodeIDFromString("OO"))
	defer ctx.Check(rt.Close)
	// add 5 nodes
	// select 3 closest
}

func TestNodeHasValidVoucher(t *testing.T){
	// no vouchers
	// no vouchers have a matching satellite id
	// node id doesn't match
	// voucher is expired
	// signature is unverified
	// one voucher doesn't match and another does
}