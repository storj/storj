// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package kademlia

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/storj"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/storage"
	"storj.io/storj/storage/storelogger"
	"storj.io/storj/storage/teststore"
)

// newTestRoutingTable returns a newly configured instance of a RoutingTable
func newTestRoutingTable(localNode storj.Node) (*RoutingTable, error) {
	rt := &RoutingTable{
		self:             localNode,
		kadBucketDB:      storelogger.New(zap.L(), teststore.New()),
		nodeBucketDB:     storelogger.New(zap.L(), teststore.New()),
		transport:        &defaultTransport,
		mutex:            &sync.Mutex{},
		replacementCache: make(map[string][]storj.Node),
		idLength:         16,
		bucketSize:       6,
		rcBucketSize:     2,
	}
	ok, err := rt.addNode(localNode)
	if !ok || err != nil {
		return nil, RoutingErr.New("could not add localNode to routing table: %s", err)
	}
	return rt, nil
}

func createRoutingTable(t *testing.T, localNodeID storj.NodeID) (*RoutingTable, func()) {
	if localNodeID == nil {
		localNodeID = testIDA
	}
	localNode := storj.NewNodeWithID(localNodeID, &pb.Node{})

	rt, err := newTestRoutingTable(localNode)
	if err != nil {
		t.Fatal(err)
	}

	return rt, func() {
		err := rt.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}

func newNodeFromID(id storj.NodeID) storj.Node {
	return storj.Node{Id: id, Node: &pb.Node{}}
}

func newNodeFromIDString(s string) storj.Node {
	return storj.NewNodeWithID(teststorj.NodeIDFromString(s), &pb.Node{})
}

func TestAddNode(t *testing.T) {
	// TODO(bryanchriiswhite): UNSKIP
	t.SkipNow()
	rt, cleanup := createRoutingTable(t, teststorj.NodeIDFromString("OO"))
	defer cleanup()
	bucket, err := rt.kadBucketDB.Get(storage.Key([]byte{255, 255}))
	assert.NoError(t, err)
	assert.NotNil(t, bucket)
	cases := []struct {
		testID  string
		node    storj.Node
		added   bool
		kadIDs  [][]byte
		nodeIDs [][]string
	}{
		{testID: "PO: add node to unfilled kbucket",
			node:    newNodeFromIDString("PO"),
			added:   true,
			kadIDs:  [][]byte{{255, 255}},
			nodeIDs: [][]string{{"OO", "PO"}},
		},
		{testID: "NO: add node to full kbucket and split",
			node:    newNodeFromIDString("NO"),
			added:   true,
			kadIDs:  [][]byte{{255, 255}},
			nodeIDs: [][]string{{"NO", "OO", "PO"}},
		},
		{testID: "MO",
			node:    newNodeFromIDString("MO"),
			added:   true,
			kadIDs:  [][]byte{{255, 255}},
			nodeIDs: [][]string{{"MO", "NO", "OO", "PO"}},
		},
		{testID: "LO",
			node:    newNodeFromIDString("LO"),
			added:   true,
			kadIDs:  [][]byte{{255, 255}},
			nodeIDs: [][]string{{"LO", "MO", "NO", "OO", "PO"}},
		},
		{testID: "QO",
			node:    newNodeFromIDString("QO"),
			added:   true,
			kadIDs:  [][]byte{{255, 255}},
			nodeIDs: [][]string{{"LO", "MO", "NO", "OO", "PO", "QO"}},
		},
		{testID: "SO: split bucket",
			node:    newNodeFromIDString("SO"),
			added:   true,
			kadIDs:  [][]byte{{63, 255}, {79, 255}, {95, 255}, {127, 255}, {255, 255}},
			nodeIDs: [][]string{{}, {"LO", "MO", "NO", "OO"}, {"PO", "QO", "SO"}, {}, {}},
		},
		{testID: "?O",
			node:    newNodeFromIDString("?O"),
			added:   true,
			kadIDs:  [][]byte{{63, 255}, {79, 255}, {95, 255}, {127, 255}, {255, 255}},
			nodeIDs: [][]string{{"?O"}, {"LO", "MO", "NO", "OO"}, {"PO", "QO", "SO"}, {}, {}},
		},
		{testID: ">O",
			node:    newNodeFromIDString(">O"),
			added:   true,
			kadIDs:  [][]byte{[]byte{63, 255}, []byte{79, 255}, []byte{95, 255}, []byte{127, 255}, []byte{255, 255}},
			nodeIDs: [][]string{{">O", "?O"}, {"LO", "MO", "NO", "OO"}, {"PO", "QO", "SO"}, {}, {}},
		},
		{testID: "=O",
			node:    newNodeFromIDString("=O"),
			added:   true,
			kadIDs:  [][]byte{[]byte{63, 255}, []byte{79, 255}, []byte{95, 255}, []byte{127, 255}, []byte{255, 255}},
			nodeIDs: [][]string{{"=O", ">O", "?O"}, {"LO", "MO", "NO", "OO"}, {"PO", "QO", "SO"}, {}, {}},
		},
		{testID: ";O",
			node:    newNodeFromIDString(";O"),
			added:   true,
			kadIDs:  [][]byte{[]byte{63, 255}, []byte{79, 255}, []byte{95, 255}, []byte{127, 255}, []byte{255, 255}},
			nodeIDs: [][]string{{";O", "=O", ">O", "?O"}, {"LO", "MO", "NO", "OO"}, {"PO", "QO", "SO"}, {}, {}},
		},
		{testID: ":O",
			node:    newNodeFromIDString(":O"),
			added:   true,
			kadIDs:  [][]byte{[]byte{63, 255}, []byte{79, 255}, []byte{95, 255}, []byte{127, 255}, []byte{255, 255}},
			nodeIDs: [][]string{{":O", ";O", "=O", ">O", "?O"}, {"LO", "MO", "NO", "OO"}, {"PO", "QO", "SO"}, {}, {}},
		},
		{testID: "9O",
			node:    newNodeFromIDString("9O"),
			added:   true,
			kadIDs:  [][]byte{[]byte{63, 255}, []byte{79, 255}, []byte{95, 255}, []byte{127, 255}, []byte{255, 255}},
			nodeIDs: [][]string{{"9O", ":O", ";O", "=O", ">O", "?O"}, {"LO", "MO", "NO", "OO"}, {"PO", "QO", "SO"}, {}, {}},
		},
		{testID: "8O: should drop",
			node:    newNodeFromIDString("8O"),
			added:   false,
			kadIDs:  [][]byte{[]byte{63, 255}, []byte{79, 255}, []byte{95, 255}, []byte{127, 255}, []byte{255, 255}},
			nodeIDs: [][]string{{"9O", ":O", ";O", "=O", ">O", "?O"}, {"LO", "MO", "NO", "OO"}, {"PO", "QO", "SO"}, {}, {}},
		},
		{testID: "KO",
			node:    newNodeFromIDString("KO"),
			added:   true,
			kadIDs:  [][]byte{[]byte{63, 255}, []byte{79, 255}, []byte{95, 255}, []byte{127, 255}, []byte{255, 255}},
			nodeIDs: [][]string{{"9O", ":O", ";O", "=O", ">O", "?O"}, {"KO", "LO", "MO", "NO", "OO"}, {"PO", "QO", "SO"}, {}, {}},
		},
		{testID: "JO",
			node:    newNodeFromIDString("JO"),
			added:   true,
			kadIDs:  [][]byte{[]byte{63, 255}, []byte{79, 255}, []byte{95, 255}, []byte{127, 255}, []byte{255, 255}},
			nodeIDs: [][]string{{"9O", ":O", ";O", "=O", ">O", "?O"}, {"JO", "KO", "LO", "MO", "NO", "OO"}, {"PO", "QO", "SO"}, {}, {}},
		},
		{testID: "]O",
			node:    newNodeFromIDString("]O"),
			added:   true,
			kadIDs:  [][]byte{[]byte{63, 255}, []byte{79, 255}, []byte{95, 255}, []byte{127, 255}, []byte{255, 255}},
			nodeIDs: [][]string{{"9O", ":O", ";O", "=O", ">O", "?O"}, {"JO", "KO", "LO", "MO", "NO", "OO"}, {"PO", "QO", "SO", "]O"}, {}, {}},
		},
		{testID: "^O",
			node:    newNodeFromIDString("^O"),
			added:   true,
			kadIDs:  [][]byte{[]byte{63, 255}, []byte{79, 255}, []byte{95, 255}, []byte{127, 255}, []byte{255, 255}},
			nodeIDs: [][]string{{"9O", ":O", ";O", "=O", ">O", "?O"}, {"JO", "KO", "LO", "MO", "NO", "OO"}, {"PO", "QO", "SO", "]O", "^O"}, {}, {}},
		},
		{testID: "_O",
			node:    newNodeFromIDString("_O"),
			added:   true,
			kadIDs:  [][]byte{[]byte{63, 255}, []byte{79, 255}, []byte{95, 255}, []byte{127, 255}, []byte{255, 255}},
			nodeIDs: [][]string{{"9O", ":O", ";O", "=O", ">O", "?O"}, {"JO", "KO", "LO", "MO", "NO", "OO"}, {"PO", "QO", "SO", "]O", "^O", "_O"}, {}, {}},
		},
		{testID: "@O: split bucket 2",
			node:    newNodeFromIDString("@O"),
			added:   true,
			kadIDs:  [][]byte{[]byte{63, 255}, []byte{71, 255}, []byte{79, 255}, []byte{95, 255}, []byte{127, 255}, []byte{255, 255}},
			nodeIDs: [][]string{{"9O", ":O", ";O", "=O", ">O", "?O"}, {"@O"}, {"JO", "KO", "LO", "MO", "NO", "OO"}, {"PO", "QO", "SO", "]O", "^O", "_O"}, {}, {}},
		},
	}
	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			ok, err := rt.addNode(c.node)
			assert.Equal(t, c.added, ok)
			assert.NoError(t, err)
			kadKeys, err := rt.kadBucketDB.List(nil, 0)
			assert.NoError(t, err)
			for i, v := range kadKeys {
				assert.Equal(t, storage.Key(c.kadIDs[i]), v)
				a, err := rt.getNodeIDsWithinKBucket(v)
				assert.NoError(t, err)
				for j, w := range a {
					assert.Equal(t, c.nodeIDs[i][j], string(w))
				}
			}

			if c.testID == "8O" {
				n := rt.replacementCache["8O"]
				assert.Equal(t, "8O", n[0].Id)
			}

		})
	}
}

