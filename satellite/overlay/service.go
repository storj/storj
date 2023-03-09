// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/storj/location"
	"storj.io/common/sync2"
	"storj.io/private/version"
	"storj.io/storj/satellite/geoip"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeevents"
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

// ErrLowDifficulty is when the node id's difficulty is too low.
var ErrLowDifficulty = errs.Class("node id difficulty too low")

// DB implements the database for overlay.Service.
//
// architecture: Database
type DB interface {
	// GetOnlineNodesForGetDelete returns a map of nodes for the supplied nodeIDs
	GetOnlineNodesForGetDelete(ctx context.Context, nodeIDs []storj.NodeID, onlineWindow time.Duration, asOf AsOfSystemTimeConfig) (map[storj.NodeID]*SelectedNode, error)
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
	UpdateReputation(ctx context.Context, id storj.NodeID, request ReputationUpdate) error
	// UpdateNodeInfo updates node dossier with info requested from the node itself like node type, email, wallet, capacity, and version.
	UpdateNodeInfo(ctx context.Context, node storj.NodeID, nodeInfo *InfoResponse) (stats *NodeDossier, err error)
	// UpdateCheckIn updates a single storagenode's check-in stats.
	UpdateCheckIn(ctx context.Context, node NodeCheckInInfo, timestamp time.Time, config NodeSelectionConfig) (err error)
	// SetNodeContained updates the contained field for the node record.
	SetNodeContained(ctx context.Context, node storj.NodeID, contained bool) (err error)
	// SetAllContainedNodes updates the contained field for all nodes, as necessary.
	SetAllContainedNodes(ctx context.Context, containedNodes []storj.NodeID) (err error)

	// AllPieceCounts returns a map of node IDs to piece counts from the db.
	AllPieceCounts(ctx context.Context) (pieceCounts map[storj.NodeID]int64, err error)
	// UpdatePieceCounts sets the piece count field for the given node IDs.
	UpdatePieceCounts(ctx context.Context, pieceCounts map[storj.NodeID]int64) (err error)

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
	DisqualifyNode(ctx context.Context, nodeID storj.NodeID, disqualifiedAt time.Time, reason DisqualificationReason) (email string, err error)

	// GetOfflineNodesForEmail gets offline nodes in need of an email.
	GetOfflineNodesForEmail(ctx context.Context, offlineWindow time.Duration, cutoff time.Duration, cooldown time.Duration, limit int) (nodes map[storj.NodeID]string, err error)
	// UpdateLastOfflineEmail updates last_offline_email for a list of nodes.
	UpdateLastOfflineEmail(ctx context.Context, nodeIDs storj.NodeIDList, timestamp time.Time) (err error)

	// DQNodesLastSeenBefore disqualifies a limited number of nodes where last_contact_success < cutoff except those already disqualified
	// or gracefully exited or where last_contact_success = '0001-01-01 00:00:00+00'.
	DQNodesLastSeenBefore(ctx context.Context, cutoff time.Time, limit int) (nodeEmails map[storj.NodeID]string, count int, err error)

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
	// TestUpdateCheckInDirectUpdate tries to update a node info directly. Returns true if it succeeded, false if there were no node with the provided (used for testing).
	TestUpdateCheckInDirectUpdate(ctx context.Context, node NodeCheckInInfo, timestamp time.Time, semVer version.SemVer, walletFeatures string) (updated bool, err error)
	// OneTimeFixLastNets updates the last_net values for all node records to be equal to their
	// last_ip_port values.
	OneTimeFixLastNets(ctx context.Context) error

	// IterateAllContactedNodes will call cb on all known nodes (used in restore trash contexts).
	IterateAllContactedNodes(context.Context, func(context.Context, *SelectedNode) error) error
	// IterateAllNodeDossiers will call cb on all known nodes (used for invoice generation).
	IterateAllNodeDossiers(context.Context, func(context.Context, *NodeDossier) error) error
}

// DisqualificationReason is disqualification reason enum type.
type DisqualificationReason int

