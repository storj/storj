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
	"storj.io/storj/storage"
)

// ErrEmptyNode is returned when the nodeID is empty
var ErrEmptyNode = errs.New("empty node ID")

// ErrNodeNotFound is returned if a node does not exist in database
var ErrNodeNotFound = errs.Class("node not found")

// ErrNodeOffline is returned if a nodes is offline
var ErrNodeOffline = errs.Class("node is offline")

// ErrNodeDisqualified is returned if a nodes is disqualified
var ErrNodeDisqualified = errs.Class("node is disqualified")

// ErrNotEnoughNodes is when selecting nodes failed with the given parameters
var ErrNotEnoughNodes = errs.Class("not enough nodes")

// DB implements the database for overlay.Service
//
// architecture: Database
type DB interface {
	// GetOnlineNodesForGetDelete returns a map of nodes for the supplied nodeIDs
	GetOnlineNodesForGetDelete(ctx context.Context, nodeIDs []storj.NodeID, onlineWindow time.Duration) (map[storj.NodeID]*SelectedNode, error)
	// SelectStorageNodes looks up nodes based on criteria
	SelectStorageNodes(ctx context.Context, totalNeededNodes, newNodeCount int, criteria *NodeCriteria) ([]*SelectedNode, error)
	// SelectAllStorageNodesUpload returns all nodes that qualify to store data, organized as reputable nodes and new nodes
	SelectAllStorageNodesUpload(ctx context.Context, selectionCfg NodeSelectionConfig) (reputable, new []*SelectedNode, err error)

	// Get looks up the node by nodeID
	Get(ctx context.Context, nodeID storj.NodeID) (*NodeDossier, error)
	// KnownOffline filters a set of nodes to offline nodes
	KnownOffline(context.Context, *NodeCriteria, storj.NodeIDList) (storj.NodeIDList, error)
	// KnownUnreliableOrOffline filters a set of nodes to unhealth or offlines node, independent of new
	KnownUnreliableOrOffline(context.Context, *NodeCriteria, storj.NodeIDList) (storj.NodeIDList, error)
	// KnownReliable filters a set of nodes to reliable (online and qualified) nodes.
	KnownReliable(ctx context.Context, onlineWindow time.Duration, nodeIDs storj.NodeIDList) ([]*pb.Node, error)
	// Reliable returns all nodes that are reliable
	Reliable(context.Context, *NodeCriteria) (storj.NodeIDList, error)
	// BatchUpdateStats updates multiple storagenode's stats in one transaction
	BatchUpdateStats(ctx context.Context, updateRequests []*UpdateRequest, batchSize int) (failed storj.NodeIDList, err error)
	// UpdateStats all parts of single storagenode's stats.
	UpdateStats(ctx context.Context, request *UpdateRequest) (stats *NodeStats, err error)
	// UpdateNodeInfo updates node dossier with info requested from the node itself like node type, email, wallet, capacity, and version.
	UpdateNodeInfo(ctx context.Context, node storj.NodeID, nodeInfo *pb.InfoResponse) (stats *NodeDossier, err error)
	// UpdateUptime updates a single storagenode's uptime stats.
	UpdateUptime(ctx context.Context, nodeID storj.NodeID, isUp bool) (stats *NodeStats, err error)
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

	// GetSuccesfulNodesNotCheckedInSince returns all nodes that last check-in was successful, but haven't checked-in within a given duration.
	GetSuccesfulNodesNotCheckedInSince(ctx context.Context, duration time.Duration) (nodeAddresses []NodeLastContact, err error)
	// GetOfflineNodesLimited returns a list of the first N offline nodes ordered by least recently contacted.
	GetOfflineNodesLimited(ctx context.Context, limit int) ([]NodeLastContact, error)

	// DisqualifyNode disqualifies a storage node.
	DisqualifyNode(ctx context.Context, nodeID storj.NodeID) (err error)

	// SuspendNode suspends a storage node.
	SuspendNode(ctx context.Context, nodeID storj.NodeID, suspendedAt time.Time) (err error)
	// UnsuspendNode unsuspends a storage node.
	UnsuspendNode(ctx context.Context, nodeID storj.NodeID) (err error)
}

