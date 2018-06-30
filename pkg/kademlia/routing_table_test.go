// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package kademlia
import (
	"github.com/stretchr/testify/assert"
	"testing"
	"os"
	"io/ioutil"
	"fmt"
	proto "storj.io/storj/protos/overlay"
	"storj.io/storj/storage"
)

//Interface tests ----------------------------------------------------
func TestLocal(t *testing.T) {}
func TestK(t *testing.T) {}

func TestCacheSize(t *testing.T) {}

func TestGetBucket(t *testing.T) {}
func TestGetBuckets(t *testing.T) {}

func TestFindNear(t *testing.T) {}

func TestConnectionSuccess(t *testing.T) {}

func TestConnectionFailed(t *testing.T) {}

func TestSetBucketTimestamp(t *testing.T) {}
func TestGetBucketTimestamp(t *testing.T) {}

//Helper tests ----------------------------------------------------
func TestAddNode(t *testing.T) {
	rt := createRT()
	//add local node
	rt.Self = mockNodes("OO") //[79, 79] or [01001111, 01001111]
	localNode := rt.Self
	rt.addNode(localNode)
	
	//add node to unfilled kbucket  
	node1 := mockNodes("PO") //[80, 79] or [01010000, 01001111]
	rt.addNode(node1)
	kadKeys, _ := (*rt.kadBucketDB).List(nil, 0)
	nodeKeys, _ := (*rt.nodeBucketDB).List(nil, 0)
	assert.Equal(t, 1, len(kadKeys))
	assert.Equal(t, 2, len(nodeKeys))

	//add node to full kbucket and split
	node2 := mockNodes("NO") //[78, 79] or [01001110, 01001111]
	rt.addNode(node2)
	node3 := mockNodes("MO") //[77, 79] or [01001101, 01001111]
	rt.addNode(node3)
	node4 := mockNodes("LO") //[76, 79] or [01001100, 01001111]
	rt.addNode(node4)
	node5 := mockNodes("QO") //[81, 79] or [01010001, 01001111]
	rt.addNode(node5)
	kadKeys, _ = (*rt.kadBucketDB).List(nil, 0)
	nodeKeys, _ = (*rt.nodeBucketDB).List(nil, 0)
	assert.Equal(t, 1, len(kadKeys))
	assert.Equal(t, 6, len(nodeKeys))
	
	//splitting here
	node6 := mockNodes("SO") //[83, 79][01010011, 01001111]
	rt.addNode(node6)
	kadKeys, _ = (*rt.kadBucketDB).List(nil, 0)
	nodeKeys, _ = (*rt.nodeBucketDB).List(nil, 0)
	assert.Equal(t, 5, len(kadKeys)) 
	assert.Equal(t, 7, len(nodeKeys))
	
	//check how many keys in each bucket
	assert.Equal(t, 0, len(rt.getNodeIDsWithinKBucket(kadKeys[0])))
	assert.Equal(t, 4, len(rt.getNodeIDsWithinKBucket(kadKeys[1])))
	assert.Equal(t, 3, len(rt.getNodeIDsWithinKBucket(kadKeys[2])))
	assert.Equal(t, 0, len(rt.getNodeIDsWithinKBucket(kadKeys[3])))
	assert.Equal(t, 0, len(rt.getNodeIDsWithinKBucket(kadKeys[4])))

	//add node to full kbucket and drop
	node7 := mockNodes("?O")
	rt.addNode(node7)
	node8 := mockNodes(">O")
	rt.addNode(node8)
	node9 := mockNodes("=O")
	rt.addNode(node9)
	node10 := mockNodes(";O")
	rt.addNode(node10)
	node11 := mockNodes(":O")
	rt.addNode(node11)
	node12 := mockNodes("9O")
	rt.addNode(node12)
	kadKeys, _ = (*rt.kadBucketDB).List(nil, 0)
	nodeKeys, _ = (*rt.nodeBucketDB).List(nil, 0)
	assert.Equal(t, 5, len(kadKeys)) 
	assert.Equal(t, 13, len(nodeKeys))
	assert.Equal(t, 6, len(rt.getNodeIDsWithinKBucket(kadKeys[0])))
	//should drop
	node13 :=mockNodes("8O")
	rt.addNode(node13)
	kadKeys, _ = (*rt.kadBucketDB).List(nil, 0)
	nodeKeys, _ = (*rt.nodeBucketDB).List(nil, 0)
	assert.Equal(t, 5, len(kadKeys)) 
	assert.Equal(t, 13, len(nodeKeys))
	assert.Equal(t, 6, len(rt.getNodeIDsWithinKBucket(kadKeys[0])))

	//add node to highly unbalanced tree
	//adding to bucket 1
	node14 := mockNodes("KO") //75
	rt.addNode(node14)
	node15 := mockNodes("JO") //74
	rt.addNode(node15)

	//adding to bucket 2
	node16 := mockNodes("]O") //93
	rt.addNode(node16)
	node17 := mockNodes("^O") //94
	rt.addNode(node17)
	node18 := mockNodes("_O") //95
	rt.addNode(node18)
	assert.Equal(t, 6, len(rt.getNodeIDsWithinKBucket(kadKeys[1])))
	assert.Equal(t, 6, len(rt.getNodeIDsWithinKBucket(kadKeys[2])))

	//split bucket 2
	fmt.Print("Attempting Split Bucket #2")
	//node19 :=mockNodes("RO") //82
	//rt.addNode(node19)
	//kadKeys, _ = (*rt.kadBucketDB).List(nil, 0)
	//nodeKeys, _ = (*rt.nodeBucketDB).List(nil, 0)

	// fmt.Printf("depth 0 %v \n" ,rt.determineLeafDepth(kadKeys[0]))
	// fmt.Printf("depth 1 %v \n" ,rt.determineLeafDepth(kadKeys[1]))
	//fmt.Printf("depth 2 %v \n" ,rt.determineLeafDepth(kadKeys[2])) //returning 3, should be 4
	// fmt.Printf("depth 3 %v \n" ,rt.determineLeafDepth(kadKeys[3])) 
	// fmt.Printf("depth 4 %v \n" ,rt.determineLeafDepth(kadKeys[4]))

	// fmt.Printf("key 0 %v/%v: ", kadKeys[0], rt.getNodeIDsWithinKBucket(kadKeys[0]))
	// fmt.Printf("key 1 %v/ %v: ",kadKeys[1],rt.getNodeIDsWithinKBucket(kadKeys[1]))
	// fmt.Printf("key 2 %v/ %v: ",kadKeys[2],rt.getNodeIDsWithinKBucket(kadKeys[2])) 
	// fmt.Printf("key 3 %v/ %v: ",kadKeys[3],rt.getNodeIDsWithinKBucket(kadKeys[3]))
	// fmt.Printf("key 4 %v/ %v: ",kadKeys[4],rt.getNodeIDsWithinKBucket(kadKeys[4]))
	
	//getting depth of 3, should be 4
	//comparing 127 to 95 rather than 95 to 70
	//assert.Equal(t, 6, len(kadKeys)) 
	//assert.Equal(t, 19, len(nodeKeys))


}