const (
	// DisqualificationReasonUnknown denotes undetermined disqualification reason.
	DisqualificationReasonUnknown DisqualificationReason = 0
	// DisqualificationReasonAuditFailure denotes disqualification due to audit score falling below threshold.
	DisqualificationReasonAuditFailure DisqualificationReason = 1
	// DisqualificationReasonSuspension denotes disqualification due to unknown audit failure after grace period for unknown audits
	// has elapsed.
	DisqualificationReasonSuspension DisqualificationReason = 2
	// DisqualificationReasonNodeOffline denotes disqualification due to node's online score falling below threshold after tracking
	// period has elapsed.
	DisqualificationReasonNodeOffline DisqualificationReason = 3
)

// NodeCheckInInfo contains all the info that will be updated when a node checkins.
type NodeCheckInInfo struct {
	NodeID                  storj.NodeID
	Address                 *pb.NodeAddress
	LastNet                 string
	LastIPPort              string
	IsUp                    bool
	Operator                *pb.NodeOperator
	Capacity                *pb.NodeCapacity
	Version                 *pb.NodeVersion
	CountryCode             location.CountryCode
	SoftwareUpdateEmailSent bool
	VersionBelowMin         bool
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
	AsOfSystemInterval time.Duration // only used for CRDB queries
	ExcludedCountries  []string
}

// ReputationStatus indicates current reputation status for a node.
type ReputationStatus struct {
	Email                  string
	Disqualified           *time.Time
	DisqualificationReason *DisqualificationReason
	UnknownAuditSuspended  *time.Time
	OfflineSuspended       *time.Time
	VettedAt               *time.Time
}

// ReputationUpdate contains reputation update data for a node.
type ReputationUpdate struct {
	Disqualified           *time.Time
	DisqualificationReason DisqualificationReason
	UnknownAuditSuspended  *time.Time
	OfflineSuspended       *time.Time
	VettedAt               *time.Time
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
	Type                    pb.NodeType
	Operator                pb.NodeOperator
	Capacity                pb.NodeCapacity
	Reputation              NodeStats
	Version                 pb.NodeVersion
	Contained               bool
	Disqualified            *time.Time
	DisqualificationReason  *DisqualificationReason
	UnknownAuditSuspended   *time.Time
	OfflineSuspended        *time.Time
	OfflineUnderReview      *time.Time
	PieceCount              int64
	ExitStatus              ExitStatus
	CreatedAt               time.Time
	LastNet                 string
	LastIPPort              string
	LastOfflineEmail        *time.Time
	LastSoftwareUpdateEmail *time.Time
	CountryCode             location.CountryCode
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
	copy := pb.CopyNode(&pb.Node{Id: node.ID, Address: node.Address})
	return &SelectedNode{
		ID:         copy.Id,
		Address:    copy.Address,
		LastNet:    node.LastNet,
		LastIPPort: node.LastIPPort,
	}
}

// Service is used to store and handle node information.
//
// architecture: Service
type Service struct {
	log              *zap.Logger
	db               DB
	nodeEvents       nodeevents.DB
	satelliteName    string
	satelliteAddress string
	config           Config

	GeoIP                  geoip.IPToCountry
	UploadSelectionCache   *UploadSelectionCache
	DownloadSelectionCache *DownloadSelectionCache
	LastNetFunc            LastNetFunc
}

// LastNetFunc is the type of a function that will be used to derive a network from an ip and port.
type LastNetFunc func(config NodeSelectionConfig, ip net.IP, port string) (string, error)

// NewService returns a new Service.
func NewService(log *zap.Logger, db DB, nodeEvents nodeevents.DB, satelliteAddr, satelliteName string, config Config) (*Service, error) {
	err := config.Node.AsOfSystemTime.isValid()
	if err != nil {
		return nil, errs.Wrap(err)
	}

	var geoIP geoip.IPToCountry = geoip.NewMockIPToCountry(config.GeoIP.MockCountries)
	if config.GeoIP.DB != "" {
		geoIP, err = geoip.OpenMaxmindDB(config.GeoIP.DB)
		if err != nil {
			return nil, Error.Wrap(err)
		}
	}

	uploadSelectionCache, err := NewUploadSelectionCache(log, db,
		config.NodeSelectionCache.Staleness, config.Node,
	)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	downloadSelectionCache, err := NewDownloadSelectionCache(log, db, DownloadSelectionCacheConfig{
		Staleness:      config.NodeSelectionCache.Staleness,
		OnlineWindow:   config.Node.OnlineWindow,
		AsOfSystemTime: config.Node.AsOfSystemTime,
	})
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return &Service{
		log:              log,
		db:               db,
		nodeEvents:       nodeEvents,
		satelliteAddress: satelliteAddr,
		satelliteName:    satelliteName,
		config:           config,

		GeoIP: geoIP,

		UploadSelectionCache:   uploadSelectionCache,
		DownloadSelectionCache: downloadSelectionCache,
		LastNetFunc:            MaskOffLastNet,
	}, nil
}

