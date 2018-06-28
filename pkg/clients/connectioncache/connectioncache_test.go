package connectioncache

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	proto "storj.io/storj/protos/overlay"
)

func TestNewConnectionCache(t *testing.T) {
	cases := []struct {
		address string
	}{
		{
			address: "127.0.0.1:8080",
		},
	}

	for _, v := range cases {
		oc, err := NewConnectionCache(context.Background(), v.address, Overlay)
		assert.NoError(t, err)

		assert.NotNil(t, oc)
		assert.NotEmpty(t, oc)
	}
}

func TestDialNode(t *testing.T) {
	var address string = "127.0.0.1:8080"
	oc, err := NewConnectionCache(context.Background(), address, Overlay)
	assert.NoError(t, err)
	assert.NotNil(t, oc)
	assert.NotEmpty(t, oc)

	node := proto.Node{
		Id: "hello I am you ID",
		Address: &proto.NodeAddress{
			Transport: proto.NodeTransport_TCP,
			Address:   "yitkokok",
		},
	}
	conn, err := oc.DialNode(context.Background(), &node)
	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Empty(t, conn)
}
