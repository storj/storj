package teststorj

import "storj.io/storj/pkg/storj"

func NodeIDFromString(s string) storj.NodeID {
	b := []byte(s)
	return NodeIDFromBytes(b)
}

func NodeIDFromBytes(b []byte) storj.NodeID {
	for {
		if l := storj.IdentityLength - len(b); l <= 0 {
			break
		}
		b = append([]byte{1}, b...)
		// b = append(b, 1)
	}
	id, _ := storj.NodeIDFromBytes(b)
	return id
}
