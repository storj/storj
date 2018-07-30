// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package kademlia

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	proto "storj.io/storj/protos/overlay"
	"storj.io/storj/storage"
)

func tempfile(fileName string) string {
	f, err := ioutil.TempFile("", fileName)
	if err != nil {
		panic(err)
	}
	f.Close()
	err = os.Remove(f.Name())
	if err != nil {
		panic(err)
	}
	return f.Name()
}

func createRT() *RoutingTable {
	localNodeID, _ := newID()
	localNode := proto.Node{Id: string(localNodeID)}
	rt, _ := NewRoutingTable(&localNode, tempfile("Kadbucket"), tempfile("Nodebucket"), 16, 6)
	return rt
}

func mockNodes(id string) *proto.Node {
	var node proto.Node
	node.Id = id
	return &node
}

func TestAddNode(t *testing.T) {
	rt := createRT()
	//add local node
	rt.self = mockNodes("OO") //[79, 79] or [01001111, 01001111]
	localNode := rt.self
	err := rt.addNode(localNode)
	assert.NoError(t, err)
	bucket, err := rt.kadBucketDB.Get(storage.Key([]byte{255, 255}))
	assert.NoError(t, err)
	assert.NotNil(t, bucket)
	//add node to unfilled kbucket
	node1 := mockNodes("PO") //[80, 79] or [01010000, 01001111]
	err = rt.addNode(node1)
	assert.NoError(t, err)
	kadKeys, err := rt.kadBucketDB.List(nil, 0)
	assert.NoError(t, err)
	nodeKeys, err := rt.nodeBucketDB.List(nil, 0)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(kadKeys))
	assert.Equal(t, 2, len(nodeKeys))

	//add node to full kbucket and split
	node2 := mockNodes("NO") //[78, 79] or [01001110, 01001111]
	err = rt.addNode(node2)
	assert.NoError(t, err)

	node3 := mockNodes("MO") //[77, 79] or [01001101, 01001111]
	err = rt.addNode(node3)
	assert.NoError(t, err)

	node4 := mockNodes("LO") //[76, 79] or [01001100, 01001111]
	err = rt.addNode(node4)
	assert.NoError(t, err)

	node5 := mockNodes("QO") //[81, 79] or [01010001, 01001111]
	err = rt.addNode(node5)
	assert.NoError(t, err)

	kadKeys, err = rt.kadBucketDB.List(nil, 0)
	assert.NoError(t, err)
	nodeKeys, err = rt.nodeBucketDB.List(nil, 0)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(kadKeys))
	assert.Equal(t, 6, len(nodeKeys))

	//splitting here
	node6 := mockNodes("SO")
	err = rt.addNode(node6)
	assert.NoError(t, err)

	kadKeys, err = rt.kadBucketDB.List(nil, 0)
	assert.NoError(t, err)
	nodeKeys, err = rt.nodeBucketDB.List(nil, 0)
	assert.NoError(t, err)
	assert.Equal(t, 5, len(kadKeys))
	assert.Equal(t, 7, len(nodeKeys))

	// //check how many keys in each bucket
	a, err := rt.getNodeIDsWithinKBucket(kadKeys[0])
	assert.NoError(t, err)
	assert.Equal(t, 0, len(a))
	b, err := rt.getNodeIDsWithinKBucket(kadKeys[1])
	assert.NoError(t, err)
	assert.Equal(t, 4, len(b))
	c, err := rt.getNodeIDsWithinKBucket(kadKeys[2])
	assert.NoError(t, err)
	assert.Equal(t, 3, len(c))
	d, err := rt.getNodeIDsWithinKBucket(kadKeys[3])
	assert.NoError(t, err)
	assert.Equal(t, 0, len(d))
	e, err := rt.getNodeIDsWithinKBucket(kadKeys[4])
	assert.NoError(t, err)
	assert.Equal(t, 0, len(e))

	//add node to full kbucket and drop
	node7 := mockNodes("?O")
	err = rt.addNode(node7)
	assert.NoError(t, err)

	node8 := mockNodes(">O")
	err = rt.addNode(node8)
	assert.NoError(t, err)

	node9 := mockNodes("=O")
	err = rt.addNode(node9)
	assert.NoError(t, err)

	node10 := mockNodes(";O")
	err = rt.addNode(node10)
	assert.NoError(t, err)

	node11 := mockNodes(":O")
	err = rt.addNode(node11)
	assert.NoError(t, err)

	node12 := mockNodes("9O")
	err = rt.addNode(node12)
	assert.NoError(t, err)

	kadKeys, err = rt.kadBucketDB.List(nil, 0)
	assert.NoError(t, err)
	nodeKeys, err = rt.nodeBucketDB.List(nil, 0)
	assert.NoError(t, err)
	assert.Equal(t, 5, len(kadKeys))
	assert.Equal(t, 13, len(nodeKeys))

	a, err = rt.getNodeIDsWithinKBucket(kadKeys[0])
	assert.NoError(t, err)
	assert.Equal(t, 6, len(a))

	//should drop
	node13 := mockNodes("8O")
	err = rt.addNode(node13)
	assert.NoError(t, err)

	kadKeys, err = rt.kadBucketDB.List(nil, 0)
	assert.NoError(t, err)
	nodeKeys, err = rt.nodeBucketDB.List(nil, 0)
	assert.NoError(t, err)
	assert.Equal(t, 5, len(kadKeys))
	assert.Equal(t, 13, len(nodeKeys))
	a, err = rt.getNodeIDsWithinKBucket(kadKeys[0])
	assert.NoError(t, err)
	assert.Equal(t, 6, len(a))

	//add node to highly unbalanced tree
	//adding to bucket 1
	node14 := mockNodes("KO") //75
	err = rt.addNode(node14)
	assert.NoError(t, err)
	node15 := mockNodes("JO") //74
	err = rt.addNode(node15)
	assert.NoError(t, err)

	//adding to bucket 2
	node16 := mockNodes("]O") //93
	err = rt.addNode(node16)
	assert.NoError(t, err)
	node17 := mockNodes("^O") //94
	err = rt.addNode(node17)
	assert.NoError(t, err)
	node18 := mockNodes("_O") //95
	err = rt.addNode(node18)
	assert.NoError(t, err)

	b, err = rt.getNodeIDsWithinKBucket(kadKeys[1])
	assert.NoError(t, err)
	assert.Equal(t, 6, len(b))
	c, err = rt.getNodeIDsWithinKBucket(kadKeys[2])
	assert.NoError(t, err)
	assert.Equal(t, 6, len(c))
	kadKeys, err = rt.kadBucketDB.List(nil, 0)
	assert.NoError(t, err)
	nodeKeys, err = rt.nodeBucketDB.List(nil, 0)
	assert.NoError(t, err)
	assert.Equal(t, 5, len(kadKeys))
	assert.Equal(t, 18, len(nodeKeys))

	//split bucket 2
	node19 := mockNodes("@O")
	err = rt.addNode(node19)
	assert.NoError(t, err)
	kadKeys, err = rt.kadBucketDB.List(nil, 0)
	assert.NoError(t, err)
	nodeKeys, err = rt.nodeBucketDB.List(nil, 0)
	assert.NoError(t, err)

	assert.Equal(t, 6, len(kadKeys))
	assert.Equal(t, 19, len(nodeKeys))
}

