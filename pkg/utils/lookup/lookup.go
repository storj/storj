package lookup

import (
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// NodeIDsToLookupRequests ...
func NodeIDsToLookupRequests(nodeIDs storj.NodeIDList) *pb.LookupRequests {
	var rq []*pb.LookupRequest
	for _, v := range nodeIDs {
		r := &pb.LookupRequest{NodeId: v}
		rq = append(rq, r)
	}
	return &pb.LookupRequests{Lookuprequest: rq}
}

// LookupResponsesToNodes ...
func LookupResponsesToNodes(responses *pb.LookupResponses) []*pb.Node {
	var nodes []*pb.Node
	for _, v := range responses.Lookupresponse {
		n := v.Node
		nodes = append(nodes, n)
	}
	return nodes
}
