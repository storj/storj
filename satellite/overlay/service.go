// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/storj/location"
	"storj.io/storj/satellite/geoip"
	"storj.io/storj/satellite/metabase"
)

// ErrEmptyNode is returned when the nodeID is empty.
var ErrEmptyNode = errs.New("empty node ID")

// ErrNodeNotFound is returned if a node does not exist in database.
var ErrNodeNotFound = errs.Class("node not found")

// ErrNodeOffline is returned if a nodes is offline.
var ErrNodeOffline = errs.Class("node is offline")

// ErrNodeDisqualified is returned if a nodes is disqualified.
var ErrNodeDisqualified = errs.Class("node is disqualified")

// ErrNodeFinishedGE is returned if a node has finished graceful exit.
var ErrNodeFinishedGE = errs.Class("node finished graceful exit")

// ErrNotEnoughNodes is when selecting nodes failed with the given parameters.
var ErrNotEnoughNodes = errs.Class("not enough nodes")

// DB implements the database for overlay.Service.
//
// architecture: Database
type DB interface {
	// GetOnlineNodesForGetDelete returns a map of nodes for the supplied nodeIDs
	GetOnlineNodesForGetDelete(ctx context.Context, nodeIDs []storj.NodeID, onlineWindow time.Duration) (map[storj.NodeID]*SelectedNode, error)
	// GetOnlineNodesForAuditRepair returns a map of nodes for the supplied nodeIDs.
	// The return value contains necessary information to create orders as well as nodes'
	// current reputation status.
	GetOnlineNodesForAuditRepair(ctx context.Context, nodeIDs []storj.NodeID, onlineWindow time.Duration) (map[storj.NodeID]*NodeReputation, error)
	// SelectStorageNodes looks up nodes based on criteria
	SelectStorageNodes(ctx context.Context, totalNeededNodes, newNodeCount int, criteria *NodeCriteria) ([]*SelectedNode, error)
	// SelectAllStorageNodesUpload returns all nodes that qualify to store data, organized as reputable nodes and new nodes
	SelectAllStorageNodesUpload(ctx context.Context, selectionCfg NodeSelectionConfig) (reputable, new []*SelectedNode, err error)
	// SelectAllStorageNodesDownload returns a nodes that are ready for downloading
	SelectAllStorageNodesDownload(ctx context.Context, onlineWindow time.Duration, asOf AsOfSystemTimeConfig) ([]*SelectedNode, error)

	// Get looks up the node by nodeID
	Get(ctx context.Context, nodeID storj.NodeID) (*NodeDossier, error)
	// KnownOffline filters a set of nodes to offline nodes
	KnownOffline(context.Context, *NodeCriteria, storj.NodeIDList) (storj.NodeIDList, error)
	// KnownUnreliableOrOffline filters a set of nodes to unhealth or offlines node, independent of new
	KnownUnreliableOrOffline(context.Context, *NodeCriteria, storj.NodeIDList) (storj.NodeIDList, error)
	// KnownReliableInExcludedCountries filters healthy nodes that are in excluded countries.
	KnownReliableInExcludedCountries(context.Context, *NodeCriteria, storj.NodeIDList) (storj.NodeIDList, error)
	// KnownReliable filters a set of nodes to reliable (online and qualified) nodes.
	KnownReliable(ctx context.Context, onlineWindow time.Duration, nodeIDs storj.NodeIDList) ([]*pb.Node, error)
	// Reliable returns all nodes that are reliable
	Reliable(context.Context, *NodeCriteria) (storj.NodeIDList, error)
	// UpdateReputation updates the DB columns for all reputation fields in ReputationStatus.
	UpdateReputation(ctx context.Context, id storj.NodeID, request *ReputationStatus) error
	// UpdateNodeInfo updates node dossier with info requested from the node itself like node type, email, wallet, capacity, and version.
	UpdateNodeInfo(ctx context.Context, node storj.NodeID, nodeInfo *InfoResponse) (stats *NodeDossier, err error)
	// UpdateCheckIn updates a single storagenode's check-in stats.
	UpdateCheckIn(ctx context.Context, node NodeCheckInInfo, timestamp time.Time, config NodeSelectionConfig) (err error)

	// AllPieceCounts returns a map of node IDs to piece counts from the db.
	AllPieceCounts(ctx context.Context) (pieceCounts map[storj.NodeID]int, err error)
	// UpdatePieceCounts sets the piece count field for the given node IDs.
	UpdatePieceCounts(ctx context.Context, pieceCounts map[storj.NodeID]int) (err error)

	// UpdateExitStatus is used to update a node's graceful exit status.
	UpdateExitStatus(ctx context.Context, request *ExitStatusRequest) (_ *NodeDossier, err error)
	// GetExitingNodes returns nodes who have initiated a graceful exit, but have not completed it.
	GetExitingNodes(ctx context.Context) (exitingNodes []*ExitStatus, err error)
	// GetGracefulExitCompletedByTimeFrame returns nodes who have completed graceful exit within a time window (time window is around graceful exit completion).
	GetGracefulExitCompletedByTimeFrame(ctx context.Context, begin, end time.Time) (exitedNodes storj.NodeIDList, err error)
	// GetGracefulExitIncompleteByTimeFrame returns nodes who have initiated, but not completed graceful exit within a time window (time window is around graceful exit initiation).
	GetGracefulExitIncompleteByTimeFrame(ctx context.Context, begin, end time.Time) (exitingNodes storj.NodeIDList, err error)
	// GetExitStatus returns a node's graceful exit status.
	GetExitStatus(ctx context.Context, nodeID storj.NodeID) (exitStatus *ExitStatus, err error)

	// GetNodesNetwork returns the /24 subnet for each storage node, order is not guaranteed.
	GetNodesNetwork(ctx context.Context, nodeIDs []storj.NodeID) (nodeNets []string, err error)

	// DisqualifyNode disqualifies a storage node.
	DisqualifyNode(ctx context.Context, nodeID storj.NodeID) (err error)

	// DQNodesLastSeenBefore disqualifies a limited number of nodes where last_contact_success < cutoff except those already disqualified
	// or gracefully exited or where last_contact_success = '0001-01-01 00:00:00+00'.
	DQNodesLastSeenBefore(ctx context.Context, cutoff time.Time, limit int) (count int, err error)

	// TestSuspendNodeUnknownAudit suspends a storage node for unknown audits.
	TestSuspendNodeUnknownAudit(ctx context.Context, nodeID storj.NodeID, suspendedAt time.Time) (err error)
	// TestUnsuspendNodeUnknownAudit unsuspends a storage node for unknown audits.
	TestUnsuspendNodeUnknownAudit(ctx context.Context, nodeID storj.NodeID) (err error)

	// TestVetNode directly sets a node's vetted_at timestamp to make testing easier
	TestVetNode(ctx context.Context, nodeID storj.NodeID) (vettedTime *time.Time, err error)
	// TestUnvetNode directly sets a node's vetted_at timestamp to null to make testing easier
	TestUnvetNode(ctx context.Context, nodeID storj.NodeID) (err error)
	// TestVetNode directly sets a node's offline_suspended timestamp to make testing easier
	TestSuspendNodeOffline(ctx context.Context, nodeID storj.NodeID, suspendedAt time.Time) (err error)
	// TestNodeCountryCode sets node country code.
	TestNodeCountryCode(ctx context.Context, nodeID storj.NodeID, countryCode string) (err error)

	// IterateAllNodes will call cb on all known nodes (used in restore trash contexts).
	IterateAllNodes(context.Context, func(context.Context, *SelectedNode) error) error
	// IterateAllNodes will call cb on all known nodes (used for invoice generation).
	IterateAllNodeDossiers(context.Context, func(context.Context, *NodeDossier) error) error
}

