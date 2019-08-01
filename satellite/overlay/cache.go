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

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
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

// ErrBucketNotFound is returned if a bucket is unable to be found in the routing table
var ErrBucketNotFound = errs.New("bucket not found")

// ErrNotEnoughNodes is when selecting nodes failed with the given parameters
var ErrNotEnoughNodes = errs.Class("not enough nodes")

// OverlayError creates class of errors for stack traces
var OverlayError = errs.Class("overlay error")

// DB implements the database for overlay.Cache
type DB interface {
	// SelectStorageNodes looks up nodes based on criteria
	SelectStorageNodes(ctx context.Context, count int, criteria *NodeCriteria) ([]*pb.Node, error)
	// SelectNewStorageNodes looks up nodes based on new node criteria
	SelectNewStorageNodes(ctx context.Context, count int, criteria *NodeCriteria) ([]*pb.Node, error)

	// Get looks up the node by nodeID
	Get(ctx context.Context, nodeID storj.NodeID) (*NodeDossier, error)
	// KnownOffline filters a set of nodes to offline nodes
	KnownOffline(context.Context, *NodeCriteria, storj.NodeIDList) (storj.NodeIDList, error)
	// KnownUnreliableOrOffline filters a set of nodes to unhealth or offlines node, independent of new
	KnownUnreliableOrOffline(context.Context, *NodeCriteria, storj.NodeIDList) (storj.NodeIDList, error)
	// Reliable returns all nodes that are reliable
	Reliable(context.Context, *NodeCriteria) (storj.NodeIDList, error)
	// Paginate will page through the database nodes
	Paginate(ctx context.Context, offset int64, limit int) ([]*NodeDossier, bool, error)
	// PaginateQualified will page through the qualified nodes
	PaginateQualified(ctx context.Context, offset int64, limit int) ([]*pb.Node, bool, error)
	// IsVetted returns whether or not the node reaches reputable thresholds
	IsVetted(ctx context.Context, id storj.NodeID, criteria *NodeCriteria) (bool, error)
	// Update updates node address
	UpdateAddress(ctx context.Context, value *pb.Node, defaults NodeSelectionConfig) error
	// BatchUpdateStats updates multiple storagenode's stats in one transaction
	BatchUpdateStats(ctx context.Context, updateRequests []*UpdateRequest, batchSize int) (failed storj.NodeIDList, err error)
	// UpdateStats all parts of single storagenode's stats.
	UpdateStats(ctx context.Context, request *UpdateRequest) (stats *NodeStats, err error)
	// UpdateNodeInfo updates node dossier with info requested from the node itself like node type, email, wallet, capacity, and version.
	UpdateNodeInfo(ctx context.Context, node storj.NodeID, nodeInfo *pb.InfoResponse) (stats *NodeDossier, err error)
	// UpdateUptime updates a single storagenode's uptime stats.
	UpdateUptime(ctx context.Context, nodeID storj.NodeID, isUp bool, lambda, weight, uptimeDQ float64) (stats *NodeStats, err error)
}

// FindStorageNodesRequest defines easy request parameters.
type FindStorageNodesRequest struct {
	MinimumRequiredNodes int
	RequestedCount       int
	FreeBandwidth        int64
	FreeDisk             int64
	ExcludedNodes        []storj.NodeID
	MinimumVersion       string // semver or empty
}

// NodeCriteria are the requirements for selecting nodes
type NodeCriteria struct {
	FreeBandwidth  int64
	FreeDisk       int64
	AuditCount     int64
	UptimeCount    int64
	ExcludedNodes  []storj.NodeID
	ExcludedIPs    []string
	MinimumVersion string // semver or empty
	OnlineWindow   time.Duration
	DistinctIP     bool
}

// UpdateRequest is used to update a node status.
type UpdateRequest struct {
	NodeID       storj.NodeID
	AuditSuccess bool
	IsUp         bool
	// n.b. these are set values from the satellite.
	// They are part of the UpdateRequest struct in order to be
	// more easily accessible in satellite/satellitedb/overlaycache.go.
	AuditLambda  float64
	AuditWeight  float64
	AuditDQ      float64
	UptimeLambda float64
	UptimeWeight float64
	UptimeDQ     float64
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
}

// NodeStats contains statistics about a node.
type NodeStats struct {
	Latency90             int64
	AuditSuccessCount     int64
	AuditCount            int64
	UptimeSuccessCount    int64
	UptimeCount           int64
	LastContactSuccess    time.Time
	LastContactFailure    time.Time
	AuditReputationAlpha  float64
	UptimeReputationAlpha float64
	AuditReputationBeta   float64
	UptimeReputationBeta  float64
	Disqualified          *time.Time
}

