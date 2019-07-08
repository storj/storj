// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

const (
	// KademliaBucket is the string representing the bucket used for the kademlia routing table k-bucket ids
	KademliaBucket = "kbuckets"
	// NodeBucket is the string representing the bucket used for the kademlia routing table node ids
	NodeBucket = "nodes"
	// AntechamberBucket is the string representing the bucket used for the kademlia antechamber nodes
	AntechamberBucket = "antechamber"
)

// RoutingErr is the class for all errors pertaining to routing table operations
var RoutingErr = errs.Class("routing table error")

// Bucket IDs exist in the same address space as node IDs
type bucketID = storj.NodeID

var firstBucketID = bucketID{
	0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF,

	0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF,
}

var emptyBucketID = bucketID{}

// RoutingTableConfig configures the routing table
type RoutingTableConfig struct {
	BucketSize           int `help:"size of each Kademlia bucket" default:"20"`
	ReplacementCacheSize int `help:"size of Kademlia replacement cache" default:"5"`
}

// RoutingTable implements the RoutingTable interface
type RoutingTable struct {
	log              *zap.Logger
	self             *overlay.NodeDossier
	kadBucketDB      storage.KeyValueStore
	nodeBucketDB     storage.KeyValueStore
	transport        *pb.NodeTransport
	mutex            *sync.Mutex
	rcMutex          *sync.Mutex
	acMutex          *sync.Mutex
	replacementCache map[bucketID][]*pb.Node
	bucketSize       int // max number of nodes stored in a kbucket = 20 (k)
	rcBucketSize     int // replacementCache bucket max length
	antechamber      storage.KeyValueStore
}

// NewRoutingTable returns a newly configured instance of a RoutingTable
func NewRoutingTable(logger *zap.Logger, localNode *overlay.NodeDossier, kdb, ndb, adb storage.KeyValueStore, config *RoutingTableConfig) (_ *RoutingTable, err error) {
	if config == nil || config.BucketSize == 0 || config.ReplacementCacheSize == 0 {
		// TODO: handle this more nicely
		config = &RoutingTableConfig{
			BucketSize:           20,
			ReplacementCacheSize: 5,
		}
	}

	rt := &RoutingTable{
		log:          logger,
		self:         localNode,
		kadBucketDB:  kdb,
		nodeBucketDB: ndb,
		transport:    &defaultTransport,

		mutex:            &sync.Mutex{},
		rcMutex:          &sync.Mutex{},
		acMutex:          &sync.Mutex{},
		replacementCache: make(map[bucketID][]*pb.Node),

		bucketSize:   config.BucketSize,
		rcBucketSize: config.ReplacementCacheSize,
		antechamber:  adb,
	}
	ok, err := rt.addNode(context.TODO(), &localNode.Node)
	if !ok || err != nil {
		return nil, RoutingErr.New("could not add localNode to routing table: %s", err)
	}
	return rt, nil
}

// Close closes without closing dependencies
func (rt *RoutingTable) Close() error {
	return nil
}

// Local returns the local node
func (rt *RoutingTable) Local() overlay.NodeDossier {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()
	return *rt.self
}

// UpdateSelf updates the local node with the provided info
func (rt *RoutingTable) UpdateSelf(capacity *pb.NodeCapacity) {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()
	if capacity != nil {
		rt.self.Capacity = *capacity
	}
}

// K returns the currently configured maximum of nodes to store in a bucket
func (rt *RoutingTable) K() int {
	return rt.bucketSize
}

// CacheSize returns the total current size of the replacement cache
func (rt *RoutingTable) CacheSize() int {
	return rt.rcBucketSize
}

// GetNodes retrieves nodes within the same kbucket as the given node id
// Note: id doesn't need to be stored at time of search
func (rt *RoutingTable) GetNodes(ctx context.Context, id storj.NodeID) ([]*pb.Node, bool) {
	defer mon.Task()(&ctx)(nil)
	bID, err := rt.getKBucketID(ctx, id)
	if err != nil {
		return nil, false
	}
	if bID == (bucketID{}) {
		return nil, false
	}
	unmarshaledNodes, err := rt.getUnmarshaledNodesFromBucket(ctx, bID)
	if err != nil {
		return nil, false
	}
	return unmarshaledNodes, true
}

// GetBucketIds returns a storage.Keys type of bucket ID's in the Kademlia instance
func (rt *RoutingTable) GetBucketIds(ctx context.Context) (_ storage.Keys, err error) {
	defer mon.Task()(&ctx)(&err)

	kbuckets, err := rt.kadBucketDB.List(ctx, nil, 0)
	if err != nil {
		return nil, err
	}
	return kbuckets, nil
}

// DumpNodes iterates through all nodes in the nodeBucketDB and marshals them to &pb.Nodes, then returns them
func (rt *RoutingTable) DumpNodes(ctx context.Context) (_ []*pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)

	var nodes []*pb.Node
	var nodeErrors errs.Group

	err = rt.iterateNodes(ctx, storj.NodeID{}, func(ctx context.Context, newID storj.NodeID, protoNode []byte) error {
		newNode := pb.Node{}
		err := proto.Unmarshal(protoNode, &newNode)
		if err != nil {
			nodeErrors.Add(err)
		}
		nodes = append(nodes, &newNode)
		return nil
	}, false)

	if err != nil {
		nodeErrors.Add(err)
	}

	return nodes, nodeErrors.Err()
}