func TestCreateOrUpdateKBucket(t *testing.T) {
	rt := createRT()
	id := []byte{255, 255}
	err := rt.createOrUpdateKBucket(storage.Key(id), time.Now())
	assert.NoError(t, err)
	val, e := rt.kadBucketDB.Get(storage.Key(id))
	assert.NotNil(t, val)
	assert.NoError(t, e)

}

func TestGetKBucketID(t *testing.T) {
	rt := createRT()

	kadIDA := storage.Key([]byte{255, 255})
	kadIDB := storage.Key([]byte{127, 255})
	kadIDC := storage.Key([]byte{63, 255})
	now := time.Now()
	rt.createOrUpdateKBucket(kadIDA, now)
	rt.createOrUpdateKBucket(kadIDB, now)
	rt.createOrUpdateKBucket(kadIDC, now)
	nodeIDA := []byte{183, 255} //[10110111, 1111111]
	nodeIDB := []byte{111, 255} //[01101111, 1111111]
	nodeIDC := []byte{47, 255}  //[00101111, 1111111]

	keyA, _ := rt.getKBucketID(nodeIDA)
	assert.Equal(t, kadIDA, keyA)

	keyB, _ := rt.getKBucketID(nodeIDB)
	assert.Equal(t, kadIDB, keyB)

	keyC, _ := rt.getKBucketID(nodeIDC)
	assert.Equal(t, kadIDC, keyC)
}

