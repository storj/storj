// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sort"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/private/date"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeapiversion"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/trust"
)

// DB implements saving order after receiving from storage node.
//
// architecture: Database
type DB interface {
	// UpdateBucketBandwidthAllocation updates 'allocated' bandwidth for given bucket
	UpdateBucketBandwidthAllocation(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) error
	// UpdateBucketBandwidthSettle updates 'settled' bandwidth for given bucket
	UpdateBucketBandwidthSettle(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, settledAmount, deadAmount int64, intervalStart time.Time) error
	// UpdateBucketBandwidthInline updates 'inline' bandwidth for given bucket
	UpdateBucketBandwidthInline(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) error
	// UpdateBandwidthBatch updates bucket and project bandwidth rollups in the database
	UpdateBandwidthBatch(ctx context.Context, rollups []BucketBandwidthRollup) error

	// UpdateStoragenodeBandwidthSettle updates 'settled' bandwidth for given storage node
	UpdateStoragenodeBandwidthSettle(ctx context.Context, storageNode storj.NodeID, action pb.PieceAction, amount int64, intervalStart time.Time) error
	// UpdateStoragenodeBandwidthSettleWithWindow updates 'settled' bandwidth for given storage node
	UpdateStoragenodeBandwidthSettleWithWindow(ctx context.Context, storageNodeID storj.NodeID, actionAmounts map[int32]int64, window time.Time) (status pb.SettlementWithWindowResponse_Status, alreadyProcessed bool, err error)

	// GetStorageNodeBandwidth gets total storage node bandwidth from period of time
	GetStorageNodeBandwidth(ctx context.Context, nodeID storj.NodeID, from, to time.Time) (int64, error)

	// TestGetBucketBandwidth gets total bucket bandwidth (allocated,inline,settled)
	TestGetBucketBandwidth(ctx context.Context, projectID uuid.UUID, bucketName []byte, from, to time.Time) (int64, int64, int64, error)
}

type noopDB struct {
}

func (noopDB) UpdateBucketBandwidthAllocation(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) error {
	return nil
}

func (noopDB) UpdateBucketBandwidthSettle(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, settledAmount, deadAmount int64, intervalStart time.Time) error {
	return nil
}

func (noopDB) UpdateBucketBandwidthInline(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) error {
	return nil
}

func (noopDB) UpdateBandwidthBatch(ctx context.Context, rollups []BucketBandwidthRollup) error {
	return nil
}

func (noopDB) UpdateStoragenodeBandwidthSettle(ctx context.Context, storageNode storj.NodeID, action pb.PieceAction, amount int64, intervalStart time.Time) error {
	return nil
}

func (noopDB) UpdateStoragenodeBandwidthSettleWithWindow(ctx context.Context, storageNodeID storj.NodeID, actionAmounts map[int32]int64, window time.Time) (status pb.SettlementWithWindowResponse_Status, alreadyProcessed bool, err error) {
	return pb.SettlementWithWindowResponse_ACCEPTED, false, nil
}

func (noopDB) TestGetBucketBandwidth(ctx context.Context, projectID uuid.UUID, bucketName []byte, from, to time.Time) (int64, int64, int64, error) {
	return 0, 0, 0, nil
}

func (noopDB) GetStorageNodeBandwidth(ctx context.Context, nodeID storj.NodeID, from, to time.Time) (int64, error) {
	return 0, nil
}

// NewNoopDB creates noop orders DB.
func NewNoopDB() DB {
	return &noopDB{}
}

// SerialDeleteOptions are option when deleting from serial tables.
type SerialDeleteOptions struct {
	BatchSize int
}

// ConsumedSerial is a serial that has been consumed and its bandwidth recorded.
type ConsumedSerial struct {
	NodeID       storj.NodeID
	SerialNumber storj.SerialNumber
	ExpiresAt    time.Time
}

// PendingSerial is a serial number reported by a storagenode waiting to be
// settled.
type PendingSerial struct {
	NodeID       storj.NodeID
	BucketID     []byte
	Action       uint
	SerialNumber storj.SerialNumber
	ExpiresAt    time.Time
	Settled      uint64
}

var (
	// Error the default orders errs class.
	Error = errs.Class("orders")
	// ErrUsingSerialNumber error class for serial number.
	ErrUsingSerialNumber = errs.Class("serial number")

	mon = monkit.Package()
)

