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
	"storj.io/storj/satellite/metainfo/metabase"
	"storj.io/storj/satellite/nodeapiversion"
)

// DB implements saving order after receiving from storage node
//
// architecture: Database
type DB interface {
	// CreateSerialInfo creates serial number entry in database.
	CreateSerialInfo(ctx context.Context, serialNumber storj.SerialNumber, bucketID []byte, limitExpiration time.Time) error
	// UseSerialNumber creates a used serial number entry in database from an
	// existing serial number.
	// It returns the bucket ID associated to serialNumber.
	UseSerialNumber(ctx context.Context, serialNumber storj.SerialNumber, storageNodeID storj.NodeID) ([]byte, error)
	// UnuseSerialNumber removes pair serial number -> storage node id from database
	UnuseSerialNumber(ctx context.Context, serialNumber storj.SerialNumber, storageNodeID storj.NodeID) error
	// DeleteExpiredSerials deletes all expired serials in serial_number, used_serials, and consumed_serials table.
	DeleteExpiredSerials(ctx context.Context, now time.Time) (_ int, err error)
	// DeleteExpiredConsumedSerials deletes all expired serials in the consumed_serials table.
	DeleteExpiredConsumedSerials(ctx context.Context, now time.Time) (_ int, err error)
	// GetBucketIDFromSerialNumber returns the bucket ID associated with the serial number
	GetBucketIDFromSerialNumber(ctx context.Context, serialNumber storj.SerialNumber) ([]byte, error)

	// UpdateBucketBandwidthAllocation updates 'allocated' bandwidth for given bucket
	UpdateBucketBandwidthAllocation(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) error
	// UpdateBucketBandwidthSettle updates 'settled' bandwidth for given bucket
	UpdateBucketBandwidthSettle(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) error
	// UpdateBucketBandwidthInline updates 'inline' bandwidth for given bucket
	UpdateBucketBandwidthInline(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) error

	// UpdateStoragenodeBandwidthSettle updates 'settled' bandwidth for given storage node
	UpdateStoragenodeBandwidthSettle(ctx context.Context, storageNode storj.NodeID, action pb.PieceAction, amount int64, intervalStart time.Time) error
	// UpdateStoragenodeBandwidthSettleWithWindow updates 'settled' bandwidth for given storage node
	UpdateStoragenodeBandwidthSettleWithWindow(ctx context.Context, storageNodeID storj.NodeID, actionAmounts map[int32]int64, window time.Time) (status pb.SettlementWithWindowResponse_Status, alreadyProcessed bool, err error)

	// GetBucketBandwidth gets total bucket bandwidth from period of time
	GetBucketBandwidth(ctx context.Context, projectID uuid.UUID, bucketName []byte, from, to time.Time) (int64, error)
	// GetStorageNodeBandwidth gets total storage node bandwidth from period of time
	GetStorageNodeBandwidth(ctx context.Context, nodeID storj.NodeID, from, to time.Time) (int64, error)

	// ProcessOrders takes a list of order requests and processes them in a batch
	ProcessOrders(ctx context.Context, requests []*ProcessOrderRequest) (responses []*ProcessOrderResponse, err error)

	// WithTransaction runs the callback and provides it with a Transaction.
	WithTransaction(ctx context.Context, cb func(ctx context.Context, tx Transaction) error) error
	// WithQueue runs the callback and provides it with a Queue. When the callback returns with
	// no error, any pending serials returned by the queue are removed from it.
	WithQueue(ctx context.Context, cb func(ctx context.Context, queue Queue) error) error
}

// Transaction represents a database transaction but with higher level actions.
type Transaction interface {
	// UpdateBucketBandwidthBatch updates all the bandwidth rollups in the database
	UpdateBucketBandwidthBatch(ctx context.Context, intervalStart time.Time, rollups []BucketBandwidthRollup) error

	// UpdateStoragenodeBandwidthBatch updates all the bandwidth rollups in the database
	UpdateStoragenodeBandwidthBatch(ctx context.Context, intervalStart time.Time, rollups []StoragenodeBandwidthRollup) error

	// CreateConsumedSerialsBatch creates the batch of ConsumedSerials.
	CreateConsumedSerialsBatch(ctx context.Context, consumedSerials []ConsumedSerial) (err error)

	// HasConsumedSerial returns true if the node and serial number have been consumed.
	HasConsumedSerial(ctx context.Context, nodeID storj.NodeID, serialNumber storj.SerialNumber) (bool, error)
}