func TestUpdateNode(t *testing.T) {
	rt, cleanup := createRoutingTable(t, testIDA)
	defer cleanup()
	node := newNodeFromID(testIDB)
	ok, err := rt.addNode(node)
	assert.True(t, ok)
	assert.NoError(t, err)
	val, err := rt.nodeBucketDB.Get(storage.Key(node.Id.Bytes()))
	assert.NoError(t, err)
	unmarshaled, err := unmarshalNodes(storage.Keys{storage.Key(node.Id.Bytes())}, []storage.Value{val})
	assert.NoError(t, err)
	x := unmarshaled[0].Address
	assert.Nil(t, x)

	node.Address = &pb.NodeAddress{Address: "BB"}
	err = rt.updateNode(node)
	assert.NoError(t, err)
	val, err = rt.nodeBucketDB.Get(storage.Key(node.Id.Bytes()))
	assert.NoError(t, err)
	unmarshaled, err = unmarshalNodes(storage.Keys{storage.Key(node.Id.Bytes())}, []storage.Value{val})
	assert.NoError(t, err)
	y := unmarshaled[0].Address.Address
	assert.Equal(t, "BB", y)
}

func TestRemoveNode(t *testing.T) {
	rt, cleanup := createRoutingTable(t, testIDA)
	defer cleanup()
	kadBucketID := []byte{255, 255}
	node := newNodeFromID(testIDB)
	ok, err := rt.addNode(node)
	assert.True(t, ok)
	assert.NoError(t, err)
	val, err := rt.nodeBucketDB.Get(storage.Key(node.Id.Bytes()))
	assert.NoError(t, err)
	assert.NotNil(t, val)
	node2 := newNodeFromID(testIDC)
	rt.addToReplacementCache(kadBucketID, node2)
	err = rt.removeNode(kadBucketID, storage.Key(node.Id.Bytes()))
	assert.NoError(t, err)
	val, err = rt.nodeBucketDB.Get(storage.Key(node.Id.Bytes()))
	assert.Nil(t, val)
	assert.Error(t, err)
	val2, err := rt.nodeBucketDB.Get(storage.Key(node2.Id.Bytes()))
	assert.NoError(t, err)
	assert.NotNil(t, val2)
	assert.Equal(t, 0, len(rt.replacementCache[string(kadBucketID)]))

	//try to remove node not in rt
	err = rt.removeNode(kadBucketID, storage.Key("DD"))
	assert.NoError(t, err)
}

