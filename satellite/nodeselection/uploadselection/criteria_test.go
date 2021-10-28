// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information

package uploadselection

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/common/storj"
	"storj.io/common/testrand"
)

func TestCriteria_AutoExcludeSubnet(t *testing.T) {

	criteria := Criteria{
		AutoExcludeSubnets: map[string]struct{}{},
	}

	assert.True(t, criteria.MatchInclude(&Node{
		LastNet: "192.168.0.1",
	}))

	assert.False(t, criteria.MatchInclude(&Node{
		LastNet: "192.168.0.1",
	}))

	assert.True(t, criteria.MatchInclude(&Node{
		LastNet: "192.168.1.1",
	}))
}

func TestCriteria_ExcludeNodeID(t *testing.T) {
	included := testrand.NodeID()
	excluded := testrand.NodeID()

	criteria := Criteria{
		ExcludeNodeIDs: []storj.NodeID{excluded},
	}

	assert.False(t, criteria.MatchInclude(&Node{
		NodeURL: storj.NodeURL{
			ID:      excluded,
			Address: "localhost",
		},
	}))

	assert.True(t, criteria.MatchInclude(&Node{
		NodeURL: storj.NodeURL{
			ID:      included,
			Address: "localhost",
		},
	}))

}

func TestCriteria_NodeIDAndSubnet(t *testing.T) {
	excluded := testrand.NodeID()

	criteria := Criteria{
		ExcludeNodeIDs:     []storj.NodeID{excluded},
		AutoExcludeSubnets: map[string]struct{}{},
	}

	// due to node id criteria
	assert.False(t, criteria.MatchInclude(&Node{
		NodeURL: storj.NodeURL{
			ID:      excluded,
			Address: "192.168.0.1",
		},
	}))

	// should be included as previous one excluded and
	// not stored for subnet exclusion
	assert.True(t, criteria.MatchInclude(&Node{
		NodeURL: storj.NodeURL{
			ID:      testrand.NodeID(),
			Address: "192.168.0.2",
		},
	}))

}
