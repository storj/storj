// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testrand"
)

func TestRandomWeight(t *testing.T) {
	w := WeightedRandom{16, 32, 32, 32, 100, 1000}
	selectionHistogram := make([]int, len(w))

	// we select 3 out of 5 elements 1000 times
	for i := 0; i < 1000; i++ {
		selection := w.Random(3, []int{6})
		for _, r := range selection {
			selectionHistogram[r]++
		}
	}

	// index 1 supposed to be selected less, as the weight is lower. It couldn't be exactly 50%, so se use 70% as a threshold.
	require.Less(t, selectionHistogram[1], selectionHistogram[2]*10/7)
}

func TestNodes(t *testing.T) {
	root := Nodes{}
	serversPerDatacenter := map[string][]string{
		"dc1": {"server1", "server2", "server3", "server4", "server5", "server6"},
		"dc2": {"server1", "server2", "server3", "server4", "server5", "server6"},
		"dc3": {"server1", "server2", "server3", "server4", "server5", "server6", "server7", "server8", "server9", "server10"},
		"dc4": {"server1", "server2", "server3", "server4", "server5", "server6"},
		"dc5": {"server1", "server2"},
	}

	var attributes []NodeAttribute
	for _, attributeName := range []string{"tag:datacenter", "tag:server"} {
		a, err := CreateNodeAttribute(attributeName)
		require.NoError(t, err)
		attributes = append(attributes, a)
	}

	// will use at the end, but we need here to initialize the nodeSelection
	dcSelection := map[string]int{}
	serverSelection := map[string]int{}
	nodeSelection := map[storj.NodeID]int{}

	var high, low, excluded storj.NodeID
	{
		// building the node pool
		for dc, servers := range serversPerDatacenter {
			for _, server := range servers {
				instanceCount := 16
				if dc == "dc1" && server == "server2" {
					instanceCount = 48
				}
				for i := 0; i < instanceCount; i++ {
					node := &SelectedNode{
						ID: testrand.NodeID(),
						Tags: NodeTags{
							NodeTag{
								Name:  "datacenter",
								Value: []byte(dc),
							},
							NodeTag{
								Name:  "server",
								Value: []byte(server),
							},
							NodeTag{
								Name:  "instance",
								Value: []byte(strconv.Itoa(i)),
							},
						},
					}
					weight := 1.0

					// special cases
					if dc == "dc4" && server == "server1" {
						switch i {
						case 0:
							weight = 10.0
							high = node.ID
						case 1:
							weight = 0.001
							low = node.ID
						case 2:
							excluded = node.ID
						}
					}

					// counter for assertion. We need 0 even if nodes are not selected.
					nodeSelection[node.ID] = 0
					root.Add(node, attributes, weight)
				}
			}
		}
	}

	for i := 0; i < 10000; i++ {
		// select 3 datacenters, 2 servers from each datacenter, 1 node from each server (total 6 nodes)
		selection := root.Select([]int{3, 2, 1}, 6, []storj.NodeID{excluded})
		require.Len(t, selection, 6)
		for _, s := range selection {
			dcSelection[attributes[0](*s)]++
			serverSelection[attributes[0](*s)+"."+attributes[1](*s)]++
			nodeSelection[s.ID]++
		}
	}

	// dc1 and dc3 have more instances, so they should be selected more often
	require.Greater(t, dcSelection["dc1"], dcSelection["dc2"]*11/10)
	require.Greater(t, dcSelection["dc3"], dcSelection["dc2"]*11/10)

	// server2 in dc1 has more instances, so it should be selected more often
	require.Greater(t, serverSelection["dc1.server2"], serverSelection["dc3.server1"]*11/10)

	max, min := 0, -1

	for id, count := range nodeSelection {
		if id == excluded {
			continue
		}
		if count > max {
			max = count
		}
		if count < min || min == -1 {
			min = count
		}
	}

	require.Equal(t, max, nodeSelection[high])
	require.Equal(t, min, nodeSelection[low])
	require.Equal(t, 0, nodeSelection[excluded])
}

func TestUnbalanced(t *testing.T) {
	root := Nodes{}

	var attributes []NodeAttribute
	for _, attributeName := range []string{"tag:datacenter", "tag:server"} {
		a, err := CreateNodeAttribute(attributeName)
		require.NoError(t, err)
		attributes = append(attributes, a)
	}

	for dc := 0; dc < 3; dc++ {
		for server := 0; server < 10; server++ {
			if dc == 0 && server > 0 {
				continue
			}
			for instance := 0; instance < 16; instance++ {
				root.Add(&SelectedNode{
					ID: testrand.NodeID(),
					Tags: NodeTags{
						NodeTag{
							Name:  "datacenter",
							Value: []byte("dc" + strconv.Itoa(dc)),
						},
						NodeTag{
							Name:  "server",
							Value: []byte("s" + strconv.Itoa(server)),
						},
					},
				}, attributes, 1)
			}
		}
	}

	// splits are just ratio
	selection := root.Select([]int{3, 2}, 11, []storj.NodeID{})
	require.Equal(t, 11, len(selection))

	// we can always select 6, but will select more than one from a server
	selection = root.Select([]int{3, 1024}, 6, []storj.NodeID{})
	require.Equal(t, 6, len(selection))

	// as one datacenter doesn't have 2 servers, we should select more from other datacenters
	selection = root.Select([]int{3, 1024}, 6, []storj.NodeID{})
	require.Equal(t, 6, len(selection))
}