func TestCreateOrUpdateKBucket(t *testing.T) {
	id := []byte{255, 255}
	rt, cleanup := createRoutingTable(t, nil)
	defer cleanup()
	err := rt.createOrUpdateKBucket(storage.Key(id), time.Now())
	assert.NoError(t, err)
	val, e := rt.kadBucketDB.Get(storage.Key(id))
	assert.NotNil(t, val)
	assert.NoError(t, e)

}

func TestGetKBucketID(t *testing.T) {
	kadIDA := storage.Key([]byte{255, 255})
	rt, cleanup := createRoutingTable(t, testIDA)
	defer cleanup()
	keyA, err := rt.getKBucketID(testIDA.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, kadIDA, keyA)
}

func TestXorTwoIds(t *testing.T) {
	x := xorTwoIds([]byte{191}, []byte{159})
	assert.Equal(t, []byte{32}, x) //00100000
}

func TestSortByXOR(t *testing.T) {
	nodeIDBytes := []byte{127, 255} //xor 0
	rt, cleanup := createRoutingTable(t, teststorj.NodeIDFromBytes(nodeIDBytes))
	defer cleanup()
	node2 := []byte{143, 255} //xor 240
	assert.NoError(t, rt.nodeBucketDB.Put(node2, []byte("")))
	node3 := []byte{255, 255} //xor 128
	assert.NoError(t, rt.nodeBucketDB.Put(node3, []byte("")))
	node4 := []byte{191, 255} //xor 192
	assert.NoError(t, rt.nodeBucketDB.Put(node4, []byte("")))
	node5 := []byte{133, 255} //xor 250
	assert.NoError(t, rt.nodeBucketDB.Put(node5, []byte("")))
	nodes, err := rt.nodeBucketDB.List(nil, 0)
	assert.NoError(t, err)
	expectedNodes := storage.Keys{nodeIDBytes, node5, node2, node4, node3}
	assert.Equal(t, expectedNodes, nodes)
	sortByXOR(nodes, nodeIDBytes)
	expectedSorted := storage.Keys{nodeIDBytes, node3, node4, node2, node5}
	assert.Equal(t, expectedSorted, nodes)
	nodes, err = rt.nodeBucketDB.List(nil, 0)
	assert.NoError(t, err)
	assert.Equal(t, expectedNodes, nodes)
}