// Cache is used to store and handle node information
type Cache struct {
	log    *zap.Logger
	db     DB
	config Config
}

// NewCache returns a new Cache
func NewCache(log *zap.Logger, db DB, config Config) *Cache {
	return &Cache{
		log:    log,
		db:     db,
		config: config,
	}
}

// Close closes resources
func (cache *Cache) Close() error { return nil }

// Inspect lists limited number of items in the cache
func (cache *Cache) Inspect(ctx context.Context) (_ storage.Keys, err error) {
	defer mon.Task()(&ctx)(&err)
	// TODO: implement inspection tools
	return nil, errors.New("not implemented")
}

// Paginate returns a list of `limit` nodes starting from `start` offset.
func (cache *Cache) Paginate(ctx context.Context, offset int64, limit int) (_ []*NodeDossier, _ bool, err error) {
	defer mon.Task()(&ctx)(&err)
	return cache.db.Paginate(ctx, offset, limit)
}

// PaginateQualified returns a list of `limit` qualified nodes starting from `start` offset.
func (cache *Cache) PaginateQualified(ctx context.Context, offset int64, limit int) (_ []*pb.Node, _ bool, err error) {
	defer mon.Task()(&ctx)(&err)
	return cache.db.PaginateQualified(ctx, offset, limit)
}

// Get looks up the provided nodeID from the overlay cache
func (cache *Cache) Get(ctx context.Context, nodeID storj.NodeID) (_ *NodeDossier, err error) {
	defer mon.Task()(&ctx)(&err)
	if nodeID.IsZero() {
		return nil, ErrEmptyNode
	}
	return cache.db.Get(ctx, nodeID)
}

// IsOnline checks if a node is 'online' based on the collected statistics.
func (cache *Cache) IsOnline(node *NodeDossier) bool {
	return time.Now().Sub(node.Reputation.LastContactSuccess) < cache.config.Node.OnlineWindow ||
		node.Reputation.LastContactSuccess.After(node.Reputation.LastContactFailure)
}

// FindStorageNodes searches the overlay network for nodes that meet the provided requirements
func (cache *Cache) FindStorageNodes(ctx context.Context, req FindStorageNodesRequest) (_ []*pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)
	return cache.FindStorageNodesWithPreferences(ctx, req, &cache.config.Node)
}

// FindStorageNodesWithPreferences searches the overlay network for nodes that meet the provided criteria
func (cache *Cache) FindStorageNodesWithPreferences(ctx context.Context, req FindStorageNodesRequest, preferences *NodeSelectionConfig) (nodes []*pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: add sanity limits to requested node count
	// TODO: add sanity limits to excluded nodes
	reputableNodeCount := req.MinimumRequiredNodes
	if reputableNodeCount <= 0 {
		reputableNodeCount = req.RequestedCount
	}

	excludedNodes := req.ExcludedNodes

	newNodeCount := 0
	if preferences.NewNodePercentage > 0 {
		newNodeCount = int(float64(reputableNodeCount) * preferences.NewNodePercentage)
	}

	var newNodes []*pb.Node
	if newNodeCount > 0 {
		newNodes, err = cache.db.SelectNewStorageNodes(ctx, newNodeCount, &NodeCriteria{
			FreeBandwidth:  req.FreeBandwidth,
			FreeDisk:       req.FreeDisk,
			AuditCount:     preferences.AuditCount,
			ExcludedNodes:  excludedNodes,
			MinimumVersion: preferences.MinimumVersion,
			OnlineWindow:   preferences.OnlineWindow,
			DistinctIP:     preferences.DistinctIP,
		})
		if err != nil {
			return nil, OverlayError.Wrap(err)
		}
	}

	var excludedIPs []string
	// add selected new nodes and their IPs to the excluded lists for reputable node selection
	for _, newNode := range newNodes {
		excludedNodes = append(excludedNodes, newNode.Id)
		if preferences.DistinctIP {
			excludedIPs = append(excludedIPs, newNode.LastIp)
		}
	}

	criteria := NodeCriteria{
		FreeBandwidth:  req.FreeBandwidth,
		FreeDisk:       req.FreeDisk,
		AuditCount:     preferences.AuditCount,
		UptimeCount:    preferences.UptimeCount,
		ExcludedNodes:  excludedNodes,
		ExcludedIPs:    excludedIPs,
		MinimumVersion: preferences.MinimumVersion,
		OnlineWindow:   preferences.OnlineWindow,
		DistinctIP:     preferences.DistinctIP,
	}
	reputableNodes, err := cache.db.SelectStorageNodes(ctx, reputableNodeCount-len(newNodes), &criteria)
	if err != nil {
		return nil, OverlayError.Wrap(err)
	}

	nodes = append(nodes, newNodes...)
	nodes = append(nodes, reputableNodes...)

	if len(nodes) < reputableNodeCount {
		return nodes, ErrNotEnoughNodes.New("requested %d found %d; %+v ", reputableNodeCount, len(nodes), criteria)
	}

	return nodes, nil
}

