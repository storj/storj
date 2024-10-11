// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/identity/testidentity"
	"storj.io/common/storj"
	"storj.io/storj/shared/location"
)

func TestNodeAttribute(t *testing.T) {
	must := func(a NodeAttribute, err error) NodeAttribute {
		require.NoError(t, err)
		return a
	}

	assert.Equal(t, "127.0.0.1", must(CreateNodeAttribute("last_net"))(SelectedNode{
		LastNet: "127.0.0.1",
	}))

	assert.Equal(t, "1.2.3.4", must(CreateNodeAttribute("last_ip"))(SelectedNode{
		LastNet:    "1.2.3.0",
		LastIPPort: "1.2.3.4:1234",
	}))

	assert.Equal(t, "0xCAFEBABE", must(CreateNodeAttribute("wallet"))(SelectedNode{
		Wallet: "0xCAFEBABE",
	}))

	assert.Equal(t, "ahoj@storj.io", must(CreateNodeAttribute("email"))(SelectedNode{
		Email: "ahoj@storj.io",
	}))

	assert.Equal(t, "DE", must(CreateNodeAttribute("country"))(SelectedNode{
		CountryCode: location.Germany,
	}))

	signerID := testidentity.MustPregeneratedIdentity(1, storj.LatestIDVersion()).ID
	otherSignerID := testidentity.MustPregeneratedIdentity(2, storj.LatestIDVersion()).ID

	assert.Equal(t, "bar", must(CreateNodeAttribute(fmt.Sprintf("tag:%s/foo", signerID)))(SelectedNode{
		Tags: NodeTags{
			{
				Signer: signerID,
				Name:   "foo",
				Value:  []byte("bar"),
			},
		},
	}))
	assert.Equal(t, "", must(CreateNodeAttribute(fmt.Sprintf("tag:%s/foo", signerID)))(SelectedNode{
		Tags: NodeTags{
			{
				Signer: otherSignerID,
				Name:   "foo",
				Value:  []byte("bar"),
			},
		},
	}))

	assert.Equal(t, "bar", must(CreateNodeAttribute("tag:foo"))(SelectedNode{
		Tags: NodeTags{
			{
				Signer: otherSignerID,
				Name:   "foo",
				Value:  []byte("bar"),
			},
		},
	}))

	assert.Equal(t, "true", must(CreateNodeAttribute("vetted"))(SelectedNode{
		Vetted: true,
	}))

	_, err := CreateNodeAttribute("tag:xxx/foo")
	require.ErrorContains(t, err, "has invalid NodeID")

	_, err = CreateNodeAttribute("tag:a/b/c")
	require.ErrorContains(t, err, "should be defined")
}

func TestSubnet(t *testing.T) {
	s := SelectedNode{
		LastIPPort: "12.23.34.45:8888",
	}
	require.Equal(t, "12.16.0.0/12", Subnet(12)(s))
	require.Equal(t, "12.23.34.45/32", Subnet(32)(s))
}

func BenchmarkSubnet(b *testing.B) {
	var s string
	for i := 0; i < b.N; i++ {
		s = Subnet(25)(SelectedNode{
			LastIPPort: fmt.Sprintf("%d.%d.%d.%d:1234", (i>>24)%256, (i>>16)%256, (i>>8)%256, i%256),
		}).(string)
		if strings.Contains(s, "error") {
			b.Fatal(s)
		}
	}
}