// NodeCheckInInfo contains all the info that will be updated when a node checkins.
type NodeCheckInInfo struct {
	NodeID      storj.NodeID
	Address     *pb.NodeAddress
	LastNet     string
	LastIPPort  string
	IsUp        bool
	Operator    *pb.NodeOperator
	Capacity    *pb.NodeCapacity
	Version     *pb.NodeVersion
	CountryCode location.CountryCode
}

// InfoResponse contains node dossier info requested from the storage node.
type InfoResponse struct {
	Type     pb.NodeType
	Operator *pb.NodeOperator
	Capacity *pb.NodeCapacity
	Version  *pb.NodeVersion
}

// FindStorageNodesRequest defines easy request parameters.
type FindStorageNodesRequest struct {
	RequestedCount     int
	ExcludedIDs        []storj.NodeID
	MinimumVersion     string        // semver or empty
	AsOfSystemInterval time.Duration // only used for CRDB queries
	Placement          storj.PlacementConstraint
}

// NodeCriteria are the requirements for selecting nodes.
type NodeCriteria struct {
	FreeDisk           int64
	ExcludedIDs        []storj.NodeID
	ExcludedNetworks   []string // the /24 subnet IPv4 or /64 subnet IPv6 for nodes
	MinimumVersion     string   // semver or empty
	OnlineWindow       time.Duration
	DistinctIP         bool
	AsOfSystemInterval time.Duration // only used for CRDB queries
	ExcludedCountries  []string
}