func TestCreateOrUpdateKBucket(t *testing.T) {
	rt := createRT()
	id := id1()
	err := rt.createOrUpdateKBucket(storage.Key(id))
	assert.NoError(t, err)
	val, e := (*rt.kadBucketDB).Get(storage.Key(id))
	assert.NotNil(t, val)
	assert.NoError(t, e)

}

func TestGetKBucketID(t *testing.T) {
	rt := createRT()
	kadIDA := storage.Key(id1())
	kadIDB := storage.Key(id3())
	kadIDC := storage.Key(id4())
	rt.createOrUpdateKBucket(kadIDA)
	rt.createOrUpdateKBucket(kadIDB)
	rt.createOrUpdateKBucket(kadIDC)

	var nodeIDA []byte //[10110111, 1111111]
	nodeIDA = append(nodeIDA, byte(183))
	nodeIDA = append(nodeIDA, byte(255))

	var nodeIDB []byte //[01101111, 1111111]
	nodeIDB = append(nodeIDB, byte(111))
	nodeIDB = append(nodeIDB, byte(255))

	var nodeIDC []byte //[00101111, 1111111]
	nodeIDC = append(nodeIDC, byte(47))
	nodeIDC = append(nodeIDC, byte(255))

	
	keyA, _ := rt.getKBucketID(nodeIDA)
	assert.Equal(t, kadIDA, keyA)
	
	keyB, _ := rt.getKBucketID(nodeIDB)
	assert.Equal(t, kadIDB, keyB)

	keyC, _ := rt.getKBucketID(nodeIDC)
	assert.Equal(t, kadIDC, keyC)
}