// BucketBandwidthRollup contains all the info needed for a bucket bandwidth rollup.
type BucketBandwidthRollup struct {
	ProjectID     uuid.UUID
	BucketName    string
	Action        pb.PieceAction
	IntervalStart time.Time
	Inline        int64
	Allocated     int64
	Settled       int64
	Dead          int64
}

// StoragenodeBandwidthRollup contains all the info needed for a storagenode bandwidth rollup.
type StoragenodeBandwidthRollup struct {
	NodeID    storj.NodeID
	Action    pb.PieceAction
	Allocated int64
	Settled   int64
}

// SortStoragenodeBandwidthRollups sorts the rollups.
func SortStoragenodeBandwidthRollups(rollups []StoragenodeBandwidthRollup) {
	sort.SliceStable(rollups, func(i, j int) bool {
		nodeCompare := bytes.Compare(rollups[i].NodeID.Bytes(), rollups[j].NodeID.Bytes())
		switch {
		case nodeCompare == -1:
			return true
		case nodeCompare == 1:
			return false
		case rollups[i].Action < rollups[j].Action:
			return true
		case rollups[i].Action > rollups[j].Action:
			return false
		default:
			return false
		}
	})
}

// Endpoint for orders receiving.
//
// architecture: Endpoint
type Endpoint struct {
	pb.DRPCOrdersUnimplementedServer
	log              *zap.Logger
	config           Config
	satelliteSignee  signing.Signee
	DB               DB
	nodeAPIVersionDB nodeapiversion.DB
	ordersService    *Service
	overlay          *overlay.Service
}

// NewEndpoint new orders receiving endpoint.
func NewEndpoint(log *zap.Logger, satelliteSignee signing.Signee, db DB, nodeAPIVersionDB nodeapiversion.DB, ordersService *Service, config Config, overlay *overlay.Service) *Endpoint {
	return &Endpoint{
		log:              log,
		config:           config,
		satelliteSignee:  satelliteSignee,
		DB:               db,
		nodeAPIVersionDB: nodeAPIVersionDB,
		ordersService:    ordersService,
		overlay:          overlay,
	}
}

type bucketIDAction struct {
	projectID  uuid.UUID
	bucketname string
	action     pb.PieceAction
}

// SettlementWithWindow processes all orders that were created in a 1 hour window.
// Only one window is processed at a time.
// Batches are atomic, all orders are settled successfully or they all fail.
func (endpoint *Endpoint) SettlementWithWindow(stream pb.DRPCOrders_SettlementWithWindowStream) (err error) {
	return endpoint.SettlementWithWindowFinal(stream)
}

func trackFinalStatus(status pb.SettlementWithWindowResponse_Status) {
	switch status {
	case pb.SettlementWithWindowResponse_ACCEPTED:
		mon.Event("settlement_response_accepted")
	case pb.SettlementWithWindowResponse_REJECTED:
		mon.Event("settlement_response_rejected")
	default:
		mon.Event("settlement_response_unknown")
	}
}