func BenchmarkSortByXOR(b *testing.B) {
	nodes := []storage.Key{}

	newNodeID := func() storage.Key {
		id := make(storage.Key, 32)
		rand.Read(id[:])
		return id
	}

	for k := 0; k < 1000; k++ {
		nodes = append(nodes, newNodeID())
	}

	b.ResetTimer()
	for m := 0; m < b.N; m++ {
		rand.Shuffle(len(nodes), func(i, k int) {
			nodes[i], nodes[k] = nodes[k], nodes[i]
		})

		sortByXOR(nodes, newNodeID())
	}
}

func TestDetermineFurthestIDWithinK(t *testing.T) {
	rt, cleanup := createRoutingTable(t, teststorj.NodeIDFromBytes([]byte{127, 255}))
	defer cleanup()

	cases := []struct {
		testID           string
		nodeID           []byte
		expectedFurthest []byte
	}{
		{testID: "xor 0",
			nodeID:           []byte{127, 255},
			expectedFurthest: []byte{127, 255},
		},
		{testID: "xor 240",
			nodeID:           []byte{143, 255},
			expectedFurthest: []byte{143, 255},
		},
		{testID: "xor 128",
			nodeID:           []byte{255, 255},
			expectedFurthest: []byte{143, 255},
		},
		{testID: "xor 192",
			nodeID:           []byte{191, 255},
			expectedFurthest: []byte{143, 255},
		},
		{testID: "xor 250",
			nodeID:           []byte{133, 255},
			expectedFurthest: []byte{133, 255},
		},
	}
	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			assert.NoError(t, rt.nodeBucketDB.Put(c.nodeID, []byte("")))
			nodes, err := rt.nodeBucketDB.List(nil, 0)
			assert.NoError(t, err)
			furthest, err := rt.determineFurthestIDWithinK(nodes)
			assert.NoError(t, err)
			assert.Equal(t, c.expectedFurthest, furthest)
		})
	}
}