func TestNodeIsWithinNearestK(t *testing.T) {
	rt := createRT()
	midNode := id3() //[01111111, 11111111]
	rt.Self.Id = string(midNode)
	g1 := id2() //[10111111, 11111111]
	g2 := id5() //[10011111, 11111111]
	
	var g3 storage.Key //[10001111, 11111111]
	g3 = append(g3, 143)
	g3 = append(g3, 255)

	l1 := id4()//[00111111, 11111111]

	var l2 storage.Key //[00011111, 11111111]
	l2 = append(l2, 31)
	l2 = append(l2, 255)

	var l3 storage.Key //[00001111, 11111111]
	l3 = append(l3, 15)
	l3 = append(l3, 255)

	(*rt.nodeBucketDB).Put(midNode, []byte(""))
	(*rt.nodeBucketDB).Put(g1, []byte(""))
	(*rt.nodeBucketDB).Put(g2, []byte(""))
	(*rt.nodeBucketDB).Put(g3, []byte(""))
	(*rt.nodeBucketDB).Put(l1, []byte(""))
	(*rt.nodeBucketDB).Put(l2, []byte(""))
	(*rt.nodeBucketDB).Put(l3, []byte(""))

	var gTrue storage.Key //[10111110, 11111111]
	gTrue = append(gTrue, 190)
	gTrue = append(gTrue, 255)

	var lTrue storage.Key //[00011110, 11111111]
	lTrue = append(gTrue, 30)
	lTrue = append(gTrue, 255)

	gFalse := id1() //[11111111, 11111111]
	lFalse := rt.createZeroAsStorageKey() //[0, 0]

	assert.True(t, rt.nodeIsWithinNearestK(gTrue)) 
	assert.True(t, rt.nodeIsWithinNearestK(lTrue)) 
	assert.False(t, rt.nodeIsWithinNearestK(gFalse))
	assert.False(t, rt.nodeIsWithinNearestK(lFalse))
}

func TestKadBucketContainsLocalNode(t *testing.T) {
	rt := createRT()
	kadIDA := storage.Key(id1())
	kadIDB := storage.Key(id3())
	rt.createOrUpdateKBucket(kadIDA)
	rt.createOrUpdateKBucket(kadIDB)
	var nodeIDA []byte //[10110111, 1111111]
	nodeIDA = append(nodeIDA, byte(183))
	nodeIDA = append(nodeIDA, byte(255))
	(*rt.nodeBucketDB).Put(nodeIDA, []byte(""))
	rt.Self.Id = string(nodeIDA)
	resultTrue := rt.kadBucketContainsLocalNode(kadIDA)
	resultFalse := rt.kadBucketContainsLocalNode(kadIDB)
	assert.True(t, resultTrue)
	assert.False(t, resultFalse)
}

