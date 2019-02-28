package pb_test

import (
	"testing"
	"time"

	proto "github.com/gogo/protobuf/proto"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/require"
	context "golang.org/x/net/context"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

func TestOrderLimitMarshalling(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()
	sat, err := testidentity.NewTestIdentity(ctx)
	require.NoError(t, err)

	orderLimit1 := NewOrderLimit(t, ctx, sat.ID)
	require.NoError(t, orderLimit1.Sign(sat.Key))
	bytes, err := proto.Marshal(orderLimit1)
	require.NoError(t, err)

	orderLimit2 := &pb.OrderLimit{}
	require.NoError(t, orderLimit2.Verify(sat.ID))
	require.NoError(t, proto.Unmarshal(bytes, orderLimit2))
	require.Equal(t, int64(2000), orderLimit1.MaxSize)
	require.Equal(t, int64(2000), orderLimit2.MaxSize)
	require.Equal(t, pb.BandwidthAction_GET_AUDIT, orderLimit1.Action)
	require.Equal(t, pb.BandwidthAction_GET_AUDIT, orderLimit1.Action)
}

func NewOrderLimit(t *testing.T, ctx context.Context, satID storj.NodeID) *pb.OrderLimit {
	upID, err := testidentity.NewTestIdentity(ctx)
	require.NoError(t, err)
	serialNum, err := uuid.New()
	require.NoError(t, err)
	return &pb.OrderLimit{
		SatelliteId:       satID,
		UplinkId:          upID.ID,
		MaxSize:           2000,
		ExpirationUnixSec: time.Now().Add(time.Hour).Unix(),
		SerialNumber:      serialNum.String(),
		Action:            pb.BandwidthAction_GET_AUDIT,
		CreatedUnixSec:    time.Now().Unix(),
	}
}