func TestNodeIsWithinNearestK(t *testing.T) {
	rt, cleanup := createRoutingTable(t, teststorj.NodeIDFromBytes([]byte{127, 255}))
	defer cleanup()

	rt.bucketSize = 2
	cases := []struct {
		testID  string
		nodeID  []byte
		closest bool
	}{
		{testID: "A",
			nodeID:  []byte{127, 255},
			closest: true,
		},
		{testID: "B",
			nodeID:  []byte{143, 255},
			closest: true,
		},
		{testID: "C",
			nodeID:  []byte{255, 255},
			closest: true,
		},
		{testID: "D",
			nodeID:  []byte{191, 255},
			closest: true,
		},
		{testID: "E",
			nodeID:  []byte{133, 255},
			closest: false,
		},
	}
	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			result, err := rt.nodeIsWithinNearestK(c.nodeID)
			assert.NoError(t, err)
			assert.Equal(t, c.closest, result)
			assert.NoError(t, rt.nodeBucketDB.Put(c.nodeID, []byte("")))
		})
	}
}

func TestKadBucketContainsLocalNode(t *testing.T) {
	nodeIDABytes := []byte{183, 255} //[10110111, 1111111]

	rt, cleanup := createRoutingTable(t, teststorj.NodeIDFromBytes(nodeIDABytes))
	defer cleanup()

	kadIDA := storage.Key([]byte{255, 255})
	kadIDB := storage.Key([]byte{127, 255})
	now := time.Now()
	err := rt.createOrUpdateKBucket(kadIDB, now)
	assert.NoError(t, err)
	resultTrue, err := rt.kadBucketContainsLocalNode(kadIDA)
	assert.NoError(t, err)
	resultFalse, err := rt.kadBucketContainsLocalNode(kadIDB)
	assert.NoError(t, err)
	assert.True(t, resultTrue)
	assert.False(t, resultFalse)
}

func TestKadBucketHasRoom(t *testing.T) {
	nodeIDBytes := []byte{255, 255}
	kadIDA := storage.Key([]byte{255, 255})

	rt, cleanup := createRoutingTable(t, teststorj.NodeIDFromBytes(nodeIDBytes))
	defer cleanup()

	node2 := []byte{191, 255}
	node3 := []byte{127, 255}
	node4 := []byte{63, 255}
	node5 := []byte{159, 255}
	node6 := []byte{0, 127}
	resultA, err := rt.kadBucketHasRoom(kadIDA)
	assert.NoError(t, err)
	assert.True(t, resultA)
	assert.NoError(t, rt.nodeBucketDB.Put(node2, []byte("")))
	assert.NoError(t, rt.nodeBucketDB.Put(node3, []byte("")))
	assert.NoError(t, rt.nodeBucketDB.Put(node4, []byte("")))
	assert.NoError(t, rt.nodeBucketDB.Put(node5, []byte("")))
	assert.NoError(t, rt.nodeBucketDB.Put(node6, []byte("")))
	resultB, err := rt.kadBucketHasRoom(kadIDA)
	assert.NoError(t, err)
	assert.False(t, resultB)
}

