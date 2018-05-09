package kademlia

import (
	"time"

	"storj.io/storj/protos/overlay"
)

type RouteTable struct {
}

func (rt RouteTable) LocalID() NodeID {
	return ""
}

func (rt RouteTable) K() int {
	return 0
}

func (rt RouteTable) CacheSize() int {
	return 0
}

func (rt RouteTable) GetBucket(id string) (bucket Bucket, ok bool) {
	return KBucket{}, true
}

func (rt RouteTable) GetBuckets() ([]*Bucket, error) {
	return []*Bucket{}, nil
}

func (rt RouteTable) FindNear(id NodeID, limit int) ([]overlay.Node, error) {
	return []overlay.Node{}, nil
}

func (rt RouteTable) ConnectionSuccess(id string, address overlay.NodeAddress) {
	return
}

func (rt RouteTable) ConnectionFailed(id string, address overlay.NodeAddress) {
	return
}

func (rt RouteTable) SetBucketTimestamp(id string, now time.Time) error {
	return nil
}

func (rt RouteTable) GetBucketTimestamp(id string, bucket Bucket) (time.Time, error) {
	return time.Now(), nil
}
