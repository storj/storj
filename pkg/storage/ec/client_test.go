// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package ecclient

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vivint/infectious"

	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/piecestore/psclient"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/transport"
)

const (
	dialFailed = "dial failed"
	opFailed   = "op failed"
)

var (
	ErrDialFailed = errors.New(dialFailed)
	ErrOpFailed   = errors.New(opFailed)
)

var (
	node0 = teststorj.MockNode("node-0")
	node1 = teststorj.MockNode("node-1")
	node2 = teststorj.MockNode("node-2")
	node3 = teststorj.MockNode("node-3")
)

func TestNewECClient(t *testing.T) {
	ident, err := identity.FullIdentityFromPEM([]byte(`-----BEGIN CERTIFICATE-----
MIIBPzCB56ADAgECAhBkctCIgrE25/vSSXpUno5SMAoGCCqGSM49BAMCMAAwIhgP
MDAwMTAxMDEwMDAwMDBaGA8wMDAxMDEwMTAwMDAwMFowADBZMBMGByqGSM49AgEG
CCqGSM49AwEHA0IABFaIq+DPJfvMv8RwFXIpGGxLOHCbsvG8iMyAarv04l8QptPP
nSEKiod+KGbhQ6pEJZ0eWEyDbkA9RsUG/axNX96jPzA9MA4GA1UdDwEB/wQEAwIF
oDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwDAYDVR0TAQH/BAIwADAK
BggqhkjOPQQDAgNHADBEAiAc+6+oquoS0zcYrLd4rmoZC6uoh4ItQvH5phP0MK3b
YAIgDznIZz/oeowiv+Ui6HZT7aclBvTGjrfHR7Uo7TeGFls=
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
MIIBOjCB4KADAgECAhA7Yb8vONMfR8ri8DCmFP7hMAoGCCqGSM49BAMCMAAwIhgP
MDAwMTAxMDEwMDAwMDBaGA8wMDAxMDEwMTAwMDAwMFowADBZMBMGByqGSM49AgEG
CCqGSM49AwEHA0IABCqtWDMdx38NKcTW58up4SLn6d6f+E4jljovCp9YY4zVg2lk
/GyDAb5tuB/WttbZUO7VUMSdYjpSH5sad8uff3+jODA2MA4GA1UdDwEB/wQEAwIC
BDATBgNVHSUEDDAKBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MAoGCCqGSM49
BAMCA0kAMEYCIQDFCnJ5qV6KyN2AGD7exywI5ls7Jo3scBO8ekuXT2yNhQIhAK3W
qYzzqaR5oPuEeRSitAbV69mNcKznpU21jCnnuSq9
-----END CERTIFICATE-----
`), []byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEICvE+Bd39LJ3VVf/SBdkw/IPjyVmMWq8Sr7GuWzkfdpJoAoGCCqGSM49
AwEHoUQDQgAEVoir4M8l+8y/xHAVcikYbEs4cJuy8byIzIBqu/TiXxCm08+dIQqK
h34oZuFDqkQlnR5YTINuQD1GxQb9rE1f3g==
-----END EC PRIVATE KEY-----`))
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mbm := 1234

	clientOptions, err := tlsopts.NewOptions(ident, tlsopts.Config{})
	require.NoError(t, err)

	clientTransport := transport.NewClient(clientOptions)

	ec := NewClient(clientTransport, mbm)
	assert.NotNil(t, ec)

	ecc, ok := ec.(*ecClient)
	assert.True(t, ok)
	assert.NotNil(t, ecc.transport)
	assert.Equal(t, mbm, ecc.memoryLimit)
}

func TestPut(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	size := 32 * 1024
	k := 2
	n := 4
	fc, err := infectious.NewFEC(k, n)
	if !assert.NoError(t, err) {
		return
	}
	es := eestream.NewRSScheme(fc, size/n)

TestLoop:
	for i, tt := range []struct {
		nodes     []*pb.Node
		min       int
		badInput  bool
		errs      []error
		errString string
	}{
		{[]*pb.Node{}, 0, true, []error{},
			fmt.Sprintf("ecclient error: size of nodes slice (0) does not match total count (%v) of erasure scheme", n)},
		{[]*pb.Node{node0, node1, node0, node3}, 0, true,
			[]error{nil, nil, nil, nil},
			"ecclient error: duplicated nodes are not allowed"},
		{[]*pb.Node{node0, node1, node2, node3}, 0, false,
			[]error{nil, nil, nil, nil}, ""},
		{[]*pb.Node{node0, node1, node2, node3}, 0, false,
			[]error{nil, ErrDialFailed, nil, nil},
			"ecclient error: successful puts (3) less than repair threshold (4)"},
		{[]*pb.Node{node0, node1, node2, node3}, 0, false,
			[]error{nil, ErrOpFailed, nil, nil},
			"ecclient error: successful puts (3) less than repair threshold (4)"},
		{[]*pb.Node{node0, node1, node2, node3}, 2, false,
			[]error{nil, ErrDialFailed, nil, nil}, ""},
		{[]*pb.Node{node0, node1, node2, node3}, 2, false,
			[]error{ErrOpFailed, ErrDialFailed, nil, ErrDialFailed},
			"ecclient error: successful puts (1) less than repair threshold (2)"},
		{[]*pb.Node{nil, nil, node2, node3}, 2, false,
			[]error{nil, nil, nil, nil}, ""},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		id := psclient.NewPieceID()
		ttl := time.Now()

		errs := make(map[*pb.Node]error, len(tt.nodes))
		for i, n := range tt.nodes {
			errs[n] = tt.errs[i]
		}

		clients := make(map[*pb.Node]psclient.Client, len(tt.nodes))
		for _, n := range tt.nodes {
			if n == nil || tt.badInput {
				continue
			}
			n.Type.DPanicOnInvalid("ec client test 1")
			derivedID, err := id.Derive(n.Id.Bytes())
			if !assert.NoError(t, err, errTag) {
				continue TestLoop
			}
			ps := NewMockPSClient(ctrl)
			gomock.InOrder(
				ps.EXPECT().Put(gomock.Any(), derivedID, gomock.Any(), ttl, gomock.Any(), gomock.Any()).Return(errs[n]).
					Do(func(ctx context.Context, id psclient.PieceID, data io.Reader, ttl time.Time, ba *pb.PayerBandwidthAllocation, authorization *pb.SignedMessage) {
						// simulate that the mocked piece store client is reading the data
						_, err := io.Copy(ioutil.Discard, data)
						assert.NoError(t, err, errTag)
					}),
				ps.EXPECT().Close().Return(nil),
			)
			clients[n] = ps
		}
		rs, err := eestream.NewRedundancyStrategy(es, tt.min, 0)
		if !assert.NoError(t, err, errTag) {
			continue
		}
		r := io.LimitReader(rand.Reader, int64(size))
		ec := ecClient{newPSClientFunc: mockNewPSClient(clients)}

		successfulNodes, err := ec.Put(ctx, tt.nodes, rs, id, r, ttl, nil, nil)

		if tt.errString != "" {
			assert.EqualError(t, err, tt.errString, errTag)
			continue
		}

		assert.NoError(t, err, errTag)
		assert.Equal(t, len(tt.nodes), len(successfulNodes), errTag)

		slowNodes := 0
		for i := range tt.nodes {
			if tt.errs[i] != nil {
				assert.Nil(t, successfulNodes[i], errTag)
			} else if successfulNodes[i] == nil && tt.nodes[i] != nil {
				slowNodes++
			} else {
				assert.Equal(t, tt.nodes[i], successfulNodes[i], errTag)
			}
		}

		if slowNodes > n-k {
			assert.Fail(t, fmt.Sprintf("Too many slow nodes: \n"+
				"expected: <= %d\n"+
				"actual  :    %d", n-k, slowNodes), errTag)
		}
	}
}

func mockNewPSClient(clients map[*pb.Node]psclient.Client) psClientFunc {
	return func(_ context.Context, _ transport.Client, n *pb.Node, _ int) (psclient.Client, error) {
		n.Type.DPanicOnInvalid("mock new ps client")
		c, ok := clients[n]
		if !ok {
			return nil, ErrDialFailed
		}

		return c, nil
	}
}

func TestGet(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	size := 32 * 1024
	k := 2
	n := 4
	fc, err := infectious.NewFEC(k, n)
	if !assert.NoError(t, err) {
		return
	}
	es := eestream.NewRSScheme(fc, size/n)

TestLoop:
	for i, tt := range []struct {
		nodes     []*pb.Node
		mbm       int
		errs      []error
		errString string
	}{
		{[]*pb.Node{}, 0, []error{}, "ecclient error: " +
			fmt.Sprintf("size of nodes slice (0) does not match total count (%v) of erasure scheme", n)},
		{[]*pb.Node{node0, node1, node2, node3}, -1,
			[]error{nil, nil, nil, nil},
			"eestream error: negative max buffer memory"},
		{[]*pb.Node{node0, node1, node2, node3}, 0,
			[]error{nil, nil, nil, nil}, ""},
		{[]*pb.Node{node0, node1, node2, node3}, 0,
			[]error{nil, ErrDialFailed, nil, nil}, ""},
		{[]*pb.Node{node0, node1, node2, node3}, 0,
			[]error{nil, ErrOpFailed, nil, nil}, ""},
		{[]*pb.Node{node0, node1, node2, node3}, 0,
			[]error{ErrOpFailed, ErrDialFailed, nil, ErrDialFailed}, ""},
		{[]*pb.Node{node0, node1, node2, node3}, 0,
			[]error{ErrDialFailed, ErrOpFailed, ErrOpFailed, ErrDialFailed}, ""},
		{[]*pb.Node{nil, nil, node2, node3}, 0,
			[]error{nil, nil, nil, nil}, ""},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		id := psclient.NewPieceID()

		errs := make(map[*pb.Node]error, len(tt.nodes))
		for i, n := range tt.nodes {
			errs[n] = tt.errs[i]
		}

		clients := make(map[*pb.Node]psclient.Client, len(tt.nodes))
		for _, n := range tt.nodes {
			if errs[n] == ErrOpFailed {
				derivedID, err := id.Derive(n.Id.Bytes())
				if !assert.NoError(t, err, errTag) {
					continue TestLoop
				}
				ps := NewMockPSClient(ctrl)
				ps.EXPECT().Get(gomock.Any(), derivedID, int64(size/k), gomock.Any(), gomock.Any()).Return(ranger.ByteRanger(nil), errs[n])
				clients[n] = ps
			}
		}
		ec := ecClient{newPSClientFunc: mockNewPSClient(clients), memoryLimit: tt.mbm}
		rr, err := ec.Get(ctx, tt.nodes, es, id, int64(size), nil, nil)
		if err == nil {
			_, err := rr.Range(ctx, 0, 0)
			assert.NoError(t, err, errTag)
		}
		if tt.errString != "" {
			assert.EqualError(t, err, tt.errString, errTag)
		} else {
			assert.NoError(t, err, errTag)
			assert.NotNil(t, rr, errTag)
		}
	}
}

func TestDelete(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

TestLoop:
	for i, tt := range []struct {
		nodes     []*pb.Node
		errs      []error
		errString string
	}{
		{[]*pb.Node{}, []error{}, ""},
		{[]*pb.Node{node0}, []error{nil}, ""},
		{[]*pb.Node{node0}, []error{ErrDialFailed}, dialFailed},
		{[]*pb.Node{node0}, []error{ErrOpFailed}, opFailed},
		{[]*pb.Node{node0, node1}, []error{nil, nil}, ""},
		{[]*pb.Node{node0, node1}, []error{ErrDialFailed, nil}, ""},
		{[]*pb.Node{node0, node1}, []error{nil, ErrOpFailed}, ""},
		{[]*pb.Node{node0, node1}, []error{ErrDialFailed, ErrDialFailed}, dialFailed},
		{[]*pb.Node{node0, node1}, []error{ErrOpFailed, ErrOpFailed}, opFailed},
		{[]*pb.Node{nil, node1}, []error{nil, nil}, ""},
		{[]*pb.Node{nil, nil}, []error{nil, nil}, ""},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		id := psclient.NewPieceID()

		errs := make(map[*pb.Node]error, len(tt.nodes))
		for i, n := range tt.nodes {
			errs[n] = tt.errs[i]
		}

		clients := make(map[*pb.Node]psclient.Client, len(tt.nodes))
		for _, n := range tt.nodes {
			if n != nil && errs[n] != ErrDialFailed {
				derivedID, err := id.Derive(n.Id.Bytes())
				if !assert.NoError(t, err, errTag) {
					continue TestLoop
				}
				ps := NewMockPSClient(ctrl)
				gomock.InOrder(
					ps.EXPECT().Delete(gomock.Any(), derivedID, gomock.Any()).Return(errs[n]),
					ps.EXPECT().Close().Return(nil),
				)
				clients[n] = ps
			}
		}

		ec := ecClient{newPSClientFunc: mockNewPSClient(clients)}
		err := ec.Delete(ctx, tt.nodes, id, nil)

		if tt.errString != "" {
			assert.EqualError(t, err, tt.errString, errTag)
		} else {
			assert.NoError(t, err, errTag)
		}
	}
}

func TestUnique(t *testing.T) {
	for i, tt := range []struct {
		nodes  []*pb.Node
		unique bool
	}{
		{nil, true},
		{[]*pb.Node{}, true},
		{[]*pb.Node{node0}, true},
		{[]*pb.Node{node0, node1}, true},
		{[]*pb.Node{node0, node0}, false},
		{[]*pb.Node{node0, node1, node0}, false},
		{[]*pb.Node{node1, node0, node0}, false},
		{[]*pb.Node{node0, node0, node1}, false},
		{[]*pb.Node{node2, node0, node1}, true},
		{[]*pb.Node{node2, node0, node3, node1}, true},
		{[]*pb.Node{node2, node0, node2, node1}, false},
		{[]*pb.Node{node1, node0, node3, node1}, false},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		assert.Equal(t, tt.unique, unique(tt.nodes), errTag)
	}
}
