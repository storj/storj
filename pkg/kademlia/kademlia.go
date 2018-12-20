// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"unsafe"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
	"storj.io/storj/storage/boltdb"
)

var (
	// NodeErr is the class for all errors pertaining to node operations
	NodeErr = errs.Class("node error")
	// BootstrapErr is the class for all errors pertaining to bootstrapping a node
	BootstrapErr = errs.Class("bootstrap node error")
	// NodeNotFound is returned when a lookup can not produce the requested node
	NodeNotFound = errs.Class("node not found")
	// TODO: shouldn't default to TCP but not sure what to do yet
	defaultTransport = pb.NodeTransport_TCP_TLS_GRPC
	defaultRetries   = 3
)

type discoveryOptions struct {
	concurrency    int
	retries        int
	bootstrap      bool
	bootstrapNodes []pb.Node
}

// Kademlia is an implementation of kademlia adhering to the DHT interface.
type Kademlia struct {
	log             *zap.Logger
	alpha           int // alpha is a system wide concurrency parameter
	routingTable    *RoutingTable
	bootstrapNodes  []pb.Node
	nodeClient      node.Client
	identity        *provider.FullIdentity
	bootstrapCancel unsafe.Pointer // context.CancelFunc
}

// NewKademlia returns a newly configured Kademlia instance
func NewKademlia(log *zap.Logger, id storj.NodeID, nodeType pb.NodeType, bootstrapNodes []pb.Node, address string, metadata *pb.NodeMetadata, identity *provider.FullIdentity, path string, alpha int) (*Kademlia, error) {
	self := pb.Node{
		Id:       id,
		Type:     nodeType,
		Address:  &pb.NodeAddress{Address: address},
		Metadata: metadata,
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0777); err != nil {
			return nil, err
		}
	}

	bucketIdentifier := id.String()[:5] // need a way to differentiate between nodes if running more than one simultaneously
	dbpath := filepath.Join(path, fmt.Sprintf("kademlia_%s.db", bucketIdentifier))

	dbs, err := boltdb.NewShared(dbpath, KademliaBucket, NodeBucket)
	if err != nil {
		return nil, BootstrapErr.Wrap(err)
	}
	kdb, ndb := dbs[0], dbs[1]

	rt, err := NewRoutingTable(self, kdb, ndb)
	if err != nil {
		return nil, BootstrapErr.Wrap(err)
	}

	return NewKademliaWithRoutingTable(log, self, bootstrapNodes, identity, alpha, rt)
}

// NewKademliaWithRoutingTable returns a newly configured Kademlia instance
func NewKademliaWithRoutingTable(log *zap.Logger, self pb.Node, bootstrapNodes []pb.Node, identity *provider.FullIdentity, alpha int, rt *RoutingTable) (*Kademlia, error) {
	k := &Kademlia{
		log:            log,
		alpha:          alpha,
		routingTable:   rt,
		bootstrapNodes: bootstrapNodes,
		identity:       identity,
	}
	nc, err := node.NewNodeClient(identity, self, k)
	if err != nil {
		return nil, BootstrapErr.Wrap(err)
	}
	k.nodeClient = nc
	return k, nil
}

// Disconnect safely closes connections to the Kademlia network
func (k *Kademlia) Disconnect() error {
	// Cancel the bootstrap context
	ptr := atomic.LoadPointer(&k.bootstrapCancel)
	if ptr != nil {
		(*(*context.CancelFunc)(ptr))()
	}
	return errs.Combine(
		k.nodeClient.Disconnect(),
		k.routingTable.Close(),
	)
}

// GetNodes returns all nodes from a starting node up to a maximum limit
// stored in the local routing table limiting the result by the specified restrictions
func (k *Kademlia) GetNodes(ctx context.Context, start storj.NodeID, limit int, restrictions ...pb.Restriction) ([]*pb.Node, error) {
	nodes := []*pb.Node{}
	iteratorMethod := func(it storage.Iterator) error {
		var item storage.ListItem
		maxLimit := storage.LookupLimit
		for ; maxLimit > 0 && it.Next(&item); maxLimit-- {
			var (
				id   storj.NodeID
				node = &pb.Node{}
			)
			err := id.Unmarshal(item.Key)
			if err != nil {
				return Error.Wrap(err)
			}
			err = proto.Unmarshal(item.Value, node)
			if err != nil {
				return Error.Wrap(err)
			}
			node.Id = id
			if meetsRestrictions(restrictions, *node) {
				nodes = append(nodes, node)
			}
			if len(nodes) == limit {
				return nil
			}
		}
		return nil
	}
	err := k.routingTable.iterate(
		storage.IterateOptions{
			First:   storage.Key(start.Bytes()),
			Recurse: true,
		},
		iteratorMethod,
	)
	if err != nil {
		return []*pb.Node{}, Error.Wrap(err)
	}
	return nodes, nil
}

