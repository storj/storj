// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"context"
	mathrand "math/rand"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
)

var (
	// ErrDownloadFailedNotEnoughPieces is returned when download failed due to missing pieces.
	ErrDownloadFailedNotEnoughPieces = errs.Class("not enough pieces for download")
	// ErrDecryptOrderMetadata is returned when a step of decrypting metadata fails.
	ErrDecryptOrderMetadata = errs.Class("decrytping order metadata")
)

// Config is a configuration struct for orders Service.
type Config struct {
	EncryptionKeys      EncryptionKeys `help:"encryption keys to encrypt info in orders" default:""`
	Expiration          time.Duration  `help:"how long until an order expires" default:"24h" testDefault:"168h"` // default is 1 day
	FlushBatchSize      int            `help:"how many items in the rollups write cache before they are flushed to the database" devDefault:"20" releaseDefault:"1000" testDefault:"10"`
	FlushInterval       time.Duration  `help:"how often to flush the rollups write cache to the database" devDefault:"30s" releaseDefault:"1m" testDefault:"$TESTINTERVAL"`
	NodeStatusLogging   bool           `hidden:"true" help:"deprecated, log the offline/disqualification status of nodes" default:"false" testDefault:"true"`
	OrdersSemaphoreSize int            `help:"how many concurrent orders to process at once. zero is unlimited" default:"2"`
}

// Overlay defines the overlay dependency of orders.Service.
// use `go install github.com/golang/mock/mockgen@v1.6.0` if missing
//
//go:generate mockgen -destination mock_test.go -package orders . OverlayForOrders
type Overlay interface {
	CachedGetOnlineNodesForGet(context.Context, []storj.NodeID) (map[storj.NodeID]*nodeselection.SelectedNode, error)
	GetOnlineNodesForAuditRepair(context.Context, []storj.NodeID) (map[storj.NodeID]*overlay.NodeReputation, error)
	Get(ctx context.Context, nodeID storj.NodeID) (*overlay.NodeDossier, error)
	IsOnline(node *overlay.NodeDossier) bool
}

// Service for creating order limits.
//
// architecture: Service
type Service struct {
	log            *zap.Logger
	satellite      signing.Signer
	overlay        Overlay
	orders         DB
	placementRules overlay.PlacementRules

	encryptionKeys EncryptionKeys

	orderExpiration time.Duration

	rngMu sync.Mutex
	rng   *mathrand.Rand
}

// NewService creates new service for creating order limits.
func NewService(
	log *zap.Logger, satellite signing.Signer, overlay Overlay,
	orders DB, placementRules overlay.PlacementRules, config Config,
) (*Service, error) {
	if config.EncryptionKeys.Default.IsZero() {
		return nil, Error.New("encryption keys must be specified to include encrypted metadata")
	}

	return &Service{
		log:            log,
		satellite:      satellite,
		overlay:        overlay,
		orders:         orders,
		placementRules: placementRules,

		encryptionKeys: config.EncryptionKeys,

		orderExpiration: config.Expiration,

		rng: mathrand.New(mathrand.NewSource(time.Now().UnixNano())),
	}, nil
}

// VerifyOrderLimitSignature verifies that the signature inside order limit belongs to the satellite.
func (service *Service) VerifyOrderLimitSignature(ctx context.Context, signed *pb.OrderLimit) (err error) {
	defer mon.Task()(&ctx)(&err)
	return signing.VerifyOrderLimitSignature(ctx, service.satellite, signed)
}

func (service *Service) updateBandwidth(ctx context.Context, bucket metabase.BucketLocation, addressedOrderLimits ...*pb.AddressedOrderLimit) (err error) {
	defer mon.Task()(&ctx)(&err)
	if len(addressedOrderLimits) == 0 {
		return nil
	}

	var action pb.PieceAction

	var bucketAllocation int64

	for _, addressedOrderLimit := range addressedOrderLimits {
		if addressedOrderLimit != nil && addressedOrderLimit.Limit != nil {
			orderLimit := addressedOrderLimit.Limit
			action = orderLimit.Action
			bucketAllocation += orderLimit.Limit
		}
	}

	now := time.Now().UTC()
	intervalStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())

	// TODO: all of this below should be a single db transaction. in fact, this whole function should probably be part of an existing transaction
	if err := service.orders.UpdateBucketBandwidthAllocation(ctx, bucket.ProjectID, []byte(bucket.BucketName), action, bucketAllocation, intervalStart); err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// CreateGetOrderLimits creates the order limits for downloading the pieces of a segment.