// SettlementWithWindowFinal processes all orders that were created in a 1 hour window.
// Only one window is processed at a time.
// Batches are atomic, all orders are settled successfully or they all fail.
func (endpoint *Endpoint) SettlementWithWindowFinal(stream pb.DRPCOrders_SettlementWithWindowStream) (err error) {
	ctx := stream.Context()
	defer mon.Task()(&ctx)(&err)

	if !endpoint.config.AcceptOrders {
		return rpcstatus.Error(rpcstatus.Unavailable, "orders endpoint is unavailable. try again later.")
	}

	var alreadyProcessed bool
	var status pb.SettlementWithWindowResponse_Status
	defer trackFinalStatus(status)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		endpoint.log.Debug("err peer identity from context", zap.Error(err))
		return rpcstatus.Error(rpcstatus.Unauthenticated, err.Error())
	}

	skipSignatures := false
	if endpoint.config.TrustedOrders {
		if node, err := endpoint.overlay.CachedGet(ctx, peer.ID); err == nil {
			if tag, err := node.Tags.FindBySignerAndName(trust.TrustedOperatorSigner, "trusted_orders"); err == nil {
				skipSignatures = string(tag.Value) == "true"
			}
		}
	}
	mon.BoolVal("skip_signatures").Observe(skipSignatures)

	versionAtLeast, err := endpoint.nodeAPIVersionDB.VersionAtLeast(ctx, peer.ID, nodeapiversion.HasWindowedOrders)
	if err != nil {
		endpoint.log.Info("could not query if node version was new enough", zap.Error(err))
		versionAtLeast = false
	}
	if !versionAtLeast {
		err = endpoint.nodeAPIVersionDB.UpdateVersionAtLeast(ctx, peer.ID, nodeapiversion.HasWindowedOrders)
		if err != nil {
			return rpcstatus.Wrap(rpcstatus.Internal, err)
		}
	}

	log := endpoint.log.Named(peer.ID.String())
	log.Debug("SettlementWithWindow")

	type bandwidthAmount struct {
		Settled int64
		Dead    int64
	}

	storagenodeSettled := map[int32]int64{}
	bucketSettled := map[bucketIDAction]bandwidthAmount{}
	seenSerials := map[storj.SerialNumber]struct{}{}

	var window int64
	var request *pb.SettlementRequest
	var receivedCount int
	for {
		request, err = stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			log.Debug("err streaming order request", zap.Error(err))
			return rpcstatus.Error(rpcstatus.Unknown, err.Error())
		}
		receivedCount++

		orderLimit := request.Limit
		if orderLimit == nil {
			log.Debug("request.OrderLimit is nil")
			continue
		}
		if window == 0 {
			window = date.TruncateToHourInNano(orderLimit.OrderCreation)
		}
		order := request.Order
		if order == nil {
			log.Debug("request.Order is nil")
			continue
		}
		serialNum := order.SerialNumber

		// don't process orders that aren't valid
		if !endpoint.isValid(ctx, log, order, orderLimit, peer.ID, window, skipSignatures) {
			continue
		}

		// don't process orders with serial numbers we've already seen
		if _, ok := seenSerials[serialNum]; ok {
			log.Debug("seen serial", zap.String("serial number", serialNum.String()))
			continue
		}
		seenSerials[serialNum] = struct{}{}

		storagenodeSettled[int32(orderLimit.Action)] += order.Amount

		// user can do only two actions which are important for bucket bandwidth usage
		userAction := orderLimit.Action == pb.PieceAction_PUT || orderLimit.Action == pb.PieceAction_GET

		// don't store anything else than user actions in bucket_bandwidth_rollups table. amounts for other
		// actions will be stored in storagenode_bandwidth_rollups.
		if !userAction {
			continue
		}

		metadata, err := endpoint.ordersService.DecryptOrderMetadata(ctx, orderLimit)
		if err != nil {
			log.Debug("decrypt order metadata err:", zap.Error(err))
			mon.Event("bucketinfo_from_orders_metadata_error_1")
			continue
		}

		var bucketInfo metabase.BucketLocation
		switch {
		case len(metadata.CompactProjectBucketPrefix) > 0:
			bucketInfo, err = metabase.ParseCompactBucketPrefix(metadata.GetCompactProjectBucketPrefix())
			if err != nil {
				log.Debug("decrypt order: ParseCompactBucketPrefix", zap.Error(err))
				mon.Event("bucketinfo_from_orders_metadata_error_compact")
				continue
			}
		case len(metadata.ProjectBucketPrefix) > 0:
			bucketInfo, err = metabase.ParseBucketPrefix(metabase.BucketPrefix(metadata.GetProjectBucketPrefix()))
			if err != nil {
				log.Debug("decrypt order: ParseBucketPrefix", zap.Error(err))
				mon.Event("bucketinfo_from_orders_metadata_error_uncompact")
				continue
			}
		default:
			log.Debug("decrypt order: project bucket prefix missing", zap.Error(err))
			mon.Event("bucketinfo_from_orders_metadata_error_default")
			continue
		}

		// log error only for orders created by users, for satellite actions order limits are created
		// without bucket name and project ID because segments loop doesn't have access to it
		if bucketInfo.BucketName == "" || bucketInfo.ProjectID.IsZero() {
			log.Warn("decrypt order: bucketName or projectID not set",
				zap.Stringer("bucketName", bucketInfo.BucketName),
				zap.String("projectID", bucketInfo.ProjectID.String()),
			)
			mon.Event("bucketinfo_from_orders_metadata_error_3")
			continue
		}

		currentBucketIDAction := bucketIDAction{
			projectID:  bucketInfo.ProjectID,
			bucketname: string(bucketInfo.BucketName),
			action:     orderLimit.Action,
		}
		bucketSettled[currentBucketIDAction] = bandwidthAmount{
			Settled: bucketSettled[currentBucketIDAction].Settled + order.Amount,
			Dead:    bucketSettled[currentBucketIDAction].Dead + orderLimit.Limit - order.Amount,
			// we are not collecting Allocated bandwidth as it won't be stored with UpdateBucketBandwidthSettle
		}
	}

	if len(storagenodeSettled) == 0 {
		log.Debug("no orders were successfully processed", zap.Int("received count", receivedCount))
		status = pb.SettlementWithWindowResponse_REJECTED
		return stream.SendAndClose(&pb.SettlementWithWindowResponse{
			Status:        status,
			ActionSettled: storagenodeSettled,
		})
	}
	status, alreadyProcessed, err = endpoint.DB.UpdateStoragenodeBandwidthSettleWithWindow(
		ctx, peer.ID, storagenodeSettled, time.Unix(0, window),
	)
	if err != nil {
		log.Debug("err updating storagenode bandwidth settle", zap.Error(err))
		return err
	}
	log.Debug("orders processed",
		zap.Int("total orders received", receivedCount),
		zap.Time("window", time.Unix(0, window)),
		zap.String("status", status.String()),
	)

	if status == pb.SettlementWithWindowResponse_ACCEPTED && !alreadyProcessed {
		for bucketIDAction, bwAmount := range bucketSettled {
			err = endpoint.DB.UpdateBucketBandwidthSettle(ctx,
				bucketIDAction.projectID, []byte(bucketIDAction.bucketname), bucketIDAction.action, bwAmount.Settled, bwAmount.Dead, time.Unix(0, window),
			)
			if err != nil {
				log.Info("err updating bucket bandwidth settle", zap.Error(err))
			}
		}
	} else {
		mon.Event("orders_already_processed")
	}

	if status == pb.SettlementWithWindowResponse_REJECTED {
		storagenodeSettled = map[int32]int64{}
	}
	return stream.SendAndClose(&pb.SettlementWithWindowResponse{
		Status:        status,
		ActionSettled: storagenodeSettled,
	})
}