// NodeCheckInInfo contains all the info that will be updated when a node checkins
type NodeCheckInInfo struct {
	NodeID     storj.NodeID
	Address    *pb.NodeAddress
	LastNet    string
	LastIPPort string
	IsUp       bool
	Operator   *pb.NodeOperator
	Capacity   *pb.NodeCapacity
	Version    *pb.NodeVersion
}

// FindStorageNodesRequest defines easy request parameters.
type FindStorageNodesRequest struct {
	MinimumRequiredNodes int
	RequestedCount       int
	ExcludedIDs          []storj.NodeID
	MinimumVersion       string // semver or empty
}

// NodeCriteria are the requirements for selecting nodes
type NodeCriteria struct {
	FreeDisk         int64
	AuditCount       int64
	UptimeCount      int64
	ExcludedIDs      []storj.NodeID
	ExcludedNetworks []string // the /24 subnet IPv4 or /64 subnet IPv6 for nodes
	MinimumVersion   string   // semver or empty
	OnlineWindow     time.Duration
	DistinctIP       bool
}

// AuditType is an enum representing the outcome of a particular audit reported to the overlay.
type AuditType int

const (
	// AuditSuccess represents a successful audit.
	AuditSuccess AuditType = iota
	// AuditFailure represents a failed audit.
	AuditFailure
	// AuditUnknown represents an audit that resulted in an unknown error from the node.
	AuditUnknown
)

