package pb_test

import (
	fmt "fmt"
	"testing"
	"time"

	proto "github.com/gogo/protobuf/proto"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/require"
	context "golang.org/x/net/context"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

func TestOrderLimitMarshalling(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()
	sat, err := testidentity.NewTestIdentity(ctx)
	require.NoError(t, err)

	orderLimit1 := NewOrderLimit(t, ctx, sat)
	requireSignedMessage(t, orderLimit1, sat.ID)
	bytes, err := proto.Marshal(orderLimit1)
	require.NoError(t, err)
	require.NotNil(t, bytes)
	require.NotZero(t, len(bytes))

	orderLimit2 := &pb.OrderLimit{}
	require.NoError(t, proto.Unmarshal(bytes, orderLimit2))
	requireSignedMessage(t, orderLimit1, sat.ID)
	require.NoError(t, orderLimit2.Verify(sat.ID))
	require.NoError(t, orderLimit2.Verify(sat.ID))
	requireSignedMessage(t, orderLimit1, sat.ID)
	requireSignedMessage(t, orderLimit2, sat.ID)

	require.Equal(t, int64(2000), orderLimit1.MaxSize)
	require.Equal(t, int64(2000), orderLimit2.MaxSize)
	require.Equal(t, pb.BandwidthAction_GET_AUDIT, orderLimit1.Action)
	require.Equal(t, pb.BandwidthAction_GET_AUDIT, orderLimit2.Action)
}

func TestOrderMarshalling(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()
	sat, err := testidentity.NewTestIdentity(ctx)
	require.NoError(t, err)
	up, err := testidentity.NewTestIdentity(ctx)
	require.NoError(t, err)

	order1 := NewOrder(t, ctx, sat, up)
	requireSignedMessage(t, order1, up.ID)
	requireSignedMessage(t, &order1.OrderLimit, sat.ID)
	bytes, err := proto.Marshal(order1)
	require.NoError(t, err)
	require.NotNil(t, bytes)
	require.NotZero(t, len(bytes))

	order2 := &pb.Order{}
	require.NoError(t, proto.Unmarshal(bytes, order2))
	requireSignedMessage(t, order1, up.ID)
	requireSignedMessage(t, &order1.OrderLimit, sat.ID)
	requireSignedMessage(t, order2, up.ID)
	requireSignedMessage(t, &order2.OrderLimit, sat.ID)

	require.Equal(t, int64(2000), order1.Total, fmt.Sprintf("!!! %+v\n", *order1))
	require.Equal(t, int64(2000), order2.Total, fmt.Sprintf("!!! %+v\n", *order2))
	require.Equal(t, pb.BandwidthAction_GET_AUDIT, order1.OrderLimit.Action, fmt.Sprintf("!!! %+v\n", *order1))
	require.Equal(t, pb.BandwidthAction_GET_AUDIT, order2.OrderLimit.Action, fmt.Sprintf("!!! %+v\n", *order2))
}

func requireSignedMessage(t *testing.T, s pb.Signed, id storj.NodeID) {
	sm := s.GetSigned()
	require.NotNil(t, sm.Data, "Data")
	require.NotNil(t, sm.Certs, "Certs")
	require.NotNil(t, sm.Signature, "Signature")
	require.NotZero(t, len(sm.Data), "Data")
	require.NotZero(t, len(sm.Certs), "Certs")
	require.NotZero(t, len(sm.Signature), "Signature")
	//require.NoError(t, pb.Verify(s, id))
}

func NewOrderLimit(t *testing.T, ctx context.Context, sat *identity.FullIdentity) *pb.OrderLimit {
	upID, err := testidentity.NewTestIdentity(ctx)
	require.NoError(t, err)
	serialNum, err := uuid.New()
	require.NoError(t, err)
	orderLimit := &pb.OrderLimit{
		SatelliteId:       sat.ID,
		UplinkId:          upID.ID,
		MaxSize:           2000,
		ExpirationUnixSec: time.Now().Add(time.Hour).Unix(),
		SerialNumber:      serialNum.String(),
		Action:            pb.BandwidthAction_GET_AUDIT,
		CreatedUnixSec:    time.Now().Unix(),
	}
	require.NoError(t, orderLimit.Sign(sat))
	return orderLimit
}

func NewOrder(t *testing.T, ctx context.Context, sat, up *identity.FullIdentity) *pb.Order {
	sn, err := testidentity.NewTestIdentity(ctx)
	require.NoError(t, err)
	order := &pb.Order{
		OrderLimit:    *NewOrderLimit(t, ctx, up),
		Total:         2000,
		StorageNodeId: sn.ID,
	}
	require.NoError(t, order.Sign(up))
	return order
}