// ReputationStatus indicates current reputation status for a node.
type ReputationStatus struct {
	Contained             bool // TODO: check to see if this column is still used.
	Disqualified          *time.Time
	UnknownAuditSuspended *time.Time
	OfflineSuspended      *time.Time
	VettedAt              *time.Time
}

// Equal checks if two ReputationStatus contains the same value.
func (status ReputationStatus) Equal(value ReputationStatus) bool {
	if status.Contained != value.Contained {
		return false
	}

	if status.Disqualified != nil && value.Disqualified != nil {
		if !status.Disqualified.Equal(*value.Disqualified) {
			return false
		}
	} else if !(status.Disqualified == nil && value.Disqualified == nil) {
		return false
	}

	if status.UnknownAuditSuspended != nil && value.UnknownAuditSuspended != nil {
		if !status.UnknownAuditSuspended.Equal(*value.UnknownAuditSuspended) {
			return false
		}
	} else if !(status.UnknownAuditSuspended == nil && value.UnknownAuditSuspended == nil) {
		return false
	}

	if status.OfflineSuspended != nil && value.OfflineSuspended != nil {
		if !status.OfflineSuspended.Equal(*value.OfflineSuspended) {
			return false
		}
	} else if !(status.OfflineSuspended == nil && value.OfflineSuspended == nil) {
		return false
	}

	if status.VettedAt != nil && value.VettedAt != nil {
		if !status.VettedAt.Equal(*value.VettedAt) {
			return false
		}
	} else if !(status.VettedAt == nil && value.VettedAt == nil) {
		return false
	}

	return true
}

// ExitStatus is used for reading graceful exit status.
type ExitStatus struct {
	NodeID              storj.NodeID
	ExitInitiatedAt     *time.Time
	ExitLoopCompletedAt *time.Time
	ExitFinishedAt      *time.Time
	ExitSuccess         bool
}

// ExitStatusRequest is used to update a node's graceful exit status.
type ExitStatusRequest struct {
	NodeID              storj.NodeID
	ExitInitiatedAt     time.Time
	ExitLoopCompletedAt time.Time
	ExitFinishedAt      time.Time
	ExitSuccess         bool
}

// NodeDossier is the complete info that the satellite tracks for a storage node.
type NodeDossier struct {
	pb.Node
	Type                  pb.NodeType
	Operator              pb.NodeOperator
	Capacity              pb.NodeCapacity
	Reputation            NodeStats
	Version               pb.NodeVersion
	Contained             bool
	Disqualified          *time.Time
	UnknownAuditSuspended *time.Time
	OfflineSuspended      *time.Time
	OfflineUnderReview    *time.Time
	PieceCount            int64
	ExitStatus            ExitStatus
	CreatedAt             time.Time
	LastNet               string
	LastIPPort            string
	CountryCode           location.CountryCode
}