// UpdateRequest is used to update a node status.
type UpdateRequest struct {
	NodeID       storj.NodeID
	AuditOutcome AuditType
	IsUp         bool
	// n.b. these are set values from the satellite.
	// They are part of the UpdateRequest struct in order to be
	// more easily accessible in satellite/satellitedb/overlaycache.go.
	AuditLambda           float64
	AuditWeight           float64
	AuditDQ               float64
	SuspensionGracePeriod time.Duration
	SuspensionDQEnabled   bool
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

// NodeDossier is the complete info that the satellite tracks for a storage node
type NodeDossier struct {
	pb.Node
	Type         pb.NodeType
	Operator     pb.NodeOperator
	Capacity     pb.NodeCapacity
	Reputation   NodeStats
	Version      pb.NodeVersion
	Contained    bool
	Disqualified *time.Time
	Suspended    *time.Time
	PieceCount   int64
	ExitStatus   ExitStatus
	CreatedAt    time.Time
	LastNet      string
	LastIPPort   string
}

// NodeStats contains statistics about a node.
type NodeStats struct {
	Latency90                   int64
	AuditSuccessCount           int64
	AuditCount                  int64
	UptimeSuccessCount          int64
	UptimeCount                 int64
	LastContactSuccess          time.Time
	LastContactFailure          time.Time
	AuditReputationAlpha        float64
	AuditReputationBeta         float64
	Disqualified                *time.Time
	UnknownAuditReputationAlpha float64
	UnknownAuditReputationBeta  float64
	Suspended                   *time.Time
}

// NodeLastContact contains the ID, address, and timestamp
type NodeLastContact struct {
	ID                 storj.NodeID
	Address            string
	LastIPPort         string
	LastContactSuccess time.Time
	LastContactFailure time.Time
}

// SelectedNode is used as a result for creating orders limits.
type SelectedNode struct {
	ID         storj.NodeID
	Address    *pb.NodeAddress
	LastNet    string
	LastIPPort string
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

// Service is used to store and handle node information
//
// architecture: Service
type Service struct {
	log            *zap.Logger
	db             DB
	config         Config
	SelectionCache *NodeSelectionCache
}

// NewService returns a new Service
func NewService(log *zap.Logger, db DB, config Config) *Service {
	return &Service{
		log:    log,
		db:     db,
		config: config,
		SelectionCache: NewNodeSelectionCache(log, db,
			config.NodeSelectionCache.Staleness, config.Node,
		),
	}
}

// Close closes resources
func (service *Service) Close() error { return nil }

// Inspect lists limited number of items in the cache
func (service *Service) Inspect(ctx context.Context) (_ storage.Keys, err error) {
	defer mon.Task()(&ctx)(&err)
	// TODO: implement inspection tools
	return nil, errors.New("not implemented")
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

// IsOnline checks if a node is 'online' based on the collected statistics.
func (service *Service) IsOnline(node *NodeDossier) bool {
	return time.Since(node.Reputation.LastContactSuccess) < service.config.Node.OnlineWindow
}

// FindStorageNodesForRepair searches the overlay network for nodes that meet the provided requirements for repair.
//
// The main difference from FindStorageNodesForUpload is that here we filter out all nodes that share a subnet with any node in req.ExcludedIDs.
// This additional complexity is not needed for other uses of finding storage nodes.
func (service *Service) FindStorageNodesForRepair(ctx context.Context, req FindStorageNodesRequest) (_ []*SelectedNode, err error) {
	defer mon.Task()(&ctx)(&err)
	return service.FindStorageNodesWithPreferences(ctx, req, &service.config.Node)
}

// FindStorageNodesForGracefulExit searches the overlay network for nodes that meet the provided requirements for graceful-exit requests.
//
// The main difference between this method and the normal FindStorageNodes is that here we avoid using the cache.
func (service *Service) FindStorageNodesForGracefulExit(ctx context.Context, req FindStorageNodesRequest) (_ []*SelectedNode, err error) {
	defer mon.Task()(&ctx)(&err)
	return service.FindStorageNodesWithPreferences(ctx, req, &service.config.Node)
}

// FindStorageNodesForUpload searches the overlay network for nodes that meet the provided requirements for upload.
//
// When enabled it uses the cache to select nodes.
// When the node selection from the cache fails, it falls back to the old implementation.
func (service *Service) FindStorageNodesForUpload(ctx context.Context, req FindStorageNodesRequest) (_ []*SelectedNode, err error) {
	defer mon.Task()(&ctx)(&err)
	if service.config.NodeSelectionCache.Disabled {
		return service.FindStorageNodesWithPreferences(ctx, req, &service.config.Node)
	}

	selectedNodes, err := service.SelectionCache.GetNodes(ctx, req)
	if err != nil {
		service.log.Warn("error selecting from node selection cache", zap.String("err", err.Error()))
	}
	if len(selectedNodes) < req.RequestedCount {
		mon.Event("default_node_selection")
		return service.FindStorageNodesWithPreferences(ctx, req, &service.config.Node)
	}
	return selectedNodes, nil
}

// FindStorageNodesWithPreferences searches the overlay network for nodes that meet the provided criteria.
//
// This does not use a cache.
func (service *Service) FindStorageNodesWithPreferences(ctx context.Context, req FindStorageNodesRequest, preferences *NodeSelectionConfig) (nodes []*SelectedNode, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: add sanity limits to requested node count
	// TODO: add sanity limits to excluded nodes
	totalNeededNodes := req.MinimumRequiredNodes
	if totalNeededNodes <= 0 {
		totalNeededNodes = req.RequestedCount
	}

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
		FreeDisk:         preferences.MinimumDiskSpace.Int64(),
		AuditCount:       preferences.AuditCount,
		UptimeCount:      preferences.UptimeCount,
		ExcludedIDs:      excludedIDs,
		ExcludedNetworks: excludedNetworks,
		MinimumVersion:   preferences.MinimumVersion,
		OnlineWindow:     preferences.OnlineWindow,
		DistinctIP:       preferences.DistinctIP,
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

// KnownOffline filters a set of nodes to offline nodes
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
	return service.db.Reliable(ctx, criteria)
}

// BatchUpdateStats updates multiple storagenode's stats in one transaction
func (service *Service) BatchUpdateStats(ctx context.Context, requests []*UpdateRequest) (failed storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)

	for _, request := range requests {
		request.AuditLambda = service.config.Node.AuditReputationLambda
		request.AuditWeight = service.config.Node.AuditReputationWeight
		request.AuditDQ = service.config.Node.AuditReputationDQ
		request.SuspensionGracePeriod = service.config.Node.SuspensionGracePeriod
		request.SuspensionDQEnabled = service.config.Node.SuspensionDQEnabled
	}
	return service.db.BatchUpdateStats(ctx, requests, service.config.UpdateStatsBatchSize)
}

// UpdateStats all parts of single storagenode's stats.
func (service *Service) UpdateStats(ctx context.Context, request *UpdateRequest) (stats *NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)

	request.AuditLambda = service.config.Node.AuditReputationLambda
	request.AuditWeight = service.config.Node.AuditReputationWeight
	request.AuditDQ = service.config.Node.AuditReputationDQ
	request.SuspensionGracePeriod = service.config.Node.SuspensionGracePeriod
	request.SuspensionDQEnabled = service.config.Node.SuspensionDQEnabled

	return service.db.UpdateStats(ctx, request)
}

// UpdateNodeInfo updates node dossier with info requested from the node itself like node type, email, wallet, capacity, and version.
func (service *Service) UpdateNodeInfo(ctx context.Context, node storj.NodeID, nodeInfo *pb.InfoResponse) (stats *NodeDossier, err error) {
	defer mon.Task()(&ctx)(&err)
	return service.db.UpdateNodeInfo(ctx, node, nodeInfo)
}

// UpdateUptime updates a single storagenode's uptime stats.
func (service *Service) UpdateUptime(ctx context.Context, nodeID storj.NodeID, isUp bool) (stats *NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)
	return service.db.UpdateUptime(ctx, nodeID, isUp)
}