func TestGetNodeIDsWithinKBucket(t *testing.T) {
	nodeIDABytes := []byte{183, 255} //[10110111, 1111111]

	rt, cleanup := createRoutingTable(t, teststorj.NodeIDFromBytes(nodeIDABytes))
	defer cleanup()

	kadIDA := storage.Key([]byte{255, 255})
	kadIDB := storage.Key([]byte{127, 255})
	now := time.Now()
	assert.NoError(t, rt.createOrUpdateKBucket(kadIDB, now))

	nodeIDBBytes := []byte{111, 255} //[01101111, 1111111]
	nodeIDCBytes := []byte{47, 255}  //[00101111, 1111111]

	assert.NoError(t, rt.nodeBucketDB.Put(nodeIDBBytes, []byte("")))
	assert.NoError(t, rt.nodeBucketDB.Put(nodeIDCBytes, []byte("")))

	cases := []struct {
		testID   string
		kadID    []byte
		expected storage.Keys
	}{
		{testID: "A",
			kadID:    kadIDA,
			expected: storage.Keys{nodeIDABytes},
		},
		{testID: "B",
			kadID:    kadIDB,
			expected: storage.Keys{nodeIDCBytes, nodeIDBBytes},
		},
	}
	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			n, err := rt.getNodeIDsWithinKBucket(c.kadID)
			assert.NoError(t, err)
			assert.Equal(t, c.expected, n)
		})
	}
}

func TestGetNodesFromIDs(t *testing.T) {
	nodeA := newNodeFromID(testIDA)
	nodeB := newNodeFromID(testIDB)
	nodeC := newNodeFromID(testIDC)

	a, err := proto.Marshal(nodeA)
	assert.NoError(t, err)
	b, err := proto.Marshal(nodeB)
	assert.NoError(t, err)
	c, err := proto.Marshal(nodeC)
	assert.NoError(t, err)

	rt, cleanup := createRoutingTable(t, testIDA)
	defer cleanup()

	assert.NoError(t, rt.nodeBucketDB.Put(testIDA.Bytes(), a))
	assert.NoError(t, rt.nodeBucketDB.Put(testIDB.Bytes(), b))
	assert.NoError(t, rt.nodeBucketDB.Put(testIDC.Bytes(), c))
	expected := []storage.Value{a, b, c}

	nodeIDs, err := rt.nodeBucketDB.List(nil, 0)
	assert.NoError(t, err)
	_, values, err := rt.getNodesFromIDs(nodeIDs)
	assert.NoError(t, err)
	assert.Equal(t, expected, values)
}

func TestUnmarshalNodes(t *testing.T) {
	nodeA := newNodeFromID(testIDA)
	nodeB := newNodeFromID(testIDB)
	nodeC := newNodeFromID(testIDC)

	a, err := proto.Marshal(nodeA)
	assert.NoError(t, err)
	b, err := proto.Marshal(nodeB)
	assert.NoError(t, err)
	c, err := proto.Marshal(nodeC)
	assert.NoError(t, err)

	rt, cleanup := createRoutingTable(t, testIDA)
	defer cleanup()

	assert.NoError(t, rt.nodeBucketDB.Put(testIDA.Bytes(), a))
	assert.NoError(t, rt.nodeBucketDB.Put(testIDB.Bytes(), b))
	assert.NoError(t, rt.nodeBucketDB.Put(testIDC.Bytes(), c))
	nodeIDs, err := rt.nodeBucketDB.List(nil, 0)
	assert.NoError(t, err)
	ids, values, err := rt.getNodesFromIDs(nodeIDs)
	assert.NoError(t, err)
	nodes, err := unmarshalNodes(ids, values)
	assert.NoError(t, err)
	expected := []storj.Node{nodeA, nodeB, nodeC}
	for i, v := range expected {
		assert.True(t, proto.Equal(v, nodes[i]))
	}
}

