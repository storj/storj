// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package pb

import (
	"context"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/storj"
)

func TestPayerBandwidthAllocationMarshalling(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()
	sat, err := testidentity.NewTestIdentity(ctx)
	require.NoError(t, err)

	PayerBandwidthAllocation1 := NewPayerBandwidthAllocation(ctx, t, sat)
	//requireSignedMessage(t, PayerBandwidthAllocation1, sat.ID)
	bytes, err := proto.Marshal(PayerBandwidthAllocation1)
	require.NoError(t, err)
	require.NotNil(t, bytes)
	require.NotZero(t, len(bytes))

	PayerBandwidthAllocation2 := &PayerBandwidthAllocation{}
	require.NoError(t, proto.Unmarshal(bytes, PayerBandwidthAllocation2))
	requireSignedMessage(t, PayerBandwidthAllocation1, sat.ID)
	require.NoError(t, auth.VerifyMsg(PayerBandwidthAllocation2, sat.ID))
	require.NoError(t, auth.VerifyMsg(PayerBandwidthAllocation2, sat.ID))
	requireSignedMessage(t, PayerBandwidthAllocation1, sat.ID)
	requireSignedMessage(t, PayerBandwidthAllocation2, sat.ID)

	require.Equal(t, int64(2000), PayerBandwidthAllocation1.MaxSize)
	require.Equal(t, int64(2000), PayerBandwidthAllocation2.MaxSize)
	require.Equal(t, BandwidthAction_GET_AUDIT, PayerBandwidthAllocation1.Action)
	require.Equal(t, BandwidthAction_GET_AUDIT, PayerBandwidthAllocation2.Action)
}

func requireSignedMessage(t *testing.T, sm auth.SignableMessage, id storj.NodeID) {
	require.NotNil(t, sm.GetCerts(), "Certs")
	require.NotNil(t, sm.GetSignature(), "Signature")
	require.NotZero(t, len(sm.GetCerts()), "Certs")
	require.NotZero(t, len(sm.GetSignature()), "Signature")
	require.NoError(t, auth.VerifyMsg(sm, id))
}

func NewPayerBandwidthAllocation(ctx context.Context, t *testing.T, sat *identity.FullIdentity) *PayerBandwidthAllocation {
	upID, err := testidentity.NewTestIdentity(ctx)
	require.NoError(t, err)
	serialNum, err := uuid.New()
	require.NoError(t, err)
	PayerBandwidthAllocation := &PayerBandwidthAllocation{
		SatelliteId:       sat.ID,
		UplinkId:          upID.ID,
		MaxSize:           2000,
		ExpirationUnixSec: time.Now().Add(time.Hour).Unix(),
		SerialNumber:      serialNum.String(),
		Action:            BandwidthAction_GET_AUDIT,
		CreatedUnixSec:    time.Now().Unix(),
		StorageNodeIds:    []storj.NodeID{upID.ID, sat.ID},
	}
	require.NoError(t, auth.SignMessage(PayerBandwidthAllocation, *sat))
	return PayerBandwidthAllocation
}

func NewOrder(ctx context.Context, t *testing.T, sat, up *identity.FullIdentity) *Order {
	sn, err := testidentity.NewTestIdentity(ctx)
	require.NoError(t, err)
	order := &Order{
		PayerAllocation: *NewPayerBandwidthAllocation(ctx, t, up),
		Total:           2000,
		StorageNodeId:   sn.ID,
	}
	require.NoError(t, auth.SignMessage(order, *up))
	return order
}
