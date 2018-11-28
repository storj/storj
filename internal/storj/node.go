package teststorj

import (
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

func NodeIDFromBytes(b []byte) storj.NodeID {
	id, _ := storj.NodeIDFromBytes(fit(b))
	return id
}

func NodeIDFromString(s string) storj.NodeID {
	return NodeIDFromBytes([]byte(s))
}

func NodeIDsFromBytes(bs ...[]byte) (ids storj.NodeIDList) {
	for _, b := range bs {
		ids = append(ids, NodeIDFromBytes(b))
	}
	return ids
}

func NodeIDsFromStrings(strs ...string) (ids storj.NodeIDList) {
	for _, s := range strs {
		ids = append(ids, NodeIDFromString(s))
	}
	return ids
}

func fit(b []byte) []byte {
	l := len(storj.NodeID{})
	if len(b) < l {
		return fit(append(b, 255))
		// return fit(append([]byte{1}, b...))
	}
	return b[:l]
}
func MockNode(s string) *pb.Node {
	id := NodeIDFromString(s)
	var node pb.Node
	node.Id = id
	return &node
}