func TestGetUnmarshaledNodesFromBucket(t *testing.T) {
	bucketID := []byte{255, 255}
	nodeA := newNodeFromID(testIDA)
	nodeB := newNodeFromID(testIDB)
	nodeC := newNodeFromID(testIDC)

	rt, cleanup := createRoutingTable(t, testIDA)
	defer cleanup()

	var err error
	_, err = rt.addNode(nodeB)
	assert.NoError(t, err)
	_, err = rt.addNode(nodeC)
	assert.NoError(t, err)
	nodes, err := rt.getUnmarshaledNodesFromBucket(bucketID)
	expected := []storj.Node{nodeA, nodeB, nodeC}
	assert.NoError(t, err)
	for i, v := range expected {
		assert.True(t, proto.Equal(v, nodes[i]))
	}
}

func TestGetKBucketRange(t *testing.T) {
	rt, cleanup := createRoutingTable(t, nil)
	defer cleanup()
	idABytes := []byte{255, 255}
	idBBytes := []byte{127, 255}
	idCBytes := []byte{63, 255}
	assert.NoError(t, rt.kadBucketDB.Put(idABytes, []byte("")))
	assert.NoError(t, rt.kadBucketDB.Put(idBBytes, []byte("")))
	assert.NoError(t, rt.kadBucketDB.Put(idCBytes, []byte("")))
	cases := []struct {
		testID   string
		id       []byte
		expected storage.Keys
	}{
		{testID: "A",
			id:       idABytes,
			expected: storage.Keys{idBBytes, idABytes},
		},
		{testID: "B",
			id:       idBBytes,
			expected: storage.Keys{idCBytes, idBBytes}},
		{testID: "C",
			id:       idCBytes,
			expected: storage.Keys{rt.createZeroAsStorageKey(), idCBytes},
		},
	}
	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			ep, err := rt.getKBucketRange(c.id)
			assert.NoError(t, err)
			assert.Equal(t, c.expected, ep)
		})
	}
}

func TestCreateFirstBucketID(t *testing.T) {
	rt, cleanup := createRoutingTable(t, nil)
	defer cleanup()
	x := rt.createFirstBucketID()
	expected := []byte{255, 255}
	assert.Equal(t, x, expected)
}

func TestCreateZeroAsStorageKey(t *testing.T) {
	rt, cleanup := createRoutingTable(t, nil)
	defer cleanup()
	zero := rt.createZeroAsStorageKey()
	expected := []byte{0, 0}
	assert.Equal(t, zero, storage.Key(expected))
}

func TestDetermineLeafDepth(t *testing.T) {
	rt, cleanup := createRoutingTable(t, nil)
	defer cleanup()
	idA := []byte{255, 255}
	idB := []byte{127, 255}
	idC := []byte{63, 255}

	cases := []struct {
		testID  string
		id      []byte
		depth   int
		addNode func()
	}{
		{testID: "A",
			id:    idA,
			depth: 0,
			addNode: func() {
				e := rt.kadBucketDB.Put(idA, []byte(""))
				assert.NoError(t, e)
			},
		},
		{testID: "B",
			id:    idB,
			depth: 1,
			addNode: func() {
				e := rt.kadBucketDB.Put(idB, []byte(""))
				assert.NoError(t, e)
			},
		},
		{testID: "C",
			id:    idA,
			depth: 1,
			addNode: func() {
				e := rt.kadBucketDB.Put(idC, []byte(""))
				assert.NoError(t, e)
			},
		},
		{testID: "D",
			id:      idB,
			depth:   2,
			addNode: func() {},
		},
		{testID: "E",
			id:      idC,
			depth:   2,
			addNode: func() {},
		},
	}
	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			c.addNode()
			d, err := rt.determineLeafDepth(c.id)
			assert.NoError(t, err)
			assert.Equal(t, c.depth, d)
		})
	}
}