// KnownOffline filters a set of nodes to offline nodes
func (cache *Cache) KnownOffline(ctx context.Context, nodeIds storj.NodeIDList) (offlineNodes storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)
	criteria := &NodeCriteria{
		OnlineWindow: cache.config.Node.OnlineWindow,
	}
	return cache.db.KnownOffline(ctx, criteria, nodeIds)
}

// KnownUnreliableOrOffline filters a set of nodes to unhealth or offlines node, independent of new.
func (cache *Cache) KnownUnreliableOrOffline(ctx context.Context, nodeIds storj.NodeIDList) (badNodes storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)
	criteria := &NodeCriteria{
		OnlineWindow: cache.config.Node.OnlineWindow,
	}
	return cache.db.KnownUnreliableOrOffline(ctx, criteria, nodeIds)
}

// Reliable filters a set of nodes that are reliable, independent of new.
func (cache *Cache) Reliable(ctx context.Context) (nodes storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)
	criteria := &NodeCriteria{
		OnlineWindow: cache.config.Node.OnlineWindow,
	}
	return cache.db.Reliable(ctx, criteria)
}

// Put adds a node id and proto definition into the overlay cache
func (cache *Cache) Put(ctx context.Context, nodeID storj.NodeID, value pb.Node) (err error) {
	defer mon.Task()(&ctx)(&err)

	// If we get a Node without an ID (i.e. bootstrap node)
	// we don't want to add to the routing tbale
	if nodeID.IsZero() {
		return nil
	}
	if nodeID != value.Id {
		return errors.New("invalid request")
	}
	if value.Address == nil {
		return errors.New("node has no address")
	}
	// Resolve IP Address Network to ensure it is set
	value.LastIp, err = GetNetwork(ctx, value.Address.Address)
	if err != nil {
		return OverlayError.Wrap(err)
	}
	return cache.db.UpdateAddress(ctx, &value, cache.config.Node)
}

// IsVetted returns whether or not the node reaches reputable thresholds
func (cache *Cache) IsVetted(ctx context.Context, nodeID storj.NodeID) (reputable bool, err error) {
	defer mon.Task()(&ctx)(&err)
	criteria := &NodeCriteria{
		AuditCount:  cache.config.Node.AuditCount,
		UptimeCount: cache.config.Node.UptimeCount,
	}
	reputable, err = cache.db.IsVetted(ctx, nodeID, criteria)
	if err != nil {
		return false, err
	}
	return reputable, nil
}

// BatchUpdateStats updates multiple storagenode's stats in one transaction
func (cache *Cache) BatchUpdateStats(ctx context.Context, requests []*UpdateRequest) (failed storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)

	for _, request := range requests {
		request.AuditLambda = cache.config.Node.AuditReputationLambda
		request.AuditWeight = cache.config.Node.AuditReputationWeight
		request.AuditDQ = cache.config.Node.AuditReputationDQ
		request.UptimeLambda = cache.config.Node.UptimeReputationLambda
		request.UptimeWeight = cache.config.Node.UptimeReputationWeight
		request.UptimeDQ = cache.config.Node.UptimeReputationDQ
	}
	return cache.db.BatchUpdateStats(ctx, requests, cache.config.UpdateStatsBatchSize)
}

// UpdateStats all parts of single storagenode's stats.
func (cache *Cache) UpdateStats(ctx context.Context, request *UpdateRequest) (stats *NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)

	request.AuditLambda = cache.config.Node.AuditReputationLambda
	request.AuditWeight = cache.config.Node.AuditReputationWeight
	request.AuditDQ = cache.config.Node.AuditReputationDQ
	request.UptimeLambda = cache.config.Node.UptimeReputationLambda
	request.UptimeWeight = cache.config.Node.UptimeReputationWeight
	request.UptimeDQ = cache.config.Node.UptimeReputationDQ

	return cache.db.UpdateStats(ctx, request)
}