func TestXorTwoIds(t *testing.T) {
	x := xorTwoIds([]byte{191}, []byte{159})
	assert.Equal(t, []byte{32}, x) //00100000
}

func TestSortByXOR(t *testing.T) {
	rt := createRT()
	node1 := []byte{127, 255} //xor 0
	rt.self.Id = string(node1)
	rt.nodeBucketDB.Put(node1, []byte(""))
	node2 := []byte{143, 255} //xor 240
	rt.nodeBucketDB.Put(node2, []byte(""))
	node3 := []byte{255, 255} //xor 128
	rt.nodeBucketDB.Put(node3, []byte(""))
	node4 := []byte{191, 255} //xor 192
	rt.nodeBucketDB.Put(node4, []byte(""))
	node5 := []byte{133, 255} //xor 250
	rt.nodeBucketDB.Put(node5, []byte(""))
	nodes, err := rt.nodeBucketDB.List(nil, 0)
	assert.NoError(t, err)
	expectedNodes := storage.Keys{node1, node5, node2, node4, node3}
	assert.Equal(t, expectedNodes, nodes)
	sortedNodes := rt.sortByXOR(nodes)
	expectedSorted := storage.Keys{node1, node3, node4, node2, node5}
	assert.Equal(t, expectedSorted, sortedNodes)
	nodes, err = rt.nodeBucketDB.List(nil, 0)
	assert.NoError(t, err)
	assert.Equal(t, expectedNodes, nodes)
}

func TestDetermineFurthestIDWithinK(t *testing.T) {
	rt := createRT()
	node1 := []byte{127, 255} //xor 0
	rt.self.Id = string(node1)
	rt.nodeBucketDB.Put(node1, []byte(""))
	expectedFurthest := node1
	nodes, err := rt.nodeBucketDB.List(nil, 0)
	assert.NoError(t, err)
	furthest, err := rt.determineFurthestIDWithinK(nodes)
	assert.Equal(t, expectedFurthest, furthest)

	node2 := []byte{143, 255} //xor 240
	rt.nodeBucketDB.Put(node2, []byte(""))
	expectedFurthest = node2
	nodes, err = rt.nodeBucketDB.List(nil, 0)
	assert.NoError(t, err)
	furthest, err = rt.determineFurthestIDWithinK(nodes)
	assert.Equal(t, expectedFurthest, furthest)

	node3 := []byte{255, 255} //xor 128
	rt.nodeBucketDB.Put(node3, []byte(""))
	expectedFurthest = node2
	nodes, err = rt.nodeBucketDB.List(nil, 0)
	assert.NoError(t, err)
	furthest, err = rt.determineFurthestIDWithinK(nodes)
	assert.Equal(t, expectedFurthest, furthest)

	node4 := []byte{191, 255} //xor 192
	rt.nodeBucketDB.Put(node4, []byte(""))
	expectedFurthest = node2
	nodes, err = rt.nodeBucketDB.List(nil, 0)
	assert.NoError(t, err)
	furthest, err = rt.determineFurthestIDWithinK(nodes)
	assert.Equal(t, expectedFurthest, furthest)

	node5 := []byte{133, 255} //xor 250
	rt.nodeBucketDB.Put(node5, []byte(""))
	expectedFurthest = node5
	nodes, err = rt.nodeBucketDB.List(nil, 0)
	assert.NoError(t, err)
	furthest, err = rt.determineFurthestIDWithinK(nodes)
	assert.Equal(t, expectedFurthest, furthest)
}