// NodeStats contains statistics about a node.
type NodeStats struct {
	Latency90          int64
	LastContactSuccess time.Time
	LastContactFailure time.Time
	OfflineUnderReview *time.Time
	Status             ReputationStatus
}

// NodeLastContact contains the ID, address, and timestamp.
type NodeLastContact struct {
	URL                storj.NodeURL
	LastIPPort         string
	LastContactSuccess time.Time
	LastContactFailure time.Time
}

// SelectedNode is used as a result for creating orders limits.
type SelectedNode struct {
	ID          storj.NodeID
	Address     *pb.NodeAddress
	LastNet     string
	LastIPPort  string
	CountryCode location.CountryCode
}

// NodeReputation is used as a result for creating orders limits for audits.
type NodeReputation struct {
	ID         storj.NodeID
	Address    *pb.NodeAddress
	LastNet    string
	LastIPPort string
	Reputation ReputationStatus
}

// Clone returns a deep clone of the selected node.
func (node *SelectedNode) Clone() *SelectedNode {
	return &SelectedNode{
		ID: node.ID,
		Address: &pb.NodeAddress{
			Transport: node.Address.Transport,
			Address:   node.Address.Address,
		},
		LastNet:    node.LastNet,
		LastIPPort: node.LastIPPort,
	}
}

// Service is used to store and handle node information.
//
// architecture: Service
type Service struct {
	log    *zap.Logger
	db     DB
	config Config

	GeoIP                  geoip.IPToCountry
	UploadSelectionCache   *UploadSelectionCache
	DownloadSelectionCache *DownloadSelectionCache
}

// NewService returns a new Service.
func NewService(log *zap.Logger, db DB, config Config) (*Service, error) {
	err := config.Node.AsOfSystemTime.isValid()
	if err != nil {
		return nil, err
	}

	var geoIP geoip.IPToCountry = geoip.NewMockIPToCountry(config.GeoIP.MockCountries)
	if config.GeoIP.DB != "" {
		geoIP, err = geoip.OpenMaxmindDB(config.GeoIP.DB)
		if err != nil {
			return nil, Error.Wrap(err)
		}
	}

	return &Service{
		log:    log,
		db:     db,
		config: config,

		GeoIP: geoIP,

		UploadSelectionCache: NewUploadSelectionCache(log, db,
			config.NodeSelectionCache.Staleness, config.Node,
		),

		DownloadSelectionCache: NewDownloadSelectionCache(log, db, DownloadSelectionCacheConfig{
			Staleness:      config.NodeSelectionCache.Staleness,
			OnlineWindow:   config.Node.OnlineWindow,
			AsOfSystemTime: config.Node.AsOfSystemTime,
		}),
	}, nil
}

// Close closes resources.
func (service *Service) Close() error {
	return service.GeoIP.Close()
}

// Get looks up the provided nodeID from the overlay.
func (service *Service) Get(ctx context.Context, nodeID storj.NodeID) (_ *NodeDossier, err error) {
	defer mon.Task()(&ctx)(&err)
	if nodeID.IsZero() {
		return nil, ErrEmptyNode
	}
	return service.db.Get(ctx, nodeID)
}

// GetOnlineNodesForGetDelete returns a map of nodes for the supplied nodeIDs.
func (service *Service) GetOnlineNodesForGetDelete(ctx context.Context, nodeIDs []storj.NodeID) (_ map[storj.NodeID]*SelectedNode, err error) {
	defer mon.Task()(&ctx)(&err)

	return service.db.GetOnlineNodesForGetDelete(ctx, nodeIDs, service.config.Node.OnlineWindow)
}