func (service *Service) CreateGetOrderLimits(ctx context.Context, bucket metabase.BucketLocation, segment metabase.Segment, desiredNodes int32, overrideLimit int64) (_ []*pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, err error) {
	defer mon.Task()(&ctx)(&err)

	orderLimit := segment.PieceSize()
	if overrideLimit > 0 && overrideLimit < orderLimit {
		orderLimit = overrideLimit
	}

	nodeIDs := make([]storj.NodeID, len(segment.Pieces))
	for i, piece := range segment.Pieces {
		nodeIDs[i] = piece.StorageNode
	}

	nodes, err := service.overlay.CachedGetOnlineNodesForGet(ctx, nodeIDs)
	if err != nil {
		service.log.Debug("error getting nodes from overlay", zap.Error(err))
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	filter := service.placementRules(segment.Placement)
	for id, node := range nodes {
		if !filter.Match(node) {
			delete(nodes, id)
		}
	}

	signer, err := NewSignerGet(service, segment.RootPieceID, time.Now(), orderLimit, bucket)
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	neededLimits := segment.Redundancy.DownloadNodes()
	if desiredNodes > neededLimits {
		neededLimits = desiredNodes
	}

	pieces := segment.Pieces
	for _, pieceIndex := range service.perm(len(pieces)) {
		piece := pieces[pieceIndex]
		node, ok := nodes[piece.StorageNode]
		if !ok {
			continue
		}

		_, err := signer.Sign(ctx, resolveStorageNode_Selected(node, true), int32(piece.Number))
		if err != nil {
			return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
		}

		if len(signer.AddressedLimits) >= int(neededLimits) {
			break
		}
	}
	if len(signer.AddressedLimits) < int(segment.Redundancy.RequiredShares) {
		mon.Meter("download_failed_not_enough_pieces_uplink").Mark(1) //mon:locked
		return nil, storj.PiecePrivateKey{}, ErrDownloadFailedNotEnoughPieces.New("not enough orderlimits: got %d, required %d", len(signer.AddressedLimits), segment.Redundancy.RequiredShares)
	}

	if err := service.updateBandwidth(ctx, bucket, signer.AddressedLimits...); err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	signer.AddressedLimits, err = sortLimits(signer.AddressedLimits, segment)
	if err != nil {
		return nil, storj.PiecePrivateKey{}, err
	}
	// workaround to avoid sending nil values on top level
	for i := range signer.AddressedLimits {
		if signer.AddressedLimits[i] == nil {
			signer.AddressedLimits[i] = &pb.AddressedOrderLimit{}
		}
	}

	return signer.AddressedLimits, signer.PrivateKey, nil
}

func (service *Service) perm(n int) []int {
	service.rngMu.Lock()
	defer service.rngMu.Unlock()
	return service.rng.Perm(n)
}

// sortLimits sorts order limits and fill missing ones with nil values.
func sortLimits(limits []*pb.AddressedOrderLimit, segment metabase.Segment) ([]*pb.AddressedOrderLimit, error) {
	sorted := make([]*pb.AddressedOrderLimit, segment.Redundancy.TotalShares)
	for _, piece := range segment.Pieces {
		if int16(piece.Number) >= segment.Redundancy.TotalShares {
			return nil, Error.New("piece number is greater than redundancy total shares: got %d, max %d",
				piece.Number, (segment.Redundancy.TotalShares - 1))
		}
		sorted[piece.Number] = getLimitByStorageNodeID(limits, piece.StorageNode)
	}
	return sorted, nil
}

func getLimitByStorageNodeID(limits []*pb.AddressedOrderLimit, storageNodeID storj.NodeID) *pb.AddressedOrderLimit {
	for _, limit := range limits {
		if limit == nil || limit.GetLimit() == nil {
			continue
		}

		if limit.GetLimit().StorageNodeId == storageNodeID {
			return limit
		}
	}
	return nil
}

// CreatePutOrderLimits creates the order limits for uploading pieces to nodes.
func (service *Service) CreatePutOrderLimits(ctx context.Context, bucket metabase.BucketLocation, nodes []*nodeselection.SelectedNode, pieceExpiration time.Time, maxPieceSize int64) (_ storj.PieceID, _ []*pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, err error) {
	defer mon.Task()(&ctx)(&err)

	signer, err := NewSignerPut(service, pieceExpiration, time.Now(), maxPieceSize, bucket)
	if err != nil {
		return storj.PieceID{}, nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	for pieceNum, node := range nodes {
		_, err := signer.Sign(ctx, resolveStorageNode_Selected(node, true), int32(pieceNum))
		if err != nil {
			return storj.PieceID{}, nil, storj.PiecePrivateKey{}, Error.Wrap(err)
		}
	}

	return signer.RootPieceID, signer.AddressedLimits, signer.PrivateKey, nil
}

// ReplacePutOrderLimits replaces order limits for uploading pieces to nodes.
func (service *Service) ReplacePutOrderLimits(ctx context.Context, rootPieceID storj.PieceID, addressedLimits []*pb.AddressedOrderLimit, nodes []*nodeselection.SelectedNode, pieceNumbers []int32) (_ []*pb.AddressedOrderLimit, err error) {
	defer mon.Task()(&ctx)(&err)

	pieceIDDeriver := rootPieceID.Deriver()

	newAddressedLimits := make([]*pb.AddressedOrderLimit, len(addressedLimits))
	copy(newAddressedLimits, addressedLimits)

	for i, pieceNumber := range pieceNumbers {
		if pieceNumber < 0 || int(pieceNumber) >= len(addressedLimits) {
			return nil, Error.New("invalid piece number %d", pieceNumber)
		}

		// TODO: clone?
		newAddressedLimit := *addressedLimits[pieceNumber].Limit
		newAddressedLimit.StorageNodeId = nodes[i].ID
		newAddressedLimit.PieceId = pieceIDDeriver.Derive(nodes[i].ID, pieceNumber)
		newAddressedLimit.SatelliteSignature = nil

		newAddressedLimits[pieceNumber].Limit, err = signing.SignOrderLimit(ctx, service.satellite, &newAddressedLimit)
		if err != nil {
			return nil, ErrSigner.Wrap(err)
		}
		newAddressedLimits[pieceNumber].StorageNodeAddress = resolveStorageNode_Selected(nodes[i], true).Address
	}

	return newAddressedLimits, nil
}

// CreateAuditOrderLimits creates the order limits for auditing the pieces of a segment.
func (service *Service) CreateAuditOrderLimits(ctx context.Context, segment metabase.Segment, skip map[storj.NodeID]bool) (_ []*pb.AddressedOrderLimit, _ storj.PiecePrivateKey, cachedNodesInfo map[storj.NodeID]overlay.NodeReputation, err error) {
	defer mon.Task()(&ctx)(&err)

	nodeIDs := make([]storj.NodeID, len(segment.Pieces))
	for i, piece := range segment.Pieces {
		nodeIDs[i] = piece.StorageNode
	}

	nodes, err := service.overlay.GetOnlineNodesForAuditRepair(ctx, nodeIDs)
	if err != nil {
		service.log.Debug("error getting nodes from overlay", zap.Error(err))
		return nil, storj.PiecePrivateKey{}, nil, Error.Wrap(err)
	}

	bucket := metabase.BucketLocation{}
	signer, err := NewSignerAudit(service, segment.RootPieceID, time.Now(), int64(segment.Redundancy.ShareSize), bucket)
	if err != nil {
		return nil, storj.PiecePrivateKey{}, nil, Error.Wrap(err)
	}

	cachedNodesInfo = make(map[storj.NodeID]overlay.NodeReputation)
	var nodeErrors errs.Group
	var limitsCount int16
	limits := make([]*pb.AddressedOrderLimit, segment.Redundancy.TotalShares)
	for _, piece := range segment.Pieces {
		if skip[piece.StorageNode] {
			continue
		}
		node, ok := nodes[piece.StorageNode]
		if !ok {
			nodeErrors.Add(errs.New("node %q is not reliable", piece.StorageNode))
			continue
		}

		cachedNodesInfo[piece.StorageNode] = *node

		limit, err := signer.Sign(ctx, resolveStorageNode_Reputation(node), int32(piece.Number))
		if err != nil {
			return nil, storj.PiecePrivateKey{}, nil, Error.Wrap(err)
		}

		limits[piece.Number] = limit
		limitsCount++
	}

	if limitsCount < segment.Redundancy.RequiredShares {
		err = ErrDownloadFailedNotEnoughPieces.New("not enough nodes available: got %d, required %d", limitsCount, segment.Redundancy.RequiredShares)
		return nil, storj.PiecePrivateKey{}, nil, errs.Combine(err, nodeErrors.Err())
	}

	return limits, signer.PrivateKey, cachedNodesInfo, nil
}

// CreateAuditOrderLimit creates an order limit for auditing a single piece from a segment.
func (service *Service) CreateAuditOrderLimit(ctx context.Context, nodeID storj.NodeID, pieceNum uint16, rootPieceID storj.PieceID, shareSize int32) (limit *pb.AddressedOrderLimit, _ storj.PiecePrivateKey, nodeInfo *overlay.NodeReputation, err error) {
	// TODO reduce number of params ?
	defer mon.Task()(&ctx)(&err)

	signer, err := NewSignerAudit(service, rootPieceID, time.Now(), int64(shareSize), metabase.BucketLocation{})
	if err != nil {
		return nil, storj.PiecePrivateKey{}, nodeInfo, Error.Wrap(err)
	}
	return service.createAuditOrderLimitWithSigner(ctx, nodeID, pieceNum, signer)
}

// CreateAuditPieceOrderLimit creates an order limit for auditing a single
// piece from a segment, requesting that the original order limit and piece
// hash be included.
//
// Unfortunately, because of the way the protocol works historically, we
// must use GET_REPAIR for this operation instead of GET_AUDIT.
func (service *Service) CreateAuditPieceOrderLimit(ctx context.Context, nodeID storj.NodeID, pieceNum uint16, rootPieceID storj.PieceID, pieceSize int32) (limit *pb.AddressedOrderLimit, _ storj.PiecePrivateKey, nodeInfo *overlay.NodeReputation, err error) {
	defer mon.Task()(&ctx)(&err)

	signer, err := NewSignerRepairGet(service, rootPieceID, time.Now(), int64(pieceSize), metabase.BucketLocation{})
	if err != nil {
		return nil, storj.PiecePrivateKey{}, nodeInfo, Error.Wrap(err)
	}
	return service.createAuditOrderLimitWithSigner(ctx, nodeID, pieceNum, signer)
}

func (service *Service) createAuditOrderLimitWithSigner(ctx context.Context, nodeID storj.NodeID, pieceNum uint16, signer *Signer) (limit *pb.AddressedOrderLimit, _ storj.PiecePrivateKey, nodeInfo *overlay.NodeReputation, err error) {
	defer mon.Task()(&ctx)(&err)

	node, err := service.overlay.Get(ctx, nodeID)
	if err != nil {
		return nil, storj.PiecePrivateKey{}, nil, Error.Wrap(err)
	}

	nodeInfo = &overlay.NodeReputation{
		ID:         nodeID,
		Address:    node.Address,
		LastNet:    node.LastNet,
		LastIPPort: node.LastIPPort,
		Reputation: node.Reputation.Status,
	}

	if node.Disqualified != nil {
		return nil, storj.PiecePrivateKey{}, nodeInfo, overlay.ErrNodeDisqualified.New("%v", nodeID)
	}
	if node.ExitStatus.ExitFinishedAt != nil {
		return nil, storj.PiecePrivateKey{}, nodeInfo, overlay.ErrNodeFinishedGE.New("%v", nodeID)
	}
	if !service.overlay.IsOnline(node) {
		return nil, storj.PiecePrivateKey{}, nodeInfo, overlay.ErrNodeOffline.New("%v", nodeID)
	}

	orderLimit, err := signer.Sign(ctx, resolveStorageNode(&node.Node, node.LastIPPort, false), int32(pieceNum))
	if err != nil {
		return nil, storj.PiecePrivateKey{}, nodeInfo, Error.Wrap(err)
	}

	return orderLimit, signer.PrivateKey, nodeInfo, nil
}

// CreateGetRepairOrderLimits creates the order limits for downloading the
// healthy pieces of segment as the source for repair.
//
// The length of the returned orders slice is the total number of pieces of the
// segment, setting to null the ones which don't correspond to a healthy piece.
func (service *Service) CreateGetRepairOrderLimits(ctx context.Context, segment metabase.Segment, healthy metabase.Pieces) (_ []*pb.AddressedOrderLimit, _ storj.PiecePrivateKey, cachedNodesInfo map[storj.NodeID]overlay.NodeReputation, err error) {
	defer mon.Task()(&ctx)(&err)

	pieceSize := segment.PieceSize()
	totalPieces := segment.Redundancy.TotalShares

	nodeIDs := make([]storj.NodeID, len(segment.Pieces))
	for i, piece := range segment.Pieces {
		nodeIDs[i] = piece.StorageNode
	}

	nodes, err := service.overlay.GetOnlineNodesForAuditRepair(ctx, nodeIDs)
	if err != nil {
		service.log.Debug("error getting nodes from overlay", zap.Error(err))
		return nil, storj.PiecePrivateKey{}, nil, Error.Wrap(err)
	}

	signer, err := NewSignerRepairGet(service, segment.RootPieceID, time.Now(), pieceSize, metabase.BucketLocation{})
	if err != nil {
		return nil, storj.PiecePrivateKey{}, nil, Error.Wrap(err)
	}

	cachedNodesInfo = make(map[storj.NodeID]overlay.NodeReputation, len(healthy))
	var nodeErrors errs.Group
	var limitsCount int
	limits := make([]*pb.AddressedOrderLimit, totalPieces)
	for _, piece := range healthy {
		node, ok := nodes[piece.StorageNode]
		if !ok {
			nodeErrors.Add(errs.New("node %q is not reliable", piece.StorageNode))
			continue
		}

		cachedNodesInfo[piece.StorageNode] = *node

		limit, err := signer.Sign(ctx, resolveStorageNode_Reputation(node), int32(piece.Number))
		if err != nil {
			return nil, storj.PiecePrivateKey{}, nil, Error.Wrap(err)
		}

		limits[piece.Number] = limit
		limitsCount++
	}

	if limitsCount < int(segment.Redundancy.RequiredShares) {
		err = ErrDownloadFailedNotEnoughPieces.New("not enough nodes available: got %d, required %d", limitsCount, segment.Redundancy.RequiredShares)
		return nil, storj.PiecePrivateKey{}, nil, errs.Combine(err, nodeErrors.Err())
	}

	return limits, signer.PrivateKey, cachedNodesInfo, nil
}

// CreatePutRepairOrderLimits creates the order limits for uploading the repaired pieces of segment to newNodes.
func (service *Service) CreatePutRepairOrderLimits(ctx context.Context, segment metabase.Segment, getOrderLimits []*pb.AddressedOrderLimit, healthySet map[uint16]struct{}, newNodes []*nodeselection.SelectedNode) (_ []*pb.AddressedOrderLimit, _ storj.PiecePrivateKey, err error) {
	defer mon.Task()(&ctx)(&err)

	// Create the order limits for being used to upload the repaired pieces
	pieceSize := segment.PieceSize()
	totalPieces := int(segment.Redundancy.TotalShares)

	var numRetrievablePieces int
	for _, o := range getOrderLimits {
		if o != nil {
			numRetrievablePieces++
		}
	}

	limits := make([]*pb.AddressedOrderLimit, totalPieces)

	expirationDate := time.Time{}
	if segment.ExpiresAt != nil {
		expirationDate = *segment.ExpiresAt
	}

	signer, err := NewSignerRepairPut(service, segment.RootPieceID, expirationDate, time.Now(), pieceSize, metabase.BucketLocation{})
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	var pieceNum uint16
	for _, node := range newNodes {
		for int(pieceNum) < totalPieces {
			_, isHealthy := healthySet[pieceNum]
			if !isHealthy {
				break
			}
			pieceNum++
		}

		if int(pieceNum) >= totalPieces { // should not happen
			return nil, storj.PiecePrivateKey{}, Error.New("piece num greater than total pieces: %d >= %d", pieceNum, totalPieces)
		}

		limit, err := signer.Sign(ctx, resolveStorageNode_Selected(node, false), int32(pieceNum))
		if err != nil {
			return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
		}

		limits[pieceNum] = limit
		pieceNum++
	}

	return limits, signer.PrivateKey, nil
}

// CreateGracefulExitPutOrderLimit creates an order limit for graceful exit put transfers.
func (service *Service) CreateGracefulExitPutOrderLimit(ctx context.Context, bucket metabase.BucketLocation, nodeID storj.NodeID, pieceNum int32, rootPieceID storj.PieceID, shareSize int32) (limit *pb.AddressedOrderLimit, _ storj.PiecePrivateKey, err error) {
	defer mon.Task()(&ctx)(&err)

	node, err := service.overlay.Get(ctx, nodeID)
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}
	if node.Disqualified != nil {
		return nil, storj.PiecePrivateKey{}, overlay.ErrNodeDisqualified.New("%v", nodeID)
	}
	if !service.overlay.IsOnline(node) {
		return nil, storj.PiecePrivateKey{}, overlay.ErrNodeOffline.New("%v", nodeID)
	}

	signer, err := NewSignerGracefulExit(service, rootPieceID, time.Now(), shareSize, bucket)
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	limit, err = signer.Sign(ctx, resolveStorageNode(&node.Node, node.LastIPPort, true), pieceNum)
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	return limit, signer.PrivateKey, nil
}

// UpdateGetInlineOrder updates amount of inline GET bandwidth for given bucket.
func (service *Service) UpdateGetInlineOrder(ctx context.Context, bucket metabase.BucketLocation, amount int64) (err error) {
	defer mon.Task()(&ctx)(&err)
	now := time.Now().UTC()
	intervalStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())

	return service.orders.UpdateBucketBandwidthInline(ctx, bucket.ProjectID, []byte(bucket.BucketName), pb.PieceAction_GET, amount, intervalStart)
}

