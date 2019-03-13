// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package ecclient

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/transport"
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

func TestUnique(t *testing.T) {
	limits := make([]*pb.AddressedOrderLimit, 4)
	for i := 0; i < len(limits); i++ {
		limits[i] = &pb.AddressedOrderLimit{
			Limit: &pb.OrderLimit2{
				StorageNodeId: teststorj.NodeIDFromString(fmt.Sprintf("node-%d", i)),
			},
		}
	}

	for i, tt := range []struct {
		limits []*pb.AddressedOrderLimit
		unique bool
	}{
		{nil, true},
		{[]*pb.AddressedOrderLimit{}, true},
		{[]*pb.AddressedOrderLimit{limits[0]}, true},
		{[]*pb.AddressedOrderLimit{limits[0], limits[1]}, true},
		{[]*pb.AddressedOrderLimit{limits[0], limits[0]}, false},
		{[]*pb.AddressedOrderLimit{limits[0], limits[1], limits[0]}, false},
		{[]*pb.AddressedOrderLimit{limits[1], limits[0], limits[0]}, false},
		{[]*pb.AddressedOrderLimit{limits[0], limits[0], limits[1]}, false},
		{[]*pb.AddressedOrderLimit{limits[2], limits[0], limits[1]}, true},
		{[]*pb.AddressedOrderLimit{limits[2], limits[0], limits[3], limits[1]}, true},
		{[]*pb.AddressedOrderLimit{limits[2], limits[0], limits[2], limits[1]}, false},
		{[]*pb.AddressedOrderLimit{limits[1], limits[0], limits[3], limits[1]}, false},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		assert.Equal(t, tt.unique, unique(tt.limits), errTag)
	}
}
