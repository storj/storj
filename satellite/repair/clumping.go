// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package repair

import "storj.io/storj/satellite/metabase"

// FindClumpedPieces finds pieces that are stored in the same last_net (i.e., the same /24 network
// in the IPv4 case). The first piece for a given last_net is fine, but any subsequent pieces in
// the same last_net will be returned as part of the 'clumped' list.
//
// lastNets must be a slice of the same length as pieces; lastNets[i] corresponds to pieces[i].
func FindClumpedPieces(pieces metabase.Pieces, lastNets []string) (clumped metabase.Pieces) {
	lastNetSet := make(map[string]struct{})
	for i, p := range pieces {
		lastNet := lastNets[i]
		_, ok := lastNetSet[lastNet]
		if ok {
			// this last_net was already seen
			clumped = append(clumped, p)
		} else {
			// add this last_net to the set of seen nets
			lastNetSet[lastNet] = struct{}{}
		}
	}
	return clumped
}