func TestNodeIsWithinNearestK(t *testing.T) {
	rt := createRT()
	rt.bucketSize = 2

	selfNode := []byte{127, 255}
	rt.self.Id = string(selfNode)
	rt.nodeBucketDB.Put(selfNode, []byte(""))
	expectTrue, err := rt.nodeIsWithinNearestK(selfNode)
	assert.NoError(t, err)
	assert.True(t, expectTrue)

	furthestNode := []byte{143, 255}
	expectTrue, err = rt.nodeIsWithinNearestK(furthestNode)
	assert.NoError(t, err)
	assert.True(t, expectTrue)
	rt.nodeBucketDB.Put(furthestNode, []byte(""))

	node1 := []byte{255, 255}
	expectTrue, err = rt.nodeIsWithinNearestK(node1)
	assert.NoError(t, err)
	assert.True(t, expectTrue)
	rt.nodeBucketDB.Put(node1, []byte(""))

	node2 := []byte{191, 255}
	expectTrue, err = rt.nodeIsWithinNearestK(node2)
	assert.NoError(t, err)
	assert.True(t, expectTrue)
	rt.nodeBucketDB.Put(node1, []byte(""))

	node3 := []byte{133, 255}
	expectFalse, err := rt.nodeIsWithinNearestK(node3)
	assert.NoError(t, err)
	assert.False(t, expectFalse)
}

func TestKadBucketContainsLocalNode(t *testing.T) {
	rt := createRT()
	kadIDA := storage.Key([]byte{255, 255})
	kadIDB := storage.Key([]byte{127, 255})
	now := time.Now()
	err := rt.createOrUpdateKBucket(kadIDA, now)
	assert.NoError(t, err)
	err = rt.createOrUpdateKBucket(kadIDB, now)
	assert.NoError(t, err)
	nodeIDA := []byte{183, 255} //[10110111, 1111111]
	err = rt.nodeBucketDB.Put(nodeIDA, []byte(""))
	assert.NoError(t, err)
	rt.self.Id = string(nodeIDA)
	resultTrue, err := rt.kadBucketContainsLocalNode(kadIDA)
	assert.NoError(t, err)
	resultFalse, err := rt.kadBucketContainsLocalNode(kadIDB)
	assert.NoError(t, err)
	assert.True(t, resultTrue)
	assert.False(t, resultFalse)
}

func TestKadBucketHasRoom(t *testing.T) {
	//valid when kad bucket has >0 nodes in it due to ../storage/boltdb/client.go:99
	rt := createRT()
	kadIDA := storage.Key([]byte{255, 255})
	now := time.Now()
	rt.createOrUpdateKBucket(kadIDA, now)
	node1 := []byte{255, 255}
	node2 := []byte{191, 255}
	node3 := []byte{127, 255}
	node4 := []byte{63, 255}
	node5 := []byte{159, 255}
	node6 := []byte{0, 127}
	rt.nodeBucketDB.Put(node1, []byte(""))
	resultA, err := rt.kadBucketHasRoom(kadIDA)
	assert.NoError(t, err)
	assert.True(t, resultA)
	rt.nodeBucketDB.Put(node2, []byte(""))
	rt.nodeBucketDB.Put(node3, []byte(""))
	rt.nodeBucketDB.Put(node4, []byte(""))
	rt.nodeBucketDB.Put(node5, []byte(""))
	rt.nodeBucketDB.Put(node6, []byte(""))
	resultB, err := rt.kadBucketHasRoom(kadIDA)
	assert.NoError(t, err)
	assert.False(t, resultB)
}