func TestKadBucketHasRoom(t *testing.T) {
	rt := createRT()
	kadIDA := storage.Key(id1())
	rt.createOrUpdateKBucket(kadIDA)
	node1 := id1()
	node2 := id2()
	node3 := id3()
	node4 := id4()
	node5 := id5()
	node6 := id6()

	//Fails when kad bucket has 0 nodes in it due to ../storage/boltdb/client.go:99
	// result := rt.kadBucketHasRoom(kadIDA)
	// assert.True(t, result)

	(*rt.nodeBucketDB).Put(node1, []byte(""))
	resultA := rt.kadBucketHasRoom(kadIDA)
	assert.True(t, resultA)
	(*rt.nodeBucketDB).Put(node2, []byte(""))
	(*rt.nodeBucketDB).Put(node3, []byte(""))
	(*rt.nodeBucketDB).Put(node4, []byte(""))
	(*rt.nodeBucketDB).Put(node5, []byte(""))
	(*rt.nodeBucketDB).Put(node6, []byte(""))
	resultB := rt.kadBucketHasRoom(kadIDA)
	assert.False(t, resultB)
}

func TestGetNodeIDsWithinKBucket(t *testing.T) {
	rt := createRT()
	kadIDA := storage.Key(id1())
	kadIDB := storage.Key(id3())
	rt.createOrUpdateKBucket(kadIDA)
	rt.createOrUpdateKBucket(kadIDB)
	
	var nodeIDA []byte //[10110111, 1111111]
	nodeIDA = append(nodeIDA, byte(183))
	nodeIDA = append(nodeIDA, byte(255))

	var nodeIDB []byte //[01101111, 1111111]
	nodeIDB = append(nodeIDB, byte(111))
	nodeIDB = append(nodeIDB, byte(255))
	
	var nodeIDC []byte //[00101111, 1111111]
	nodeIDC = append(nodeIDC, byte(47))
	nodeIDC = append(nodeIDC, byte(255))
	
	(*rt.nodeBucketDB).Put(nodeIDA, []byte(""))
	(*rt.nodeBucketDB).Put(nodeIDB, []byte(""))
	(*rt.nodeBucketDB).Put(nodeIDC, []byte(""))

	var expectedA storage.Keys
	var expectedB storage.Keys

	expectedA = append(expectedA, nodeIDA)
	expectedB = append(expectedB, nodeIDC)
	expectedB = append(expectedB, nodeIDB)

	A := rt.getNodeIDsWithinKBucket(kadIDA)
	B := rt.getNodeIDsWithinKBucket(kadIDB)

	assert.Equal(t, expectedA, A)
	assert.Equal(t, expectedB, B)
}

func TestGetKBucketRange(t *testing.T) {
	rt := createRT()
	idA := id1()
	idB := id3()
	idC := id4()
	(*rt.kadBucketDB).Put(idA, []byte(""))
	(*rt.kadBucketDB).Put(idB, []byte(""))
	(*rt.kadBucketDB).Put(idC, []byte(""))
	var expectedA storage.Keys
	expectedA = append(expectedA, idB)
	expectedA = append(expectedA, idA)

	var expectedB storage.Keys
	expectedB = append(expectedB, idC)
	expectedB = append(expectedB, idB)

	var expectedC storage.Keys
	expectedC = append(expectedC, rt.createZeroAsStorageKey())
	expectedC = append(expectedC, idC)

	endpointsA := rt.getKBucketRange(idA)
	endpointsB := rt.getKBucketRange(idB)
	endpointsC := rt.getKBucketRange(idC)
	assert.Equal(t, expectedA, endpointsA)
	assert.Equal(t, expectedB, endpointsB)
	assert.Equal(t, expectedC, endpointsC)
}

func TestCreateFirstBucketID(t *testing.T) {
	rt := createRT()
	x := rt.createFirstBucketID()
	var expected []byte
	expected = append(expected, byte(255))
	expected = append(expected, byte(255))
	assert.Equal(t, x, expected)
}

func TestCreateZeroAsStorageKey(t *testing.T) {
	rt := createRT()
	zero := rt.createZeroAsStorageKey()
	var expected []byte
	expected = append(expected, byte(0))
	expected = append(expected, byte(0))
	assert.Equal(t, zero, storage.Key(expected))
}

