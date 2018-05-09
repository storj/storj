package kademlia

import "storj.io/storj/protos/overlay"

type KBucket struct {
}

func (b KBucket) Routing() []overlay.Node {
	return []overlay.Node{}
}

func (b KBucket) Cache() []overlay.Node {
	return []overlay.Node{}
}

func (b KBucket) Midpoint() string {
	return ""
}