// Run runs the background processes needed for caches.
func (service *Service) Run(ctx context.Context) error {
	return errs.Combine(sync2.Concurrently(
		func() error { return service.UploadSelectionCache.Run(ctx) },
		func() error { return service.DownloadSelectionCache.Run(ctx) },
	)...)
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

	return service.db.GetOnlineNodesForGetDelete(ctx, nodeIDs, service.config.Node.OnlineWindow, service.config.Node.AsOfSystemTime)
}

// CachedGetOnlineNodesForGet returns a map of nodes from the download selection cache from the suppliedIDs.
func (service *Service) CachedGetOnlineNodesForGet(ctx context.Context, nodeIDs []storj.NodeID) (_ map[storj.NodeID]*SelectedNode, err error) {
	defer mon.Task()(&ctx)(&err)
	return service.DownloadSelectionCache.GetNodes(ctx, nodeIDs)
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
	// keep track of the network to make sure we only select nodes from different networks
	var excludedNetworks []string
	if len(excludedIDs) > 0 {
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

// InsertOfflineNodeEvents inserts offline events into node events.
func (service *Service) InsertOfflineNodeEvents(ctx context.Context, cooldown time.Duration, cutoff time.Duration, limit int) (count int, err error) {
	defer mon.Task()(&ctx)(&err)

	if !service.config.SendNodeEmails {
		return 0, nil
	}

	nodes, err := service.db.GetOfflineNodesForEmail(ctx, service.config.Node.OnlineWindow, cutoff, cooldown, limit)
	if err != nil {
		return 0, err
	}

	count = len(nodes)

	var successful storj.NodeIDList
	for id, email := range nodes {
		_, err = service.nodeEvents.Insert(ctx, email, id, nodeevents.Offline)
		if err != nil {
			service.log.Error("could not insert node offline into node events", zap.Error(err))
		} else {
			successful = append(successful, id)
		}
	}
	if len(successful) > 0 {
		err = service.db.UpdateLastOfflineEmail(ctx, successful, time.Now())
		if err != nil {
			return count, err
		}
	}
	return count, err
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
func (service *Service) UpdateReputation(ctx context.Context, id storj.NodeID, email string, request ReputationUpdate, reputationChanges []nodeevents.Type) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = service.db.UpdateReputation(ctx, id, request)
	if err != nil {
		return err
	}

	if service.config.SendNodeEmails {
		service.insertReputationNodeEvents(ctx, email, id, reputationChanges)
	}

	return nil
}

// UpdateNodeInfo updates node dossier with info requested from the node itself like node type, email, wallet, capacity, and version.
func (service *Service) UpdateNodeInfo(ctx context.Context, node storj.NodeID, nodeInfo *InfoResponse) (stats *NodeDossier, err error) {
	defer mon.Task()(&ctx)(&err)
	return service.db.UpdateNodeInfo(ctx, node, nodeInfo)
}

// SetNodeContained updates the contained field for the node record. If
// `contained` is true, the contained field in the record is set to the current
// database time, if it is not already set. If `contained` is false, the
// contained field in the record is set to NULL. All other fields are left
// alone.
func (service *Service) SetNodeContained(ctx context.Context, node storj.NodeID, contained bool) (err error) {
	defer mon.Task()(&ctx)(&err)

	return service.db.SetNodeContained(ctx, node, contained)
}

// UpdateCheckIn updates a single storagenode's check-in info if needed.
/*
The check-in info is updated in the database if:
	(1) there is no previous entry and the node is allowed (id difficulty, etc);
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
		if !node.IsUp {
			// this is a previously unknown node, and we couldn't pingback to verify that it even
			// exists. Don't bother putting it in the db.
			return nil
		}

		difficulty, err := node.NodeID.Difficulty()
		if err != nil {
			// this should never happen
			return err
		}
		if int(difficulty) < service.config.MinimumNewNodeIDDifficulty {
			return ErrLowDifficulty.New("node id difficulty is %d when %d is the minimum",
				difficulty, service.config.MinimumNewNodeIDDifficulty)
		}

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

	addrChanged := !pb.AddressEqual(node.Address, oldInfo.Address)

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

	if service.config.SendNodeEmails && service.config.Node.MinimumVersion != "" {
		min, err := version.NewSemVer(service.config.Node.MinimumVersion)
		if err != nil {
			return err
		}
		v, err := version.NewSemVer(node.Version.GetVersion())
		if err != nil {
			return err
		}

		if v.Compare(min) == -1 {
			node.VersionBelowMin = true
			if oldInfo.LastSoftwareUpdateEmail == nil ||
				oldInfo.LastSoftwareUpdateEmail.Add(service.config.NodeSoftwareUpdateEmailCooldown).Before(timestamp) {
				_, err = service.nodeEvents.Insert(ctx, node.Operator.Email, node.NodeID, nodeevents.BelowMinVersion)
				if err != nil {
					service.log.Error("could not insert node software below minimum version into node events", zap.Error(err))
				} else {
					node.SoftwareUpdateEmailSent = true
				}
			}
		}
	}

	if dbStale || addrChanged || walletChanged || verChanged || spaceChanged ||
		oldInfo.LastNet != node.LastNet || oldInfo.LastIPPort != node.LastIPPort ||
		oldInfo.CountryCode != node.CountryCode || node.SoftwareUpdateEmailSent {
		err = service.db.UpdateCheckIn(ctx, node, timestamp, service.config.Node)
		if err != nil {
			return Error.Wrap(err)
		}

		if service.config.SendNodeEmails && node.IsUp && oldInfo.Reputation.LastContactSuccess.Add(service.config.Node.OnlineWindow).Before(timestamp) {
			_, err = service.nodeEvents.Insert(ctx, node.Operator.Email, node.NodeID, nodeevents.Online)
			return Error.Wrap(err)
		}
		return nil
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

// DQNodesLastSeenBefore disqualifies nodes who have not been contacted since the cutoff time.
func (service *Service) DQNodesLastSeenBefore(ctx context.Context, cutoff time.Time, limit int) (count int, err error) {
	defer mon.Task()(&ctx)(&err)

	nodes, count, err := service.db.DQNodesLastSeenBefore(ctx, cutoff, limit)
	if err != nil {
		return 0, err
	}
	if service.config.SendNodeEmails {
		for nodeID, email := range nodes {
			_, err = service.nodeEvents.Insert(ctx, email, nodeID, nodeevents.Disqualified)
			if err != nil {
				service.log.Error("could not insert node disqualified into node events", zap.Error(err))
			}
		}
	}
	return count, err
}

// DisqualifyNode disqualifies a storage node.
func (service *Service) DisqualifyNode(ctx context.Context, nodeID storj.NodeID, reason DisqualificationReason) (err error) {
	defer mon.Task()(&ctx)(&err)
	email, err := service.db.DisqualifyNode(ctx, nodeID, time.Now().UTC(), reason)
	if err != nil {
		return err
	}
	if service.config.SendNodeEmails {
		_, err = service.nodeEvents.Insert(ctx, email, nodeID, nodeevents.Disqualified)
		if err != nil {
			service.log.Error("could not insert node disqualified into node events")
		}
	}
	return nil
}

// SelectAllStorageNodesDownload returns a nodes that are ready for downloading.
func (service *Service) SelectAllStorageNodesDownload(ctx context.Context, onlineWindow time.Duration, asOf AsOfSystemTimeConfig) (_ []*SelectedNode, err error) {
	defer mon.Task()(&ctx)(&err)
	return service.db.SelectAllStorageNodesDownload(ctx, onlineWindow, asOf)
}

// ResolveIPAndNetwork resolves the target address and determines its IP and appropriate subnet IPv4 or subnet IPv6.
func (service *Service) ResolveIPAndNetwork(ctx context.Context, target string) (ip net.IP, port, network string, err error) {
	// LastNetFunc is MaskOffLastNet, unless changed for a test.
	return ResolveIPAndNetwork(ctx, target, service.config.Node, service.LastNetFunc)
}

// ResolveIPAndNetwork resolves the target address and determines its IP and appropriate last_net, as indicated.
func ResolveIPAndNetwork(ctx context.Context, target string, config NodeSelectionConfig, lastNetFunc LastNetFunc) (ip net.IP, port, network string, err error) {
	defer mon.Task()(&ctx)(&err)

	host, port, err := net.SplitHostPort(target)
	if err != nil {
		return nil, "", "", err
	}
	ipAddr, err := net.ResolveIPAddr("ip", host)
	if err != nil {
		return nil, "", "", err
	}

	network, err = lastNetFunc(config, ipAddr.IP, port)
	if err != nil {
		return nil, "", "", err
	}

	return ipAddr.IP, port, network, nil
}

// MaskOffLastNet truncates the target address to the configured CIDR ipv6Cidr or ipv6Cidr prefix,
// if DistinctIP is enabled in the config. Otherwise, it returns the joined IP and port.
func MaskOffLastNet(config NodeSelectionConfig, addr net.IP, port string) (string, error) {
	if config.DistinctIP {
		// Filter all IPv4 Addresses into /24 subnets, and filter all IPv6 Addresses into /64 subnets
		return truncateIPToNet(addr, config.NetworkPrefixIPv4, config.NetworkPrefixIPv6)
	}
	// The "network" here will be the full IP and port; that is, every node will be considered to
	// be on a separate network, even if they all come from one IP (such as localhost).
	return net.JoinHostPort(addr.String(), port), nil
}

// truncateIPToNet truncates the target address to the given CIDR ipv4Cidr or ipv6Cidr prefix,
// according to which type of IP it is.
func truncateIPToNet(ipAddr net.IP, ipv4Cidr, ipv6Cidr int) (network string, err error) {
	// If addr can be converted to 4byte notation, it is an IPv4 address, else its an IPv6 address
	if ipv4 := ipAddr.To4(); ipv4 != nil {
		mask := net.CIDRMask(ipv4Cidr, 32)
		return ipv4.Mask(mask).String(), nil
	}
	if ipv6 := ipAddr.To16(); ipv6 != nil {
		mask := net.CIDRMask(ipv6Cidr, 128)
		return ipv6.Mask(mask).String(), nil
	}
	return "", fmt.Errorf("unable to get network for address %s", ipAddr.String())
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

func (service *Service) insertReputationNodeEvents(ctx context.Context, email string, id storj.NodeID, repEvents []nodeevents.Type) {
	defer mon.Task()(&ctx)(nil)

	for _, event := range repEvents {
		switch event {
		case nodeevents.Disqualified:
			_, err := service.nodeEvents.Insert(ctx, email, id, nodeevents.Disqualified)
			if err != nil {
				service.log.Error("could not insert node disqualified into node events", zap.Error(err))
			}
		case nodeevents.UnknownAuditSuspended:
			_, err := service.nodeEvents.Insert(ctx, email, id, nodeevents.UnknownAuditSuspended)
			if err != nil {
				service.log.Error("could not insert node unknown audit suspended into node events", zap.Error(err))
			}
		case nodeevents.UnknownAuditUnsuspended:
			_, err := service.nodeEvents.Insert(ctx, email, id, nodeevents.UnknownAuditUnsuspended)
			if err != nil {
				service.log.Error("could not insert node unknown audit unsuspended into node events", zap.Error(err))
			}
		case nodeevents.OfflineSuspended:
			_, err := service.nodeEvents.Insert(ctx, email, id, nodeevents.OfflineSuspended)
			if err != nil {
				service.log.Error("could not insert node offline suspended into node events", zap.Error(err))
			}
		case nodeevents.OfflineUnsuspended:
			_, err := service.nodeEvents.Insert(ctx, email, id, nodeevents.OfflineUnsuspended)
			if err != nil {
				service.log.Error("could not insert node offline unsuspended into node events", zap.Error(err))
			}
		default:
		}
	}
}