// UpdatePutInlineOrder updates amount of inline PUT bandwidth for given bucket.
func (service *Service) UpdatePutInlineOrder(ctx context.Context, bucket metabase.BucketLocation, amount int64) (err error) {
	defer mon.Task()(&ctx)(&err)
	now := time.Now().UTC()
	intervalStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())

	return service.orders.UpdateBucketBandwidthInline(ctx, bucket.ProjectID, []byte(bucket.BucketName), pb.PieceAction_PUT, amount, intervalStart)
}

// DecryptOrderMetadata decrypts the order metadata.
func (service *Service) DecryptOrderMetadata(ctx context.Context, order *pb.OrderLimit) (_ *internalpb.OrderLimitMetadata, err error) {
	defer mon.Task()(&ctx)(&err)

	var orderKeyID EncryptionKeyID
	copy(orderKeyID[:], order.EncryptedMetadataKeyId)

	key := service.encryptionKeys.Default
	if key.ID != orderKeyID {
		val, ok := service.encryptionKeys.KeyByID[orderKeyID]
		if !ok {
			return nil, ErrDecryptOrderMetadata.New("no encryption key found that matches the order.EncryptedMetadataKeyId")
		}
		key = EncryptionKey{
			ID:  orderKeyID,
			Key: val,
		}
	}
	return key.DecryptMetadata(order.SerialNumber, order.EncryptedMetadata)
}

func resolveStorageNode_Selected(node *nodeselection.SelectedNode, resolveDNS bool) *pb.Node {
	return resolveStorageNode(&pb.Node{
		Id:      node.ID,
		Address: node.Address,
	}, node.LastIPPort, resolveDNS)
}

func resolveStorageNode_Reputation(node *overlay.NodeReputation) *pb.Node {
	return resolveStorageNode(&pb.Node{
		Id:      node.ID,
		Address: node.Address,
	}, node.LastIPPort, false)
}

func resolveStorageNode(node *pb.Node, lastIPPort string, resolveDNS bool) *pb.Node {
	if resolveDNS && lastIPPort != "" {
		node = pb.CopyNode(node) // we mutate
		node.Address.Address = lastIPPort
	}
	return node
}