// GetOnlineNodesForAuditRepair returns a map of nodes for the supplied nodeIDs.
func (service *Service) GetOnlineNodesForAuditRepair(ctx context.Context, nodeIDs []storj.NodeID) (_ map[storj.NodeID]*NodeReputation, err error) {
	defer mon.Task()(&ctx)(&err)

	return service.db.GetOnlineNodesForAuditRepair(ctx, nodeIDs, service.config.Node.OnlineWindow)
}

// GetNodeIPs returns a map of node ip:port for the supplied nodeIDs.
func (service *Service) GetNodeIPs(ctx context.Context, nodeIDs []storj.NodeID) (_ map[storj.NodeID]string, err error) {
	defer mon.Task()(&ctx)(&err)
	return service.DownloadSelectionCache.GetNodeIPs(ctx, nodeIDs)
}

// IsOnline checks if a node is 'online' based on the collected statistics.
func (service *Service) IsOnline(node *NodeDossier) bool {
	return time.Since(node.Reputation.LastContactSuccess) < service.config.Node.OnlineWindow
}

// FindStorageNodesForGracefulExit searches the overlay network for nodes that meet the provided requirements for graceful-exit requests.
func (service *Service) FindStorageNodesForGracefulExit(ctx context.Context, req FindStorageNodesRequest) (_ []*SelectedNode, err error) {
	defer mon.Task()(&ctx)(&err)
	return service.UploadSelectionCache.GetNodes(ctx, req)
}

// FindStorageNodesForUpload searches the overlay network for nodes that meet the provided requirements for upload.
//
// When enabled it uses the cache to select nodes.
// When the node selection from the cache fails, it falls back to the old implementation.
func (service *Service) FindStorageNodesForUpload(ctx context.Context, req FindStorageNodesRequest) (_ []*SelectedNode, err error) {
	defer mon.Task()(&ctx)(&err)
	if service.config.Node.AsOfSystemTime.Enabled && service.config.Node.AsOfSystemTime.DefaultInterval < 0 {
		req.AsOfSystemInterval = service.config.Node.AsOfSystemTime.DefaultInterval
	}

	// TODO excluding country codes on upload if cache is disabled is not implemented
	if service.config.NodeSelectionCache.Disabled {
		return service.FindStorageNodesWithPreferences(ctx, req, &service.config.Node)
	}

	selectedNodes, err := service.UploadSelectionCache.GetNodes(ctx, req)
	if err != nil {
		return selectedNodes, err
	}
	if len(selectedNodes) < req.RequestedCount {

		excludedIDs := make([]string, 0)
		for _, e := range req.ExcludedIDs {
			excludedIDs = append(excludedIDs, e.String())
		}

		service.log.Warn("Not enough nodes are available from Node Cache",
			zap.String("minVersion", req.MinimumVersion),
			zap.Strings("excludedIDs", excludedIDs),
			zap.Duration("asOfSystemInterval", req.AsOfSystemInterval),
			zap.Int("requested", req.RequestedCount),
			zap.Int("available", len(selectedNodes)),
			zap.Uint16("placement", uint16(req.Placement)))
	}
	return selectedNodes, err
}

// FindStorageNodesWithPreferences searches the overlay network for nodes that meet the provided criteria.
//
// This does not use a cache.
func (service *Service) FindStorageNodesWithPreferences(ctx context.Context, req FindStorageNodesRequest, preferences *NodeSelectionConfig) (nodes []*SelectedNode, err error) {
	defer mon.Task()(&ctx)(&err)
	// TODO: add sanity limits to requested node count
	// TODO: add sanity limits to excluded nodes
	totalNeededNodes := req.RequestedCount

	excludedIDs := req.ExcludedIDs
	// if distinctIP is enabled, keep track of the network
	// to make sure we only select nodes from different networks
	var excludedNetworks []string
	if preferences.DistinctIP && len(excludedIDs) > 0 {
		excludedNetworks, err = service.db.GetNodesNetwork(ctx, excludedIDs)
		if err != nil {
			return nil, Error.Wrap(err)
		}
	}

	newNodeCount := 0
	if preferences.NewNodeFraction > 0 {
		newNodeCount = int(float64(totalNeededNodes) * preferences.NewNodeFraction)
	}

	criteria := NodeCriteria{
		FreeDisk:           preferences.MinimumDiskSpace.Int64(),
		ExcludedIDs:        excludedIDs,
		ExcludedNetworks:   excludedNetworks,
		MinimumVersion:     preferences.MinimumVersion,
		OnlineWindow:       preferences.OnlineWindow,
		DistinctIP:         preferences.DistinctIP,
		AsOfSystemInterval: req.AsOfSystemInterval,
	}
	nodes, err = service.db.SelectStorageNodes(ctx, totalNeededNodes, newNodeCount, &criteria)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if len(nodes) < totalNeededNodes {
		return nodes, ErrNotEnoughNodes.New("requested %d found %d; %+v ", totalNeededNodes, len(nodes), criteria)
	}

	return nodes, nil
}