func TestGetNodeIDsWithinKBucket(t *testing.T) {
	rt := createRT()
	kadIDA := storage.Key([]byte{255, 255})
	kadIDB := storage.Key([]byte{127, 255})
	now := time.Now()
	rt.createOrUpdateKBucket(kadIDA, now)
	rt.createOrUpdateKBucket(kadIDB, now)

	nodeIDA := []byte{183, 255} //[10110111, 1111111]
	nodeIDB := []byte{111, 255} //[01101111, 1111111]
	nodeIDC := []byte{47, 255}  //[00101111, 1111111]

	rt.nodeBucketDB.Put(nodeIDA, []byte(""))
	rt.nodeBucketDB.Put(nodeIDB, []byte(""))
	rt.nodeBucketDB.Put(nodeIDC, []byte(""))

	expectedA := storage.Keys{nodeIDA}
	expectedB := storage.Keys{nodeIDC, nodeIDB}

	A, err := rt.getNodeIDsWithinKBucket(kadIDA)
	assert.NoError(t, err)
	B, err := rt.getNodeIDsWithinKBucket(kadIDB)
	assert.NoError(t, err)

	assert.Equal(t, expectedA, A)
	assert.Equal(t, expectedB, B)
}

func TestGetKBucketRange(t *testing.T) {
	rt := createRT()
	idA := []byte{255, 255}
	idB := []byte{127, 255}
	idC := []byte{63, 255}
	rt.kadBucketDB.Put(idA, []byte(""))
	rt.kadBucketDB.Put(idB, []byte(""))
	rt.kadBucketDB.Put(idC, []byte(""))
	expectedA := storage.Keys{idB, idA}
	expectedB := storage.Keys{idC, idB}
	expectedC := storage.Keys{rt.createZeroAsStorageKey(), idC}

	endpointsA, err := rt.getKBucketRange(idA)
	assert.NoError(t, err)
	endpointsB, err := rt.getKBucketRange(idB)
	assert.NoError(t, err)
	endpointsC, err := rt.getKBucketRange(idC)
	assert.NoError(t, err)
	assert.Equal(t, expectedA, endpointsA)
	assert.Equal(t, expectedB, endpointsB)
	assert.Equal(t, expectedC, endpointsC)
}

func TestCreateFirstBucketID(t *testing.T) {
	rt := createRT()
	x := rt.createFirstBucketID()
	expected := []byte{255, 255}
	assert.Equal(t, x, expected)
}

func TestCreateZeroAsStorageKey(t *testing.T) {
	rt := createRT()
	zero := rt.createZeroAsStorageKey()
	expected := []byte{0, 0}
	assert.Equal(t, zero, storage.Key(expected))
}

func TestDetermineLeafDepth(t *testing.T) {
	rt := createRT()
	idA := []byte{255, 255}
	idB := []byte{127, 255}
	idC := []byte{63, 255}

	err := rt.kadBucketDB.Put(idA, []byte(""))
	assert.NoError(t, err)

	first, err := rt.determineLeafDepth(idA)
	assert.NoError(t, err)
	assert.Equal(t, 0, first)

	err = rt.kadBucketDB.Put(idB, []byte(""))
	assert.NoError(t, err)

	second, err := rt.determineLeafDepth(idB)
	assert.NoError(t, err)
	assert.Equal(t, 1, second)

	err = rt.kadBucketDB.Put(idC, []byte(""))
	assert.NoError(t, err)

	one, err := rt.determineLeafDepth(idA)
	assert.NoError(t, err)
	assert.Equal(t, 1, one)

	two, err := rt.determineLeafDepth(idB)
	assert.NoError(t, err)
	assert.Equal(t, 2, two)

	alsoTwo, err := rt.determineLeafDepth(idC)
	assert.NoError(t, err)
	assert.Equal(t, 2, alsoTwo)

}