// FindNear returns the node corresponding to the provided nodeID
// returns all Nodes (excluding self) closest via XOR to the provided nodeID up to the provided limit
func (rt *RoutingTable) FindNear(ctx context.Context, target storj.NodeID, limit int) (_ []*pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)
	closestNodes := make([]*pb.Node, 0, limit+1)
	err = rt.iterateNodes(ctx, storj.NodeID{}, func(ctx context.Context, newID storj.NodeID, protoNode []byte) error {
		newPos := len(closestNodes)
		for ; newPos > 0 && compareByXor(closestNodes[newPos-1].Id, newID, target) > 0; newPos-- {
		}
		if newPos != limit {
			newNode := pb.Node{}
			err := proto.Unmarshal(protoNode, &newNode)
			if err != nil {
				return err
			}
			closestNodes = append(closestNodes, &newNode)
			if newPos != len(closestNodes) { //reorder
				copy(closestNodes[newPos+1:], closestNodes[newPos:])
				closestNodes[newPos] = &newNode
				if len(closestNodes) > limit {
					closestNodes = closestNodes[:limit]
				}
			}
		}
		return nil
	}, true)
	return closestNodes, Error.Wrap(err)
}

// ConnectionSuccess updates or adds a node to the routing table when
// a successful connection is made to the node on the network
func (rt *RoutingTable) ConnectionSuccess(ctx context.Context, node *pb.Node) (err error) {
	defer mon.Task()(&ctx)(&err)
	// valid to connect to node without ID but don't store connection
	if node.Id == (storj.NodeID{}) {
		return nil
	}

	v, err := rt.nodeBucketDB.Get(ctx, storage.Key(node.Id.Bytes()))
	if err != nil && !storage.ErrKeyNotFound.Has(err) {
		return RoutingErr.New("could not get node %s", err)
	}
	if v != nil {
		err = rt.updateNode(ctx, node)
		if err != nil {
			return RoutingErr.New("could not update node %s", err)
		}
		return nil
	}
	_, err = rt.addNode(ctx, node)
	if err != nil {
		return RoutingErr.New("could not add node %s", err)
	}

	return nil
}

// ConnectionFailed removes a node from the routing table when
// a connection fails for the node on the network
func (rt *RoutingTable) ConnectionFailed(ctx context.Context, node *pb.Node) (err error) {
	defer mon.Task()(&ctx)(&err)
	err = rt.removeNode(ctx, node)
	if err != nil {
		return RoutingErr.New("could not remove node %s", err)
	}
	return nil
}

// SetBucketTimestamp records the time of the last node lookup for a bucket
func (rt *RoutingTable) SetBucketTimestamp(ctx context.Context, bIDBytes []byte, now time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	rt.mutex.Lock()
	defer rt.mutex.Unlock()
	err = rt.createOrUpdateKBucket(ctx, keyToBucketID(bIDBytes), now)
	if err != nil {
		return NodeErr.New("could not update bucket timestamp %s", err)
	}
	return nil
}

// GetBucketTimestamp retrieves time of the last node lookup for a bucket
func (rt *RoutingTable) GetBucketTimestamp(ctx context.Context, bIDBytes []byte) (_ time.Time, err error) {
	defer mon.Task()(&ctx)(&err)

	t, err := rt.kadBucketDB.Get(ctx, bIDBytes)
	if err != nil {
		return time.Now(), RoutingErr.New("could not get bucket timestamp %s", err)
	}
	timestamp, _ := binary.Varint(t)
	return time.Unix(0, timestamp).UTC(), nil
}

func (rt *RoutingTable) iterateNodes(ctx context.Context, start storj.NodeID, f func(context.Context, storj.NodeID, []byte) error, skipSelf bool) (err error) {
	defer mon.Task()(&ctx)(&err)
	return rt.nodeBucketDB.Iterate(ctx, storage.IterateOptions{First: storage.Key(start.Bytes()), Recurse: true},
		func(ctx context.Context, it storage.Iterator) error {
			var item storage.ListItem
			for it.Next(ctx, &item) {
				nodeID, err := storj.NodeIDFromBytes(item.Key)
				if err != nil {
					return err
				}
				if skipSelf && nodeID == rt.self.Id {
					continue
				}
				err = f(ctx, nodeID, item.Value)
				if err != nil {
					return err
				}
			}
			return nil
		},
	)
}

// ConnFailure implements the Transport failure function
func (rt *RoutingTable) ConnFailure(ctx context.Context, node *pb.Node, err error) {
	err2 := rt.ConnectionFailed(ctx, node)
	if err2 != nil {
		zap.L().Debug(fmt.Sprintf("error with ConnFailure hook  %+v : %+v", err, err2))
	}
}

// ConnSuccess implements the Transport success function
func (rt *RoutingTable) ConnSuccess(ctx context.Context, node *pb.Node) {
	err := rt.ConnectionSuccess(ctx, node)
	if err != nil {
		zap.L().Debug("connection success error:", zap.Error(err))
	}
}