func TestDetermineDifferingBitIndex(t *testing.T) {
	rt, cleanup := createRoutingTable(t, nil)
	defer cleanup()
	cases := []struct {
		testID   string
		bucketID []byte
		key      []byte
		expected int
		err      *errs.Class
	}{
		{testID: "A",
			bucketID: []byte{191, 255},
			key:      []byte{255, 255},
			expected: 1,
			err:      nil,
		},
		{testID: "B",
			bucketID: []byte{255, 255},
			key:      []byte{191, 255},
			expected: 1,
			err:      nil,
		},
		{testID: "C",
			bucketID: []byte{95, 255},
			key:      []byte{127, 255},
			expected: 2,
			err:      nil,
		},
		{testID: "D",
			bucketID: []byte{95, 255},
			key:      []byte{79, 255},
			expected: 3,
			err:      nil,
		},
		{testID: "E",
			bucketID: []byte{95, 255},
			key:      []byte{63, 255},
			expected: 2,
			err:      nil,
		},
		{testID: "F",
			bucketID: []byte{95, 255},
			key:      []byte{79, 255},
			expected: 3,
			err:      nil,
		},
		{testID: "G",
			bucketID: []byte{255, 255},
			key:      []byte{255, 255},
			expected: -2,
			err:      &RoutingErr,
		},
		{testID: "H",
			bucketID: []byte{255, 255},
			key:      []byte{0, 0},
			expected: -1,
			err:      nil,
		},
		{testID: "I",
			bucketID: []byte{127, 255},
			key:      []byte{0, 0},
			expected: 0,
			err:      nil,
		},
		{testID: "J",
			bucketID: []byte{63, 255},
			key:      []byte{0, 0},
			expected: 1,
			err:      nil,
		},
		{testID: "K",
			bucketID: []byte{31, 255},
			key:      []byte{0, 0},
			expected: 2,
			err:      nil,
		},
		{testID: "L",
			bucketID: []byte{95, 255},
			key:      []byte{63, 255},
			expected: 2,
			err:      nil,
		},
	}

	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			diff, err := rt.determineDifferingBitIndex(c.bucketID, c.key)
			assertErrClass(t, c.err, err)
			assert.Equal(t, c.expected, diff)
		})
	}
}

func TestSplitBucket(t *testing.T) {
	rt, cleanup := createRoutingTable(t, nil)
	defer cleanup()
	cases := []struct {
		testID string
		idA    []byte
		idB    []byte
		depth  int
	}{
		{testID: "A: [11111111, 11111111] -> [10111111, 11111111]",
			idA:   []byte{255, 255},
			idB:   []byte{191, 255},
			depth: 1,
		},
		{testID: "B: [10111111, 11111111] -> [10011111, 11111111]",
			idA:   []byte{191, 255},
			idB:   []byte{159, 255},
			depth: 2,
		},
		{testID: "C: [01111111, 11111111] -> [00111111, 11111111]",
			idA:   []byte{127, 255},
			idB:   []byte{63, 255},
			depth: 1,
		},
		{testID: "D: [00000000, 11111111] -> [00000000, 01111111]",
			idA:   []byte{0, 255},
			idB:   []byte{0, 127},
			depth: 8,
		},
		{testID: "E: [01011111, 11111111] -> [01010111, 11111111]",
			idA:   []byte{95, 255},
			idB:   []byte{87, 255},
			depth: 4,
		},
		{testID: "F: [01011111, 11111111] -> [01001111, 11111111]",
			idA:   []byte{95, 255},
			idB:   []byte{79, 255},
			depth: 3,
		},
	}
	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			newID := rt.splitBucket(c.idA, c.depth)
			assert.Equal(t, c.idB, newID)
		})
	}
}

func assertErrClass(t *testing.T, class *errs.Class, err error) {
	t.Helper()
	if class != nil {
		assert.True(t, class.Has(err))
	} else {
		assert.NoError(t, err)
	}
}