func TestDetermineDifferingBitIndex(t *testing.T) {
	rt := createRT()
	diff, err := rt.determineDifferingBitIndex([]byte{191, 255}, []byte{255, 255})
	assert.NoError(t, err)
	assert.Equal(t, 1, diff)

	diff, err = rt.determineDifferingBitIndex([]byte{255, 255}, []byte{191, 255})
	assert.NoError(t, err)
	assert.Equal(t, 1, diff)

	diff, err = rt.determineDifferingBitIndex([]byte{95, 255}, []byte{127, 255})
	assert.NoError(t, err)
	assert.Equal(t, 2, diff)

	diff, err = rt.determineDifferingBitIndex([]byte{95, 255}, []byte{79, 255})
	assert.NoError(t, err)
	assert.Equal(t, 3, diff)

	diff, err = rt.determineDifferingBitIndex([]byte{95, 255}, []byte{63, 255})
	assert.NoError(t, err)
	assert.Equal(t, 2, diff)

	diff, err = rt.determineDifferingBitIndex([]byte{95, 255}, []byte{79, 255})
	assert.NoError(t, err)
	assert.Equal(t, 3, diff)

	diff, err = rt.determineDifferingBitIndex([]byte{255, 255}, []byte{255, 255})
	assert.Error(t, err)
	assert.Equal(t, -2, diff)

	diff, err = rt.determineDifferingBitIndex([]byte{255, 255}, []byte{0, 0})
	assert.NoError(t, err)
	assert.Equal(t, -1, diff)

	diff, err = rt.determineDifferingBitIndex([]byte{127, 255}, []byte{0, 0})
	assert.NoError(t, err)
	assert.Equal(t, 0, diff)

	diff, err = rt.determineDifferingBitIndex([]byte{63, 255}, []byte{0, 0})
	assert.NoError(t, err)
	assert.Equal(t, 1, diff)

	diff, err = rt.determineDifferingBitIndex([]byte{31, 255}, []byte{0, 0})
	assert.NoError(t, err)
	assert.Equal(t, 2, diff)

	diff, err = rt.determineDifferingBitIndex([]byte{95, 255}, []byte{63, 255})
	assert.NoError(t, err)
	assert.Equal(t, 2, diff)

}

func TestSplitBucket(t *testing.T) {
	rt := createRT()
	id1 := []byte{255, 255}
	id2 := []byte{191, 255}
	id3 := []byte{127, 255}
	id4 := []byte{63, 255}
	id5 := []byte{159, 255}
	id6 := []byte{0, 127}
	id7 := []byte{0, 255}
	id8 := []byte{95, 255}
	id9 := []byte{87, 255}

	newID1 := rt.splitBucket(id1, 1) //[11111111, 11111111] -> [10111111, 11111111]
	assert.Equal(t, id2, newID1)

	newID2 := rt.splitBucket(id2, 2) //[10111111, 11111111] -> [10011111, 11111111]
	assert.Equal(t, id5, newID2)

	newID3 := rt.splitBucket(id3, 1) //[01111111, 11111111] -> [00111111, 11111111]
	assert.Equal(t, id4, newID3)

	newID4 := rt.splitBucket(id7, 8) //[00000000, 11111111] -> [00000000, 01111111]
	assert.Equal(t, id6, newID4)

	newID5 := rt.splitBucket(id8, 4) //[01011111, 11111111] -> [01010111, 11111111]
	assert.Equal(t, id9, newID5)

	newID6 := rt.splitBucket(id8, 3)
	assert.Equal(t, []byte{79, 255}, newID6)
}