// GetRoutingTable provides the routing table for the Kademlia DHT
func (k *Kademlia) GetRoutingTable(ctx context.Context) (dht.RoutingTable, error) {
	return k.routingTable, nil
}

// Bootstrap contacts one of a set of pre defined trusted nodes on the network and
// begins populating the local Kademlia node
func (k *Kademlia) Bootstrap(ctx context.Context) error {
	if len(k.bootstrapNodes) == 0 {
		return BootstrapErr.New("no bootstrap nodes provided")
	}
	bootstrapContext, bootstrapCancel := context.WithCancel(ctx)
	atomic.StorePointer(&k.bootstrapCancel, unsafe.Pointer(&bootstrapCancel))
	//find nodes most similar to self
	_, err := k.lookup(bootstrapContext, k.routingTable.self.Id, true)
	return err
}

// Ping checks that the provided node is still accessible on the network
func (k *Kademlia) Ping(ctx context.Context, node pb.Node) (pb.Node, error) {
	ok, err := k.nodeClient.Ping(ctx, node)
	if err != nil {
		return pb.Node{}, NodeErr.Wrap(err)
	}
	if !ok {
		return pb.Node{}, NodeErr.New("Failed pinging node")
	}
	return node, nil
}

// FindNode looks up the provided NodeID first in the local Node, and if it is not found
// begins searching the network for the NodeID. Returns and error if node was not found
func (k *Kademlia) FindNode(ctx context.Context, ID storj.NodeID) (pb.Node, error) {
	return k.lookup(ctx, ID, false)
}

//lookup initiates a kadmelia node lookup
func (k *Kademlia) lookup(ctx context.Context, ID storj.NodeID, isBootstrap bool) (pb.Node, error) {
	kb := k.routingTable.K()
	var nodes []*pb.Node
	if isBootstrap {
		for _, v := range k.bootstrapNodes {
			nodes = append(nodes, &v)
		}
	} else {
		var err error
		nodes, err = k.routingTable.FindNear(ID, kb)
		if err != nil {
			return pb.Node{}, err
		}
	}
	lookup := newPeerDiscovery(k.log, nodes, k.nodeClient, ID, discoveryOptions{
		concurrency: k.alpha, retries: defaultRetries, bootstrap: isBootstrap, bootstrapNodes: k.bootstrapNodes,
	})
	target, err := lookup.Run(ctx)
	if err != nil {
		return pb.Node{}, err
	}
	bucket, err := k.routingTable.getKBucketID(ID)
	if err != nil {
		k.log.Warn("Error getting getKBucketID in kad lookup")
	} else {
		err = k.routingTable.SetBucketTimestamp(bucket[:], time.Now())
		if err != nil {
			k.log.Warn("Error updating bucket timestamp in kad lookup")
		}
	}
	if target == nil {
		if isBootstrap {
			return pb.Node{}, nil
		}
		return pb.Node{}, NodeNotFound.New("")
	}
	return *target, nil
}

// Seen returns all nodes that this kademlia instance has successfully communicated with
func (k *Kademlia) Seen() []*pb.Node {
	nodes := []*pb.Node{}
	k.routingTable.mutex.Lock()
	for _, v := range k.routingTable.seen {
		nodes = append(nodes, pb.CopyNode(v))
	}
	k.routingTable.mutex.Unlock()
	return nodes
}

// GetIntroNode determines the best node to bootstrap a new node onto the network
func GetIntroNode(addr string) (*pb.Node, error) {
	if addr == "" {
		addr = "bootstrap.storj.io:8080"
	}
	return &pb.Node{
		Address: &pb.NodeAddress{
			Transport: defaultTransport,
			Address:   addr,
		},
	}, nil
}