// KnownOffline filters a set of nodes to offline nodes.
func (service *Service) KnownOffline(ctx context.Context, nodeIds storj.NodeIDList) (offlineNodes storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)
	criteria := &NodeCriteria{
		OnlineWindow: service.config.Node.OnlineWindow,
	}
	return service.db.KnownOffline(ctx, criteria, nodeIds)
}

// KnownUnreliableOrOffline filters a set of nodes to unhealth or offlines node, independent of new.
func (service *Service) KnownUnreliableOrOffline(ctx context.Context, nodeIds storj.NodeIDList) (badNodes storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)
	criteria := &NodeCriteria{
		OnlineWindow: service.config.Node.OnlineWindow,
	}
	return service.db.KnownUnreliableOrOffline(ctx, criteria, nodeIds)
}

// KnownReliableInExcludedCountries filters healthy nodes that are in excluded countries.
func (service *Service) KnownReliableInExcludedCountries(ctx context.Context, nodeIds storj.NodeIDList) (reliableInExcluded storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)

	criteria := &NodeCriteria{
		OnlineWindow:      service.config.Node.OnlineWindow,
		ExcludedCountries: service.config.RepairExcludedCountryCodes,
	}
	return service.db.KnownReliableInExcludedCountries(ctx, criteria, nodeIds)
}

// KnownReliable filters a set of nodes to reliable (online and qualified) nodes.
func (service *Service) KnownReliable(ctx context.Context, nodeIDs storj.NodeIDList) (nodes []*pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)
	return service.db.KnownReliable(ctx, service.config.Node.OnlineWindow, nodeIDs)
}

// Reliable filters a set of nodes that are reliable, independent of new.
func (service *Service) Reliable(ctx context.Context) (nodes storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)

	criteria := &NodeCriteria{
		OnlineWindow: service.config.Node.OnlineWindow,
	}
	criteria.ExcludedCountries = service.config.RepairExcludedCountryCodes
	return service.db.Reliable(ctx, criteria)
}

// UpdateReputation updates the DB columns for any of the reputation fields.
func (service *Service) UpdateReputation(ctx context.Context, id storj.NodeID, request *ReputationStatus) (err error) {
	defer mon.Task()(&ctx)(&err)
	return service.db.UpdateReputation(ctx, id, request)
}

// UpdateNodeInfo updates node dossier with info requested from the node itself like node type, email, wallet, capacity, and version.
func (service *Service) UpdateNodeInfo(ctx context.Context, node storj.NodeID, nodeInfo *InfoResponse) (stats *NodeDossier, err error) {
	defer mon.Task()(&ctx)(&err)
	return service.db.UpdateNodeInfo(ctx, node, nodeInfo)
}