// Queue is an abstraction around a queue of pending serials.
type Queue interface {
	// GetPendingSerialsBatch returns a batch of pending serials containing at most size
	// entries. It returns a boolean indicating true if the queue is empty.
	GetPendingSerialsBatch(ctx context.Context, size int) ([]PendingSerial, bool, error)
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
	Error = errs.Class("orders error")
	// ErrUsingSerialNumber error class for serial number.
	ErrUsingSerialNumber = errs.Class("serial number")

	errExpiredOrder = errs.Class("order limit expired")

	mon = monkit.Package()
)

// BucketBandwidthRollup contains all the info needed for a bucket bandwidth rollup.
type BucketBandwidthRollup struct {
	ProjectID  uuid.UUID
	BucketName string
	Action     pb.PieceAction
	Inline     int64
	Allocated  int64
	Settled    int64
}

// SortBucketBandwidthRollups sorts the rollups.
func SortBucketBandwidthRollups(rollups []BucketBandwidthRollup) {
	sort.SliceStable(rollups, func(i, j int) bool {
		uuidCompare := bytes.Compare(rollups[i].ProjectID[:], rollups[j].ProjectID[:])
		switch {
		case uuidCompare == -1:
			return true
		case uuidCompare == 1:
			return false
		case rollups[i].BucketName < rollups[j].BucketName:
			return true
		case rollups[i].BucketName > rollups[j].BucketName:
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

// ProcessOrderRequest for batch order processing.
type ProcessOrderRequest struct {
	Order      *pb.Order
	OrderLimit *pb.OrderLimit
}

// ProcessOrderResponse for batch order processing responses.
type ProcessOrderResponse struct {
	SerialNumber storj.SerialNumber
	Status       pb.SettlementResponse_Status
}

// Endpoint for orders receiving
//
// architecture: Endpoint
type Endpoint struct {
	log                        *zap.Logger
	satelliteSignee            signing.Signee
	DB                         DB
	nodeAPIVersionDB           nodeapiversion.DB
	settlementBatchSize        int
	windowEndpointRolloutPhase WindowEndpointRolloutPhase
	ordersSemaphore            chan struct{}
}

// NewEndpoint new orders receiving endpoint.
//
// ordersSemaphoreSize controls the number of concurrent clients allowed to submit orders at once.
// A value of zero means unlimited.
func NewEndpoint(log *zap.Logger, satelliteSignee signing.Signee, db DB, nodeAPIVersionDB nodeapiversion.DB, settlementBatchSize int, windowEndpointRolloutPhase WindowEndpointRolloutPhase, ordersSemaphoreSize int) *Endpoint {
	var ordersSemaphore chan struct{}
	if ordersSemaphoreSize > 0 {
		ordersSemaphore = make(chan struct{}, ordersSemaphoreSize)
	}

	return &Endpoint{
		log:                        log,
		satelliteSignee:            satelliteSignee,
		DB:                         db,
		nodeAPIVersionDB:           nodeAPIVersionDB,
		settlementBatchSize:        settlementBatchSize,
		windowEndpointRolloutPhase: windowEndpointRolloutPhase,
		ordersSemaphore:            ordersSemaphore,
	}
}

func monitoredSettlementStreamReceive(ctx context.Context, stream pb.DRPCOrders_SettlementStream) (_ *pb.SettlementRequest, err error) {
	defer mon.Task()(&ctx)(&err)
	return stream.Recv()
}

func monitoredSettlementStreamSend(ctx context.Context, stream pb.DRPCOrders_SettlementStream, resp *pb.SettlementResponse) (err error) {
	defer mon.Task()(&ctx)(&err)
	switch resp.Status {
	case pb.SettlementResponse_ACCEPTED:
		mon.Event("settlement_response_accepted")
	case pb.SettlementResponse_REJECTED:
		mon.Event("settlement_response_rejected")
	default:
		mon.Event("settlement_response_unknown")
	}
	return stream.Send(resp)
}

// withOrdersSemaphore acquires a slot with the ordersSemaphore if one exists and returns
// a function to exit it. If the context expires, it returns an error.
func (endpoint *Endpoint) withOrdersSemaphore(ctx context.Context, cb func(ctx context.Context) error) error {
	if endpoint.ordersSemaphore == nil {
		return cb(ctx)
	}
	select {
	case endpoint.ordersSemaphore <- struct{}{}:
		err := cb(ctx)
		<-endpoint.ordersSemaphore
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Settlement receives orders and handles them in batches.
func (endpoint *Endpoint) Settlement(stream pb.DRPCOrders_SettlementStream) (err error) {
	ctx := stream.Context()
	defer mon.Task()(&ctx)(&err)

	switch endpoint.windowEndpointRolloutPhase {
	case WindowEndpointRolloutPhase1:
	case WindowEndpointRolloutPhase2, WindowEndpointRolloutPhase3:
		return rpcstatus.Error(rpcstatus.Unavailable, "endpoint disabled")
	default:
		return rpcstatus.Error(rpcstatus.Internal, "invalid window endpoint rollout phase")
	}

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return rpcstatus.Error(rpcstatus.Unauthenticated, err.Error())
	}

	formatError := func(err error) error {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return rpcstatus.Error(rpcstatus.Unknown, err.Error())
	}

	log := endpoint.log.Named(peer.ID.String())
	log.Debug("Settlement")

	requests := make([]*ProcessOrderRequest, 0, endpoint.settlementBatchSize)

	defer func() {
		if len(requests) > 0 {
			err = errs.Combine(err, endpoint.processOrders(ctx, stream, requests))
			if err != nil {
				err = formatError(err)
			}
		}
	}()

	var expirationCount int64
	defer func() {
		if expirationCount > 0 {
			log.Debug("order verification found expired orders", zap.Int64("amount", expirationCount))
		}
	}()

	for {
		request, err := monitoredSettlementStreamReceive(ctx, stream)
		if err != nil {
			return formatError(err)
		}

		if request == nil {
			return rpcstatus.Error(rpcstatus.InvalidArgument, "request missing")
		}
		if request.Limit == nil {
			return rpcstatus.Error(rpcstatus.InvalidArgument, "order limit missing")
		}
		if request.Order == nil {
			return rpcstatus.Error(rpcstatus.InvalidArgument, "order missing")
		}

		orderLimit := request.Limit
		order := request.Order

		rejectErr := func() error {
			if orderLimit.StorageNodeId != peer.ID {
				return rpcstatus.Error(rpcstatus.Unauthenticated, "only specified storage node can settle order")
			}

			// check expiration first before the signatures so that we can throw out the large
			// amount of expired orders being sent to us before doing expensive signature
			// verification.
			if orderLimit.OrderExpiration.Before(time.Now()) {
				mon.Event("order_verification_failed_expired")
				expirationCount++
				return errExpiredOrder.New("order limit expired")
			}

			// satellite verifies that it signed the order limit
			if err := signing.VerifyOrderLimitSignature(ctx, endpoint.satelliteSignee, orderLimit); err != nil {
				mon.Event("order_verification_failed_satellite_signature")
				return Error.New("unable to verify order limit")
			}

			// satellite verifies that the order signature matches pub key in order limit
			if err := signing.VerifyUplinkOrderSignature(ctx, orderLimit.UplinkPublicKey, order); err != nil {
				mon.Event("order_verification_failed_uplink_signature")
				return Error.New("unable to verify order")
			}

			// TODO should this reject or just error ??
			if orderLimit.SerialNumber != order.SerialNumber {
				mon.Event("order_verification_failed_serial_mismatch")
				return Error.New("invalid serial number")
			}
			return nil
		}()
		if rejectErr != nil {
			mon.Event("order_verification_failed")
			if !errExpiredOrder.Has(rejectErr) {
				log.Debug("order limit/order verification failed", zap.Stringer("serial", orderLimit.SerialNumber), zap.Error(rejectErr))
			}
			err := monitoredSettlementStreamSend(ctx, stream, &pb.SettlementResponse{
				SerialNumber: orderLimit.SerialNumber,
				Status:       pb.SettlementResponse_REJECTED,
			})
			if err != nil {
				return formatError(err)
			}
			continue
		}

		requests = append(requests, &ProcessOrderRequest{Order: order, OrderLimit: orderLimit})

		if len(requests) >= endpoint.settlementBatchSize {
			err = endpoint.processOrders(ctx, stream, requests)
			requests = requests[:0]
			if err != nil {
				return formatError(err)
			}
		}
	}
}

func (endpoint *Endpoint) processOrders(ctx context.Context, stream pb.DRPCOrders_SettlementStream, requests []*ProcessOrderRequest) (err error) {
	defer mon.Task()(&ctx)(&err)

	var responses []*ProcessOrderResponse
	err = endpoint.withOrdersSemaphore(ctx, func(ctx context.Context) error {
		responses, err = endpoint.DB.ProcessOrders(ctx, requests)
		return err
	})
	if err != nil {
		return err
	}

	for _, response := range responses {
		r := &pb.SettlementResponse{
			SerialNumber: response.SerialNumber,
			Status:       response.Status,
		}
		err = monitoredSettlementStreamSend(ctx, stream, r)
		if err != nil {
			return err
		}
	}

	return nil
}

type bucketIDAction struct {
	bucketname string
	projectID  uuid.UUID
	action     pb.PieceAction
}

// SettlementWithWindow processes all orders that were created in a 1 hour window.
// Only one window is processed at a time.
// Batches are atomic, all orders are settled successfully or they all fail.
func (endpoint *Endpoint) SettlementWithWindow(stream pb.DRPCOrders_SettlementWithWindowStream) (err error) {
	switch endpoint.windowEndpointRolloutPhase {
	case WindowEndpointRolloutPhase1, WindowEndpointRolloutPhase2:
		return endpoint.SettlementWithWindowMigration(stream)
	case WindowEndpointRolloutPhase3:
		return endpoint.SettlementWithWindowFinal(stream)
	default:
		return rpcstatus.Error(rpcstatus.Internal, "invalid window endpoint rollout phase")
	}
}

// SettlementWithWindowMigration implements phase 1 and phase 2 of the windowed order rollout where
// it uses the same backend as the non-windowed settlement and inserts entries containing 0 for
// the window which ensures that it is either entirely handled by the queue or entirely handled by
// the phase 3 endpoint.
func (endpoint *Endpoint) SettlementWithWindowMigration(stream pb.DRPCOrders_SettlementWithWindowStream) (err error) {
	ctx := stream.Context()
	defer mon.Task()(&ctx)(&err)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		endpoint.log.Debug("err peer identity from context", zap.Error(err))
		return rpcstatus.Error(rpcstatus.Unauthenticated, err.Error())
	}

	// update the node api version inside of the semaphore
	err = endpoint.withOrdersSemaphore(ctx, func(ctx context.Context) error {
		return endpoint.nodeAPIVersionDB.UpdateVersionAtLeast(ctx, peer.ID, nodeapiversion.HasWindowedOrders)
	})
	if err != nil {
		return rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	log := endpoint.log.Named(peer.ID.String())
	log.Debug("SettlementWithWindow")

	var receivedCount int
	var window int64
	var actions = map[pb.PieceAction]struct{}{}
	var requests []*ProcessOrderRequest
	var finished bool

	for !finished {
		requests = requests[:0]

		for len(requests) < endpoint.settlementBatchSize {
			request, err := stream.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					finished = true
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

			order := request.Order
			if order == nil {
				log.Debug("request.Order is nil")
				continue
			}

			if window == 0 {
				window = date.TruncateToHourInNano(orderLimit.OrderCreation)
			}

			// don't process orders that aren't valid
			if !endpoint.isValid(ctx, log, order, orderLimit, peer.ID, window) {
				continue
			}

			actions[orderLimit.Action] = struct{}{}

			requests = append(requests, &ProcessOrderRequest{
				Order:      order,
				OrderLimit: orderLimit,
			})
		}

		// process all of the orders in the old way inside of the semaphore
		err := endpoint.withOrdersSemaphore(ctx, func(ctx context.Context) error {
			_, err = endpoint.DB.ProcessOrders(ctx, requests)
			return err
		})
		if err != nil {
			return rpcstatus.Wrap(rpcstatus.Internal, err)
		}
	}

	// if we received no valid orders, then respond with rejected
	if len(actions) == 0 || window == 0 {
		return stream.SendAndClose(&pb.SettlementWithWindowResponse{
			Status: pb.SettlementWithWindowResponse_REJECTED,
		})
	}

	// insert zero rows for every action involved in the set of orders. this prevents
	// many problems (double spends and underspends) by ensuring that any window is
	// either handled entirely by the queue or entirely with the phase 3 windowed endpoint.
	// enter the semaphore for the duration of the updates.

	windowTime := time.Unix(0, window)
	err = endpoint.withOrdersSemaphore(ctx, func(ctx context.Context) error {
		for action := range actions {
			if err := endpoint.DB.UpdateStoragenodeBandwidthSettle(ctx, peer.ID, action, 0, windowTime); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	log.Debug("orders processed",
		zap.Int("total orders received", receivedCount),
		zap.Time("window", windowTime),
	)

	return stream.SendAndClose(&pb.SettlementWithWindowResponse{
		Status: pb.SettlementWithWindowResponse_ACCEPTED,
	})
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

	var alreadyProcessed bool
	var status pb.SettlementWithWindowResponse_Status
	defer trackFinalStatus(status)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		endpoint.log.Debug("err peer identity from context", zap.Error(err))
		return rpcstatus.Error(rpcstatus.Unauthenticated, err.Error())
	}

	err = endpoint.nodeAPIVersionDB.UpdateVersionAtLeast(ctx, peer.ID, nodeapiversion.HasWindowedOrders)
	if err != nil {
		return rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	log := endpoint.log.Named(peer.ID.String())
	log.Debug("SettlementWithWindow")

	var storagenodeSettled = map[int32]int64{}
	var bucketSettled = map[bucketIDAction]int64{}
	var seenSerials = map[storj.SerialNumber]struct{}{}

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
		if !endpoint.isValid(ctx, log, order, orderLimit, peer.ID, window) {
			continue
		}

		// don't process orders with serial numbers we've already seen
		if _, ok := seenSerials[serialNum]; ok {
			log.Debug("seen serial", zap.String("serial number", serialNum.String()))
			continue
		}
		seenSerials[serialNum] = struct{}{}

		storagenodeSettled[int32(orderLimit.Action)] += order.Amount

		bucketPrefix, err := endpoint.DB.GetBucketIDFromSerialNumber(ctx, serialNum)
		if err != nil {
			log.Info("get bucketPrefix from serial number table err", zap.Error(err))
			continue
		}
		bucket, err := metabase.ParseBucketPrefix(metabase.BucketPrefix(bucketPrefix))
		if err != nil {
			log.Info("split bucket err", zap.Error(err), zap.String("bucketPrefix", string(bucketPrefix)))
			continue
		}
		bucketSettled[bucketIDAction{
			bucketname: bucket.BucketName,
			projectID:  bucket.ProjectID,
			action:     orderLimit.Action,
		}] += order.Amount
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
		for bucketIDAction, amount := range bucketSettled {
			err = endpoint.DB.UpdateBucketBandwidthSettle(ctx,
				bucketIDAction.projectID, []byte(bucketIDAction.bucketname), bucketIDAction.action, amount, time.Unix(0, window),
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

func (endpoint *Endpoint) isValid(ctx context.Context, log *zap.Logger, order *pb.Order, orderLimit *pb.OrderLimit, peerID storj.NodeID, window int64) bool {
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
	return true
}
