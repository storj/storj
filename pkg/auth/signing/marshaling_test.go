// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package signing_test

import (
	"encoding/hex"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"

	"storj.io/storj/pkg/pb"
)

func TestOrderSerialization(t *testing.T) {
	t.Skip("broken")

	orderLimitBytes, err := hex.DecodeString(`0A1027C6C39653A24B94BA560A7951698FF312209BD465AB990C1E62C7B99FEE63E71761FF1A7ECD951D502CE95F4A41D4C91A001A209BD465AB990C1E62C7B99FEE63E71761FF1A7ECD951D502CE95F4A41D4C91A0022209A27D4F09F85609E85B861B11F95C785899DC394FEC6BD4E303C502C3B7E2B002A20A86125ACD1B98E7262F9D38D9B27204DAF4E44092B0FBA786474B4754D45753330800838034A0C08B1E1A2EA0510AEF4AED70352463044022035EE84CAE8FE8CEBA52B2C1BD7A3891FA049557D5C4DE6BDEDAF5C92E2D004FA0220170DA89541EF962538763B0B55FDD04F14A623E118F55601FD8FA7DF266A374F`)
	require.NoError(t, err)
	orderBytes, err := hex.DecodeString(`0A1027C6C39653A24B94BA560A7951698FF31080081A473045022100BB7A53C2835BF5CAC59479C7A3A17447AC9D3DAE894B20849FDDF9E3533F173202207910685EB70107BFF73A2F94AF345369E51B35208941EB5CE903E48EFFB41642`)
	require.NoError(t, err)

	orderLimit := pb.OrderLimit{}
	err = proto.Unmarshal(orderLimitBytes, &orderLimit)
	require.NoError(t, err)

	orderLimitMarshaled, err := proto.Marshal(&orderLimit)
	require.NoError(t, err)
	require.Equal(t, orderLimitBytes, orderLimitMarshaled, "order limit marshaling changed")

	order := pb.Order{}
	err = proto.Unmarshal(orderBytes, &order)
	require.NoError(t, err)

	orderBytesMarshaled, err := proto.Marshal(&order)
	require.NoError(t, err)
	require.Equal(t, orderBytes, orderBytesMarshaled, "order marshaling changed")
}