func TestDetermineLeafDepth(t *testing.T) {
	rt := createRT()
	idA := id1()
	idB := id3()
	idC := id4()

	(*rt.kadBucketDB).Put(idA, []byte(""))
	first := rt.determineLeafDepth(idA)
	assert.Equal(t, 0, first)

	(*rt.kadBucketDB).Put(idB, []byte(""))
	second := rt.determineLeafDepth(idB)
	assert.Equal(t, 1, second)

	(*rt.kadBucketDB).Put(idC, []byte(""))
	one := rt.determineLeafDepth(idA)
	assert.Equal(t, 1, one) 
	two := rt.determineLeafDepth(idB)
	assert.Equal(t, 2, two) 
	alsoTwo := rt.determineLeafDepth(idC)
	assert.Equal(t, 2, alsoTwo)

}

func TestDetermineDifferingBitIndex(t *testing.T) {
	id1 := id1()
	id2 := id2()
	assert.Equal(t, 1, determineDifferingBitIndex(id2, id1))
	id3 := id3()
	id8 := id8() 
	id10 := id10()
	assert.Equal(t, 2, determineDifferingBitIndex(id8, id3)) 
	assert.Equal(t, 3, determineDifferingBitIndex(id8, id10))
}

func TestSplitBucket(t *testing.T) {
	rt := createRT()
	id1 := id1()
	id2 := id2()
	id3 := id3()
	id4 := id4()
	id5 := id5()
	id6 := id6()
	id7 := id7()
	id8 := id8()
	id9 := id9()
	newID1 := rt.splitBucket(id1, 1) //[11111111, 11111111] -> [10111111, 11111111]
	assert.Equal(t, id2, newID1)

	newID2 := rt.splitBucket(id2, 2) //[10111111, 11111111] -> [10011111, 11111111]
	assert.Equal(t, id5, newID2) 
	
	newID3 := rt.splitBucket(id3, 1) //[01111111, 11111111] -> [00111111, 11111111]
	assert.Equal(t, id4, newID3) 

	newID4 := rt.splitBucket(id7, 8) //[00000000, 11111111] -> [00000000, 01111111]
	assert.Equal(t, id6, newID4) 

	newID5 := rt.splitBucket(id8, 4)
	assert.Equal(t, id9 ,newID5)
}

//test helpers ----------------------------------------------------
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
	localNode := proto.Node{ Id: string(localNodeID) }
	rt, _ := NewRoutingTable(&localNode, tempfile("Kadbucket"),tempfile("Nodebucket"), 16, 6)
	return rt
}


func mockNodes(id string) *proto.Node {
	var node proto.Node
	node.Id = id
	return &node
}


func id1() []byte {
	var id1 []byte //[11111111, 11111111]
	id1 = append(id1, byte(255))
	id1 = append(id1, byte(255))
	return id1
}

func id2() []byte{
	var id2 []byte //[10111111, 11111111]
	id2 = append(id2, byte(191))
	id2 = append(id2, byte(255))
	return id2
}

func id3() []byte {
	var id3 []byte //[01111111, 11111111]
	id3 = append(id3, byte(127))
	id3 = append(id3, byte(255))
	return id3
}

func id4() []byte {
	var id4 []byte //[00111111, 11111111]
	id4 = append(id4, byte(63))
	id4 = append(id4, byte(255))
	return id4
}

func id5() []byte {
	var id5 []byte //[10011111, 11111111]
	id5 = append(id5, byte(159))
	id5 = append(id5, byte(255))
	return id5
}

func id6() []byte {
	var id6 []byte //[00000000, 01111111]
	id6 = append(id6, byte(0))
	id6 = append(id6, byte(127))
	return id6
}

func id7() []byte {
	var id7 []byte //[00000000, 11111111]
	id7 = append(id7, byte(0))
	id7 = append(id7, byte(255))
	return id7
}

func id8() []byte { 
	var id8 []byte //[01011111, 11111111]
	id8 = append(id8, byte(95))
	id8 = append(id8, byte(255))
	return id8
}

func id9() []byte {
	var id9 []byte //[01010111, 11111111]
	id9 = append(id9, byte(87))
	id9 = append(id9, byte(255))
	return id9
}

func id10() []byte {
	var id10 []byte //[01001111, 11111111]
	id10 = append(id10, byte(79))
	id10 = append(id10, byte(255))
	return id10
}