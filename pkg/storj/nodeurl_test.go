// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/pkg/storj"
)

func TestNodeURL(t *testing.T) {
	emptyID := storj.NodeID{}
	id, err := storj.NodeIDFromString("12vha9oTFnerxYRgeQ2BZqoFrLrnmmf5UWTCY2jA77dF3YvWew7")
	require.NoError(t, err)

	t.Run("Valid", func(t *testing.T) {
		type Test struct {
			String   string
			Expected storj.NodeURL
		}

		for _, testcase := range []Test{
			// host
			{"33.20.0.1:7777", storj.NodeURL{emptyID, "33.20.0.1:7777"}},
			{"[2001:db8:1f70::999:de8:7648:6e8]:7777", storj.NodeURL{emptyID, "[2001:db8:1f70::999:de8:7648:6e8]:7777"}},
			{"example.com:7777", storj.NodeURL{emptyID, "example.com:7777"}},
			// node id + host
			{"12vha9oTFnerxYRgeQ2BZqoFrLrnmmf5UWTCY2jA77dF3YvWew7@33.20.0.1:7777", storj.NodeURL{id, "33.20.0.1:7777"}},
			{"12vha9oTFnerxYRgeQ2BZqoFrLrnmmf5UWTCY2jA77dF3YvWew7@[2001:db8:1f70::999:de8:7648:6e8]:7777", storj.NodeURL{id, "[2001:db8:1f70::999:de8:7648:6e8]:7777"}},
			{"12vha9oTFnerxYRgeQ2BZqoFrLrnmmf5UWTCY2jA77dF3YvWew7@example.com:7777", storj.NodeURL{id, "example.com:7777"}},
			// node id
			{"12vha9oTFnerxYRgeQ2BZqoFrLrnmmf5UWTCY2jA77dF3YvWew7@", storj.NodeURL{id, ""}},
		} {
			url, err := storj.ParseNodeURL(testcase.String)
			require.NoError(t, err, testcase.String)

			assert.Equal(t, testcase.Expected, url)
			assert.Equal(t, testcase.String, url.String())
		}
	})

	t.Run("Invalid", func(t *testing.T) {
		for _, testcase := range []string{
			"",
			// invalid host
			"exampl e.com:7777",
			// invalid node id
			"12vha9oTFnerxgeQ2BZqoFrLrnmmf5UWTCY2jA77dF3YvWew7@33.20.0.1:7777",
			"12vha9oTFnerx YRgeQ2BZqoFrLrnmmf5UWTCY2jA77dF3YvWew7@[2001:db8:1f70::999:de8:7648:6e8]:7777",
			"12vha9oTFnerxYRgeQ2BZqoFrLrn_5UWTCY2jA77dF3YvWew7@example.com:7777",
			// invalid node id
			"1112vha9oTFnerxYRgeQ2BZqoFrLrnmmf5UWTCY2jA77dF3YvWew7@",
		} {
			_, err := storj.ParseNodeURL(testcase)
			assert.Error(t, err, testcase)
		}
	})
}

func TestNodeURLs(t *testing.T) {
	emptyID := storj.NodeID{}
	id, err := storj.NodeIDFromString("12vha9oTFnerxYRgeQ2BZqoFrLrnmmf5UWTCY2jA77dF3YvWew7")
	require.NoError(t, err)

	s := "33.20.0.1:7777," +
		"12vha9oTFnerxYRgeQ2BZqoFrLrnmmf5UWTCY2jA77dF3YvWew7@[2001:db8:1f70::999:de8:7648:6e8]:7777," +
		"12vha9oTFnerxYRgeQ2BZqoFrLrnmmf5UWTCY2jA77dF3YvWew7@example.com," +
		"12vha9oTFnerxYRgeQ2BZqoFrLrnmmf5UWTCY2jA77dF3YvWew7@"
	urls, err := storj.ParseNodeURLs(s)
	require.NoError(t, err)
	require.Equal(t, storj.NodeURLs{
		storj.NodeURL{emptyID, "33.20.0.1:7777"},
		storj.NodeURL{id, "[2001:db8:1f70::999:de8:7648:6e8]:7777"},
		storj.NodeURL{id, "example.com"},
		storj.NodeURL{id, ""},
	}, urls)

	require.Equal(t, s, urls.String())
}
