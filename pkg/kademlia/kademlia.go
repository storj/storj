// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/storage"
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
	log            *zap.Logger
	alpha          int // alpha is a system wide concurrency parameter
	routingTable   *RoutingTable
	bootstrapNodes []pb.Node
	dialer         *Dialer
	lookups        sync2.WorkGroup

	bootstrapFinished sync2.Fence

	refreshThreshold int64
	RefreshBuckets   sync2.Cycle

	mu          sync.Mutex
	lastPinged  time.Time
	lastQueried time.Time
}

// NewService returns a newly configured Kademlia instance
func NewService(log *zap.Logger, self pb.Node, bootstrapNodes []pb.Node, transport transport.Client, alpha int, rt *RoutingTable) (*Kademlia, error) {
	k := &Kademlia{
		log:              log,
		alpha:            alpha,
		routingTable:     rt,
		bootstrapNodes:   bootstrapNodes,
		dialer:           NewDialer(log.Named("dialer"), transport),
		refreshThreshold: int64(time.Minute),
	}

	return k, nil
}

// Close closes all kademlia connections and prevents new ones from being created.
func (k *Kademlia) Close() error {
	dialerErr := k.dialer.Close()
	k.lookups.Close()
	k.lookups.Wait()
	return dialerErr
}

// LastPinged returns last time someone pinged this node.
func (k *Kademlia) LastPinged() time.Time {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.lastPinged
}

// Pinged notifies the service it has been remotely pinged.
func (k *Kademlia) Pinged() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.lastPinged = time.Now()
}

// LastQueried returns last time someone queried this node.
func (k *Kademlia) LastQueried() time.Time {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.lastQueried
}

// Queried notifies the service it has been remotely queried
func (k *Kademlia) Queried() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.lastQueried = time.Now()
}

// FindNear returns all nodes from a starting node up to a maximum limit
// stored in the local routing table limiting the result by the specified restrictions
func (k *Kademlia) FindNear(ctx context.Context, start storj.NodeID, limit int, restrictions ...pb.Restriction) ([]*pb.Node, error) {
	return k.routingTable.FindNear(start, limit, restrictions...)
}

// GetBucketIds returns a storage.Keys type of bucket ID's in the Kademlia instance
func (k *Kademlia) GetBucketIds() (storage.Keys, error) {
	return k.routingTable.GetBucketIds()
}

// Local returns the local nodes ID
func (k *Kademlia) Local() pb.Node {
	return k.routingTable.Local()
}

// SetBootstrapNodes sets the bootstrap nodes.
// Must be called before anything starting to use kademlia.
func (k *Kademlia) SetBootstrapNodes(nodes []pb.Node) { k.bootstrapNodes = nodes }

// GetBootstrapNodes gets the bootstrap nodes.
func (k *Kademlia) GetBootstrapNodes() []pb.Node { return k.bootstrapNodes }

// DumpNodes returns all the nodes in the node database
func (k *Kademlia) DumpNodes(ctx context.Context) ([]*pb.Node, error) {
	return k.routingTable.DumpNodes()
}

// Bootstrap contacts one of a set of pre defined trusted nodes on the network and
// begins populating the local Kademlia node
func (k *Kademlia) Bootstrap(ctx context.Context) error {
	defer k.bootstrapFinished.Release()

	if !k.lookups.Start() {
		return context.Canceled
	}
	defer k.lookups.Done()

	if len(k.bootstrapNodes) == 0 {
		k.log.Warn("No bootstrap address specified.")
		return nil
	}

	var errs errs.Group
	for i, node := range k.bootstrapNodes {
		if ctx.Err() != nil {
			errs.Add(ctx.Err())
			return errs.Err()
		}

		p := &peer.Peer{}
		pCall := grpc.Peer(p)
		_, err := k.dialer.PingAddress(ctx, node.Address.Address, pCall)
		if err != nil {
			errs.Add(err)
		}

		ident, err := identity.PeerIdentityFromPeer(p)
		if err != nil {
			errs.Add(err)
			continue
		}

		k.routingTable.mutex.Lock()
		node.Id = ident.ID
		k.bootstrapNodes[i] = node
		k.routingTable.mutex.Unlock()
		if err == nil {
			// We have pinged successfully one bootstrap node.
			// Clear any errors and break the cycle.
			errs = nil
			break
		}
		errs.Add(err)
	}
	err := errs.Err()
	if err != nil {
		return err
	}

	//find nodes most similar to self
	k.routingTable.mutex.Lock()
	id := k.routingTable.self.Id
	k.routingTable.mutex.Unlock()
	_, err = k.lookup(ctx, id, true)

	// TODO(dylan): We do not currently handle this last bit of behavior.
	// ```
	// Finally, u refreshes all k-buckets further away than its closest neighbor.
	// During the refreshes, u both populates its own k-buckets and inserts
	// itself into other nodes' k-buckets as necessary.
	// ``

	return err
}