// UpdateCheckIn updates a single storagenode's check-in info.
func (service *Service) UpdateCheckIn(ctx context.Context, node NodeCheckInInfo, timestamp time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	return service.db.UpdateCheckIn(ctx, node, timestamp, service.config.Node)
}

// GetSuccesfulNodesNotCheckedInSince returns all nodes that last check-in was successful, but haven't checked-in within a given duration.
func (service *Service) GetSuccesfulNodesNotCheckedInSince(ctx context.Context, duration time.Duration) (nodeLastContacts []NodeLastContact, err error) {
	defer mon.Task()(&ctx)(&err)

	return service.db.GetSuccesfulNodesNotCheckedInSince(ctx, duration)
}

// GetMissingPieces returns the list of offline nodes
func (service *Service) GetMissingPieces(ctx context.Context, pieces []*pb.RemotePiece) (missingPieces []int32, err error) {
	defer mon.Task()(&ctx)(&err)
	var nodeIDs storj.NodeIDList
	for _, p := range pieces {
		nodeIDs = append(nodeIDs, p.NodeId)
	}
	badNodeIDs, err := service.KnownUnreliableOrOffline(ctx, nodeIDs)
	if err != nil {
		return nil, Error.New("error getting nodes %s", err)
	}

	for _, p := range pieces {
		for _, nodeID := range badNodeIDs {
			if nodeID == p.NodeId {
				missingPieces = append(missingPieces, p.GetPieceNum())
			}
		}
	}
	return missingPieces, nil
}

// DisqualifyNode disqualifies a storage node.
func (service *Service) DisqualifyNode(ctx context.Context, nodeID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)
	return service.db.DisqualifyNode(ctx, nodeID)
}

// GetOfflineNodesLimited returns a list of the first N offline nodes ordered by least recently contacted.
func (service *Service) GetOfflineNodesLimited(ctx context.Context, limit int) (offlineNodes []NodeLastContact, err error) {
	defer mon.Task()(&ctx)(&err)
	return service.db.GetOfflineNodesLimited(ctx, limit)
}

// ResolveIPAndNetwork resolves the target address and determines its IP and /24 subnet IPv4 or /64 subnet IPv6
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
		//Filter all IPv4 Addresses into /24 Subnet's
		mask := net.CIDRMask(24, 32)
		return net.JoinHostPort(ipAddr.String(), port), ipv4.Mask(mask).String(), nil
	}
	if ipv6 := ipAddr.IP.To16(); ipv6 != nil {
		//Filter all IPv6 Addresses into /64 Subnet's
		mask := net.CIDRMask(64, 128)
		return net.JoinHostPort(ipAddr.String(), port), ipv6.Mask(mask).String(), nil
	}

	return "", "", errors.New("unable to get network for address " + ipAddr.String())
}