//StartRefresh occasionally refreshes stale kad buckets
func (k *Kademlia) StartRefresh(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		time.Sleep(time.Duration(rand.Intn(300)) * time.Second) //stagger
		for {
			if err := k.refresh(ctx); err != nil {
				k.log.Warn("bucket refresh failed", zap.Error(err))
			}
			select {
			case <-ticker.C:
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

//refresh updates each Kademlia bucket not contacted in the last hour
func (k *Kademlia) refresh(ctx context.Context) error {
	bIDs, err := k.routingTable.GetBucketIds()
	if err != nil {
		return Error.Wrap(err)
	}
	now := time.Now()
	startID := bucketID{}
	var errors errs.Group
	for _, bID := range bIDs {
		ts, tErr := k.routingTable.GetBucketTimestamp(bID)
		if tErr != nil {
			errors.Add(tErr)
		} else if now.After(ts.Add(time.Hour)) {
			rID, _ := randomIDInRange(startID, keyToBucketID(bID))
			_, _ = k.FindNode(ctx, rID) // ignore node not found
		}
	}
	return Error.Wrap(errors.Err())
}

// randomIDInRange finds a random node ID with a range (start..end]
func randomIDInRange(start, end bucketID) (storj.NodeID, error) {
	randID := storj.NodeID{}
	divergedHigh := false
	divergedLow := false
	for x := 0; x < 32; x++ {
		s := byte(0)
		if !divergedLow {
			s = start[x]
		}
		e := byte(255)
		if !divergedHigh {
			e = end[x]
		}
		if s > e {
			return storj.NodeID{}, errs.New("Random id range was invalid")
		}
		if s == e {
			randID[x] = s
		} else {
			r := s + byte(rand.Intn(int(e-s))) + 1
			if r < e {
				divergedHigh = true
			}
			if r > s {
				divergedLow = true
			}
			randID[x] = r
		}
	}
	if !divergedLow {
		if !divergedHigh { // start == end
			return storj.NodeID{}, errs.New("Random id range was invalid")
		} else if randID[31] == start[31] { // start == randID
			randID[31] = start[31] + 1
		}
	}
	return randID, nil
}

// Restrict is used to limit nodes returned that don't match the miniumum storage requirements
func Restrict(r pb.Restriction, n []*pb.Node) []*pb.Node {
	oper := r.GetOperand()
	op := r.GetOperator()
	val := r.GetValue()
	var comp int64

	results := []*pb.Node{}
	for _, v := range n {
		switch oper {
		case pb.Restriction_FREE_BANDWIDTH:
			comp = v.GetRestrictions().GetFreeBandwidth()
		case pb.Restriction_FREE_DISK:
			comp = v.GetRestrictions().GetFreeDisk()
		}

		switch op {
		case pb.Restriction_EQ:
			if comp == val {
				results = append(results, v)
				continue
			}
		case pb.Restriction_LT:
			if comp < val {
				results = append(results, v)
				continue
			}
		case pb.Restriction_LTE:
			if comp <= val {
				results = append(results, v)
				continue
			}
		case pb.Restriction_GT:
			if comp > val {
				results = append(results, v)
				continue
			}
		case pb.Restriction_GTE:
			if comp >= val {
				results = append(results, v)
				continue
			}
		}
	}
	return results
}

func meetsRestrictions(rs []pb.Restriction, n pb.Node) bool {
	for _, r := range rs {
		oper := r.GetOperand()
		op := r.GetOperator()
		val := r.GetValue()
		var comp int64
		switch oper {
		case pb.Restriction_FREE_BANDWIDTH:
			comp = n.GetRestrictions().GetFreeBandwidth()
		case pb.Restriction_FREE_DISK:
			comp = n.GetRestrictions().GetFreeDisk()
		}
		switch op {
		case pb.Restriction_EQ:
			if comp != val {
				return false
			}
		case pb.Restriction_LT:
			if comp >= val {
				return false
			}
		case pb.Restriction_LTE:
			if comp > val {
				return false
			}
		case pb.Restriction_GT:
			if comp <= val {
				return false
			}
		case pb.Restriction_GTE:
			if comp < val {
				return false
			}
		}
	}
	return true
}