// UpdateCheckIn updates a single storagenode's check-in info if needed.
/*
The check-in info is updated in the database if:
	(1) there is no previous entry;
	(2) it has been too long since the last known entry; or
	(3) the node hostname, IP address, port, wallet, sw version, or disk capacity
	has changed.
Note that there can be a race between acquiring the previous entry and
performing the update, so if two updates happen at about the same time it is
not defined which one will end up in the database.
*/
func (service *Service) UpdateCheckIn(ctx context.Context, node NodeCheckInInfo, timestamp time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	failureMeter := mon.Meter("geofencing_lookup_failed")

	oldInfo, err := service.Get(ctx, node.NodeID)
	if err != nil && !ErrNodeNotFound.Has(err) {
		return Error.New("failed to get node info from DB")
	}

	if oldInfo == nil {
		node.CountryCode, err = service.GeoIP.LookupISOCountryCode(node.LastIPPort)
		if err != nil {
			failureMeter.Mark(1)
			service.log.Debug("failed to resolve country code for node",
				zap.String("node address", node.Address.Address),
				zap.Stringer("Node ID", node.NodeID),
				zap.Error(err))
		}

		return service.db.UpdateCheckIn(ctx, node, timestamp, service.config.Node)
	}

	lastUp, lastDown := oldInfo.Reputation.LastContactSuccess, oldInfo.Reputation.LastContactFailure
	lastContact := lastUp
	if lastContact.Before(lastDown) {
		lastContact = lastDown
	}

	dbStale := lastContact.Add(service.config.NodeCheckInWaitPeriod).Before(timestamp) ||
		(node.IsUp && lastUp.Before(lastDown)) || (!node.IsUp && lastDown.Before(lastUp))

	addrChanged := ((node.Address == nil) != (oldInfo.Address == nil)) ||
		(oldInfo.Address != nil && node.Address != nil && oldInfo.Address.Address != node.Address.Address)

	walletChanged := (node.Operator == nil && oldInfo.Operator.Wallet != "") ||
		(node.Operator != nil && oldInfo.Operator.Wallet != node.Operator.Wallet)

	verChanged := (node.Version == nil && oldInfo.Version.Version != "") ||
		(node.Version != nil && oldInfo.Version.Version != node.Version.Version)

	spaceChanged := (node.Capacity == nil && oldInfo.Capacity.FreeDisk != 0) ||
		(node.Capacity != nil && node.Capacity.FreeDisk != oldInfo.Capacity.FreeDisk)

	if oldInfo.CountryCode == location.CountryCode(0) || oldInfo.LastIPPort != node.LastIPPort {
		node.CountryCode, err = service.GeoIP.LookupISOCountryCode(node.LastIPPort)
		if err != nil {
			failureMeter.Mark(1)
			service.log.Debug("failed to resolve country code for node",
				zap.String("node address", node.Address.Address),
				zap.Stringer("Node ID", node.NodeID),
				zap.Error(err))
		}
	} else {
		node.CountryCode = oldInfo.CountryCode
	}

	if dbStale || addrChanged || walletChanged || verChanged || spaceChanged ||
		oldInfo.LastNet != node.LastNet || oldInfo.LastIPPort != node.LastIPPort ||
		oldInfo.CountryCode != node.CountryCode {
		return service.db.UpdateCheckIn(ctx, node, timestamp, service.config.Node)
	}

	service.log.Debug("ignoring unnecessary check-in",
		zap.String("node address", node.Address.Address),
		zap.Stringer("Node ID", node.NodeID))
	mon.Event("unnecessary_node_check_in")

	return nil
}

// GetMissingPieces returns the list of offline nodes and the corresponding pieces.
func (service *Service) GetMissingPieces(ctx context.Context, pieces metabase.Pieces) (missingPieces []uint16, err error) {
	defer mon.Task()(&ctx)(&err)
	var nodeIDs storj.NodeIDList
	for _, p := range pieces {
		nodeIDs = append(nodeIDs, p.StorageNode)
	}
	badNodeIDs, err := service.KnownUnreliableOrOffline(ctx, nodeIDs)
	if err != nil {
		return nil, Error.New("error getting nodes %s", err)
	}

	for _, p := range pieces {
		for _, nodeID := range badNodeIDs {
			if nodeID == p.StorageNode {
				missingPieces = append(missingPieces, p.Number)
			}
		}
	}
	return missingPieces, nil
}

