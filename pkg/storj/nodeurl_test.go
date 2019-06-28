// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/pkg/storj"
)

func TestNodeURL(t *testing.T) {
	emptyId := storj.NodeID{}
	id, err := storj.NodeIDFromString("12vha9oTFnerxYRgeQ2BZqoFrLrnmmf5UWTCY2jA77dF3YvWew7")
	require.NoError(t, err)

	t.Run("Valid", func(t *testing.T) {
		type Test struct {
			String   string
			Expected storj.NodeURL
		}

		for _, testcase := range []Test{
			// host
			{"33.20.0.1:7777", storj.NodeURL{emptyId, "33.20.0.1:7777"}},
			{"[2001:db8:1f70::999:de8:7648:6e8]:7777", storj.NodeURL{emptyId, "[2001:db8:1f70::999:de8:7648:6e8]:7777"}},
			{"example.com:7777", storj.NodeURL{emptyId, "example.com:7777"}},
			// node id + host
			{"12vha9oTFnerxYRgeQ2BZqoFrLrnmmf5UWTCY2jA77dF3YvWew7@33.20.0.1:7777", storj.NodeURL{id, "33.20.0.1:7777"}},
			{"12vha9oTFnerxYRgeQ2BZqoFrLrnmmf5UWTCY2jA77dF3YvWew7@[2001:db8:1f70::999:de8:7648:6e8]:7777", storj.NodeURL{id, "[2001:db8:1f70::999:de8:7648:6e8]:7777"}},
			{"12vha9oTFnerxYRgeQ2BZqoFrLrnmmf5UWTCY2jA77dF3YvWew7@example.com:7777", storj.NodeURL{id, "example.com:7777"}},
			// node id
			{"12vha9oTFnerxYRgeQ2BZqoFrLrnmmf5UWTCY2jA77dF3YvWew7@", storj.NodeURL{id, ""}},
		} {
			url, err := storj.ParseNodeURL(testcase.String)
			require.NoError(t, err, testcase.String)

			require.Equal(t, testcase.Expected, url)
			require.Equal(t, testcase.String, url.String())
		}
	})
}