// UpdateNodeInfo updates node dossier with info requested from the node itself like node type, email, wallet, capacity, and version.
func (cache *Cache) UpdateNodeInfo(ctx context.Context, node storj.NodeID, nodeInfo *pb.InfoResponse) (stats *NodeDossier, err error) {
	defer mon.Task()(&ctx)(&err)
	return cache.db.UpdateNodeInfo(ctx, node, nodeInfo)
}

// UpdateUptime updates a single storagenode's uptime stats.
func (cache *Cache) UpdateUptime(ctx context.Context, nodeID storj.NodeID, isUp bool) (stats *NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)
	lambda := cache.config.Node.UptimeReputationLambda
	weight := cache.config.Node.UptimeReputationWeight
	uptimeDQ := cache.config.Node.UptimeReputationDQ

	return cache.db.UpdateUptime(ctx, nodeID, isUp, lambda, weight, uptimeDQ)
}

// ConnFailure implements the Transport Observer `ConnFailure` function
func (cache *Cache) ConnFailure(ctx context.Context, node *pb.Node, failureError error) {
	var err error
	defer mon.Task()(&ctx)(&err)

	lambda := cache.config.Node.UptimeReputationLambda
	weight := cache.config.Node.UptimeReputationWeight
	uptimeDQ := cache.config.Node.UptimeReputationDQ

	// TODO: Kademlia paper specifies 5 unsuccessful PINGs before removing the node
	// from our routing table, but this is the cache so maybe we want to treat
	// it differently.
	_, err = cache.db.UpdateUptime(ctx, node.Id, false, lambda, weight, uptimeDQ)
	if err != nil {
		cache.log.Debug("error updating uptime for node", zap.Error(err))
	}
}

// ConnSuccess implements the Transport Observer `ConnSuccess` function
func (cache *Cache) ConnSuccess(ctx context.Context, node *pb.Node) {
	var err error
	defer mon.Task()(&ctx)(&err)

	err = cache.Put(ctx, node.Id, *node)
	if err != nil {
		cache.log.Debug("error updating uptime for node", zap.Error(err))
	}

	lambda := cache.config.Node.UptimeReputationLambda
	weight := cache.config.Node.UptimeReputationWeight
	uptimeDQ := cache.config.Node.UptimeReputationDQ

	_, err = cache.db.UpdateUptime(ctx, node.Id, true, lambda, weight, uptimeDQ)
	if err != nil {
		cache.log.Debug("error updating node connection info", zap.Error(err))
	}
}

// GetMissingPieces returns the list of offline nodes
func (cache *Cache) GetMissingPieces(ctx context.Context, pieces []*pb.RemotePiece) (missingPieces []int32, err error) {
	defer mon.Task()(&ctx)(&err)
	var nodeIDs storj.NodeIDList
	for _, p := range pieces {
		nodeIDs = append(nodeIDs, p.NodeId)
	}
	badNodeIDs, err := cache.KnownUnreliableOrOffline(ctx, nodeIDs)
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

func getIP(ctx context.Context, target string) (ip net.IPAddr, err error) {
	defer mon.Task()(&ctx)(&err)
	host, _, err := net.SplitHostPort(target)
	if err != nil {
		return net.IPAddr{}, err
	}
	ipAddr, err := net.ResolveIPAddr("ip", host)
	if err != nil {
		return net.IPAddr{}, err
	}
	return *ipAddr, nil
}

// GetNetwork resolves the target address and determines its IP /24 Subnet
func GetNetwork(ctx context.Context, target string) (network string, err error) {
	defer mon.Task()(&ctx)(&err)

	addr, err := getIP(ctx, target)
	if err != nil {
		return "", err
	}

	// If addr can be converted to 4byte notation, it is an IPv4 address, else its an IPv6 address
	if ipv4 := addr.IP.To4(); ipv4 != nil {
		//Filter all IPv4 Addresses into /24 Subnet's
		mask := net.CIDRMask(24, 32)
		return ipv4.Mask(mask).String(), nil
	}
	if ipv6 := addr.IP.To16(); ipv6 != nil {
		//Filter all IPv6 Addresses into /64 Subnet's
		mask := net.CIDRMask(64, 128)
		return ipv6.Mask(mask).String(), nil
	}

	return "", errors.New("unable to get network for address " + addr.String())
}