// GetReliablePiecesInExcludedCountries returns the list of pieces held by nodes located in excluded countries.
func (service *Service) GetReliablePiecesInExcludedCountries(ctx context.Context, pieces metabase.Pieces) (piecesInExcluded []uint16, err error) {
	defer mon.Task()(&ctx)(&err)
	var nodeIDs storj.NodeIDList
	for _, p := range pieces {
		nodeIDs = append(nodeIDs, p.StorageNode)
	}
	inExcluded, err := service.KnownReliableInExcludedCountries(ctx, nodeIDs)
	if err != nil {
		return nil, Error.New("error getting nodes %s", err)
	}

	for _, p := range pieces {
		for _, nodeID := range inExcluded {
			if nodeID == p.StorageNode {
				piecesInExcluded = append(piecesInExcluded, p.Number)
			}
		}
	}
	return piecesInExcluded, nil
}

// DisqualifyNode disqualifies a storage node.
func (service *Service) DisqualifyNode(ctx context.Context, nodeID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)
	return service.db.DisqualifyNode(ctx, nodeID)
}

// ResolveIPAndNetwork resolves the target address and determines its IP and /24 subnet IPv4 or /64 subnet IPv6.
func ResolveIPAndNetwork(ctx context.Context, target string) (ipPort, network string, err error) {
	defer mon.Task()(&ctx)(&err)

	host, port, err := net.SplitHostPort(target)
	if err != nil {
		return "", "", err
	}
	ipAddr, err := net.ResolveIPAddr("ip", host)
	if err != nil {
		return "", "", err
	}

	// If addr can be converted to 4byte notation, it is an IPv4 address, else its an IPv6 address
	if ipv4 := ipAddr.IP.To4(); ipv4 != nil {
		// Filter all IPv4 Addresses into /24 Subnet's
		mask := net.CIDRMask(24, 32)
		return net.JoinHostPort(ipAddr.String(), port), ipv4.Mask(mask).String(), nil
	}
	if ipv6 := ipAddr.IP.To16(); ipv6 != nil {
		// Filter all IPv6 Addresses into /64 Subnet's
		mask := net.CIDRMask(64, 128)
		return net.JoinHostPort(ipAddr.String(), port), ipv6.Mask(mask).String(), nil
	}

	return "", "", errors.New("unable to get network for address " + ipAddr.String())
}

// TestVetNode directly sets a node's vetted_at timestamp to make testing easier.
func (service *Service) TestVetNode(ctx context.Context, nodeID storj.NodeID) (vettedTime *time.Time, err error) {
	vettedTime, err = service.db.TestVetNode(ctx, nodeID)
	service.log.Warn("node vetted", zap.Stringer("node ID", nodeID), zap.Stringer("vetted time", vettedTime))
	if err != nil {
		service.log.Warn("error vetting node", zap.Stringer("node ID", nodeID))
		return nil, err
	}
	err = service.UploadSelectionCache.Refresh(ctx)
	service.log.Warn("nodecache refresh err", zap.Error(err))
	return vettedTime, err
}

// TestUnvetNode directly sets a node's vetted_at timestamp to null to make testing easier.
func (service *Service) TestUnvetNode(ctx context.Context, nodeID storj.NodeID) (err error) {
	err = service.db.TestUnvetNode(ctx, nodeID)
	if err != nil {
		service.log.Warn("error unvetting node", zap.Stringer("node ID", nodeID), zap.Error(err))
		return err
	}
	err = service.UploadSelectionCache.Refresh(ctx)
	service.log.Warn("nodecache refresh err", zap.Error(err))
	return err
}

// TestNodeCountryCode directly sets a node's vetted_at timestamp to null to make testing easier.
func (service *Service) TestNodeCountryCode(ctx context.Context, nodeID storj.NodeID, countryCode string) (err error) {
	err = service.db.TestNodeCountryCode(ctx, nodeID, countryCode)
	if err != nil {
		service.log.Warn("error updating node", zap.Stringer("node ID", nodeID), zap.Error(err))
		return err
	}

	return nil
}