func (endpoint *Endpoint) isValid(ctx context.Context, log *zap.Logger, order *pb.Order,
	orderLimit *pb.OrderLimit, peerID storj.NodeID, window int64, skipSignatures bool) bool {
	if orderLimit.StorageNodeId != peerID {
		log.Debug("storage node id mismatch")
		mon.Event("order_not_valid_storagenodeid")
		return false
	}
	// check expiration first before the signatures so that we can throw out the large amount
	// of expired orders being sent to us before doing expensive signature verification.
	if orderLimit.OrderExpiration.Before(time.Now().UTC()) {
		log.Debug("invalid settlement: order limit expired")
		mon.Event("order_not_valid_expired")
		return false
	}
	if !skipSignatures {
		// satellite verifies that it signed the order limit
		if err := signing.VerifyOrderLimitSignature(ctx, endpoint.satelliteSignee, orderLimit); err != nil {
			log.Debug("invalid settlement: unable to verify order limit")
			mon.Event("order_not_valid_satellite_signature")
			return false
		}
		// satellite verifies that the order signature matches pub key in order limit
		if err := signing.VerifyUplinkOrderSignature(ctx, orderLimit.UplinkPublicKey, order); err != nil {
			log.Debug("invalid settlement: unable to verify order")
			mon.Event("order_not_valid_uplink_signature")
			return false
		}
	}
	if orderLimit.SerialNumber != order.SerialNumber {
		log.Debug("invalid settlement: invalid serial number")
		mon.Event("order_not_valid_serialnum_mismatch")
		return false
	}
	// verify the 1 hr windows match
	if window != date.TruncateToHourInNano(orderLimit.OrderCreation) {
		log.Debug("invalid settlement: window mismatch")
		mon.Event("order_not_valid_window_mismatch")
		return false
	}
	if orderLimit.Limit < order.Amount {
		log.Debug("invalid settlement: amounts mismatch")
		mon.Event("order_not_valid_amounts_mismatch")
		return false
	}
	return true
}

// TestingSetAcceptOrdersValid sets endpoint acceptOrders to the provided value. Used only for testing.
func (endpoint *Endpoint) TestingSetAcceptOrdersValid(acceptOrders bool) {
	endpoint.config.AcceptOrders = acceptOrders
}