// WaitForBootstrap waits for bootstrap pinging has been completed.
func (k *Kademlia) WaitForBootstrap() {
	k.bootstrapFinished.Wait()
}

// FetchPeerIdentity connects to a node and returns its peer identity
func (k *Kademlia) FetchPeerIdentity(ctx context.Context, nodeID storj.NodeID) (*identity.PeerIdentity, error) {
	if !k.lookups.Start() {
		return nil, context.Canceled
	}
	defer k.lookups.Done()
	node, err := k.FindNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	return k.dialer.FetchPeerIdentity(ctx, node)
}

// Ping checks that the provided node is still accessible on the network
func (k *Kademlia) Ping(ctx context.Context, node pb.Node) (pb.Node, error) {
	if !k.lookups.Start() {
		return pb.Node{}, context.Canceled
	}
	defer k.lookups.Done()

	ok, err := k.dialer.PingNode(ctx, node)
	if err != nil {
		return pb.Node{}, NodeErr.Wrap(err)
	}
	if !ok {
		return pb.Node{}, NodeErr.New("Failed pinging node")
	}
	return node, nil
}

// FetchInfo connects to a node address and returns the node info
func (k *Kademlia) FetchInfo(ctx context.Context, node pb.Node) (*pb.InfoResponse, error) {
	if !k.lookups.Start() {
		return nil, context.Canceled
	}
	defer k.lookups.Done()

	info, err := k.dialer.FetchInfo(ctx, node)
	if err != nil {
		return nil, NodeErr.Wrap(err)
	}
	return info, nil
}

// FindNode looks up the provided NodeID first in the local Node, and if it is not found
// begins searching the network for the NodeID. Returns and error if node was not found
func (k *Kademlia) FindNode(ctx context.Context, ID storj.NodeID) (pb.Node, error) {
	if !k.lookups.Start() {
		return pb.Node{}, context.Canceled
	}
	defer k.lookups.Done()

	return k.lookup(ctx, ID, false)
}

//lookup initiates a kadmelia node lookup
func (k *Kademlia) lookup(ctx context.Context, ID storj.NodeID, isBootstrap bool) (pb.Node, error) {
	if !k.lookups.Start() {
		return pb.Node{}, context.Canceled
	}
	defer k.lookups.Done()

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
	lookup := newPeerDiscovery(k.log, k.routingTable.Local(), nodes, k.dialer, ID, discoveryOptions{
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

// SetBucketRefreshThreshold changes the threshold when buckets are considered stale and need refreshing.
func (k *Kademlia) SetBucketRefreshThreshold(threshold time.Duration) {
	atomic.StoreInt64(&k.refreshThreshold, int64(threshold))
}

// Run occasionally refreshes stale kad buckets
func (k *Kademlia) Run(ctx context.Context) error {
	if !k.lookups.Start() {
		return context.Canceled
	}
	defer k.lookups.Done()

	k.RefreshBuckets.SetInterval(5 * time.Minute)
	return k.RefreshBuckets.Run(ctx, func(ctx context.Context) error {
		threshold := time.Duration(atomic.LoadInt64(&k.refreshThreshold))
		err := k.refresh(ctx, threshold)
		if err != nil {
			k.log.Warn("bucket refresh failed", zap.Error(err))
		}
		return nil
	})
}

// refresh updates each Kademlia bucket not contacted in the last hour
func (k *Kademlia) refresh(ctx context.Context, threshold time.Duration) error {
	bIDs, err := k.routingTable.GetBucketIds()
	if err != nil {
		return Error.Wrap(err)
	}
	now := time.Now()
	startID := bucketID{}
	var errors errs.Group
	for _, bID := range bIDs {
		endID := keyToBucketID(bID)
		ts, tErr := k.routingTable.GetBucketTimestamp(bID)
		if tErr != nil {
			errors.Add(tErr)
		} else if now.After(ts.Add(threshold)) {
			rID, _ := randomIDInRange(startID, endID)
			_, _ = k.FindNode(ctx, rID) // ignore node not found
		}
		startID = endID
	}
	return Error.Wrap(errors.Err())
}

// randomIDInRange finds a random node ID with a range (start..end]
func randomIDInRange(start, end bucketID) (storj.NodeID, error) {
	randID := storj.NodeID{}
	divergedHigh := false
	divergedLow := false
	for x := 0; x < len(randID); x++ {
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
		} else if randID[len(randID)-1] == start[len(randID)-1] { // start == randID
			randID[len(randID)-1] = start[len(randID)-1] + 1
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
