// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"sync"
	"time"

	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/common/version"
	"storj.io/storj/satellite/nodeselection"
)

// Mockdb is an in-memory  mock implementation of the nodeevents.DB interface for testing.
type Mockdb struct {
	mu        sync.Mutex
	CallCount int
	Reputable []*nodeselection.SelectedNode
	New       []*nodeselection.SelectedNode
}

// SelectAllStorageNodesUpload satisfies nodeevents.DB interface.
func (m *Mockdb) SelectAllStorageNodesUpload(ctx context.Context, selectionCfg NodeSelectionConfig) (reputable, new []*nodeselection.SelectedNode, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sync2.Sleep(ctx, 500*time.Millisecond)
	m.CallCount++

	reputable = make([]*nodeselection.SelectedNode, len(m.Reputable))
	for i, n := range m.Reputable {
		reputable[i] = n.Clone()
	}
	new = make([]*nodeselection.SelectedNode, len(m.New))
	for i, n := range m.New {
		new[i] = n.Clone()
	}

	return reputable, new, nil
}

// GetOnlineNodesForAuditAndRepair satisfies nodeevents.DB interface.
func (m *Mockdb) GetOnlineNodesForAuditAndRepair(ctx context.Context, nodeIDs []storj.NodeID, onlineWindow time.Duration) (map[storj.NodeID]*NodeReputation, error) {
	panic("implement me")
}

// GetAllOnlineNodesForRepair satisfies nodeevents.DB interface.
func (m *Mockdb) GetAllOnlineNodesForRepair(ctx context.Context, onlineWindow time.Duration) (map[storj.NodeID]*NodeReputation, error) {
	panic("implement me")
}

// SelectAllStorageNodesDownload satisfies nodeevents.DB interface.
func (m *Mockdb) SelectAllStorageNodesDownload(ctx context.Context, onlineWindow time.Duration, asOf AsOfSystemTimeConfig) ([]*nodeselection.SelectedNode, error) {
	panic("implement me")
}

// Get satisfies nodeevents.DB interface.
func (m *Mockdb) Get(ctx context.Context, nodeID storj.NodeID) (*NodeDossier, error) {
	panic("implement me")
}

// GetParticipatingNodes satisfies nodeevents.DB interface.
func (m *Mockdb) GetParticipatingNodes(ctx context.Context, nodeIDs storj.NodeIDList, onlineWindow, asOfSystemInterval time.Duration) (_ []nodeselection.SelectedNode, err error) {
	panic("implement me")
}

// GetAllParticipatingNodes satisfies nodeevents.DB interface.
func (m *Mockdb) GetAllParticipatingNodes(ctx context.Context, onlineWindow, asOfSystemInterval time.Duration) (_ []nodeselection.SelectedNode, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var res []nodeselection.SelectedNode
	for _, node := range m.Reputable {
		res = append(res, *node.Clone())
	}
	for _, node := range m.New {
		res = append(res, *node.Clone())
	}
	return res, nil
}

// KnownReliable satisfies nodeevents.DB interface.
func (m *Mockdb) KnownReliable(ctx context.Context, nodeIDs storj.NodeIDList, onlineWindow, asOfSystemInterval time.Duration) (online []nodeselection.SelectedNode, offline []nodeselection.SelectedNode, err error) {
	panic("implement me")
}

// Reliable satisfies nodeevents.DB interface.
func (m *Mockdb) Reliable(ctx context.Context, onlineWindow, asOfSystemInterval time.Duration) (online []nodeselection.SelectedNode, offline []nodeselection.SelectedNode, err error) {
	panic("implement me")
}

// UpdateReputation satisfies nodeevents.DB interface.
func (m *Mockdb) UpdateReputation(ctx context.Context, id storj.NodeID, request ReputationUpdate) error {
	panic("implement me")
}

// UpdateNodeInfo satisfies nodeevents.DB interface.
func (m *Mockdb) UpdateNodeInfo(ctx context.Context, node storj.NodeID, nodeInfo *InfoResponse) (stats *NodeDossier, err error) {
	panic("implement me")
}

// UpdateCheckIn satisfies nodeevents.DB interface.
func (m *Mockdb) UpdateCheckIn(ctx context.Context, node NodeCheckInInfo, timestamp time.Time, config NodeSelectionConfig) (err error) {
	panic("implement me")
}

// SetNodeContained satisfies nodeevents.DB interface.
func (m *Mockdb) SetNodeContained(ctx context.Context, node storj.NodeID, contained bool) (err error) {
	panic("implement me")
}

// SetAllContainedNodes satisfies nodeevents.DB interface.
func (m *Mockdb) SetAllContainedNodes(ctx context.Context, containedNodes []storj.NodeID) (err error) {
	panic("implement me")
}

// ActiveNodesPieceCounts satisfies nodeevents.DB interface.
func (m *Mockdb) ActiveNodesPieceCounts(ctx context.Context) (pieceCounts map[storj.NodeID]int64, err error) {
	panic("implement me")
}

// UpdatePieceCounts satisfies nodeevents.DB interface.
func (m *Mockdb) UpdatePieceCounts(ctx context.Context, pieceCounts map[storj.NodeID]int64) (err error) {
	panic("implement me")
}

// UpdateExitStatus satisfies nodeevents.DB interface.
func (m *Mockdb) UpdateExitStatus(ctx context.Context, request *ExitStatusRequest) (_ *NodeDossier, err error) {
	panic("implement me")
}

// GetExitingNodes satisfies nodeevents.DB interface.
func (m *Mockdb) GetExitingNodes(ctx context.Context) (exitingNodes []*ExitStatus, err error) {
	panic("implement me")
}

// GetGracefulExitCompletedByTimeFrame satisfies nodeevents.DB interface.
func (m *Mockdb) GetGracefulExitCompletedByTimeFrame(ctx context.Context, begin, end time.Time) (exitedNodes storj.NodeIDList, err error) {
	panic("implement me")
}

// GetGracefulExitIncompleteByTimeFrame satisfies nodeevents.DB interface.
func (m *Mockdb) GetGracefulExitIncompleteByTimeFrame(ctx context.Context, begin, end time.Time) (exitingNodes storj.NodeIDList, err error) {
	panic("implement me")
}

// GetExitStatus satisfies nodeevents.DB interface.
func (m *Mockdb) GetExitStatus(ctx context.Context, nodeID storj.NodeID) (exitStatus *ExitStatus, err error) {
	panic("implement me")
}

// DisqualifyNode satisfies nodeevents.DB interface.
func (m *Mockdb) DisqualifyNode(ctx context.Context, nodeID storj.NodeID, disqualifiedAt time.Time, reason DisqualificationReason) (email string, err error) {
	panic("implement me")
}

// GetOfflineNodesForEmail satisfies nodeevents.DB interface.
func (m *Mockdb) GetOfflineNodesForEmail(ctx context.Context, offlineWindow time.Duration, cutoff time.Duration, cooldown time.Duration, limit int) (nodes map[storj.NodeID]string, err error) {
	panic("implement me")
}

// UpdateLastOfflineEmail satisfies nodeevents.DB interface.
func (m *Mockdb) UpdateLastOfflineEmail(ctx context.Context, nodeIDs storj.NodeIDList, timestamp time.Time) (err error) {
	panic("implement me")
}

// DQNodesLastSeenBefore satisfies nodeevents.DB interface.
func (m *Mockdb) DQNodesLastSeenBefore(ctx context.Context, cutoff time.Time, limit int) (nodeEmails map[storj.NodeID]string, count int, err error) {
	panic("implement me")
}

// TestSuspendNodeUnknownAudit satisfies nodeevents.DB interface.
func (m *Mockdb) TestSuspendNodeUnknownAudit(ctx context.Context, nodeID storj.NodeID, suspendedAt time.Time) (err error) {
	panic("implement me")
}

// TestUnsuspendNodeUnknownAudit satisfies nodeevents.DB interface.
func (m *Mockdb) TestUnsuspendNodeUnknownAudit(ctx context.Context, nodeID storj.NodeID) (err error) {
	panic("implement me")
}

// TestAddNodes satisfies nodeevents.DB interface.
func (m *Mockdb) TestAddNodes(ctx context.Context, nodes []*NodeDossier) (err error) {
	panic("implement me")
}

// TestVetNode satisfies nodeevents.DB interface.
func (m *Mockdb) TestVetNode(ctx context.Context, nodeID storj.NodeID) (vettedTime *time.Time, err error) {
	panic("implement me")
}

// TestUnvetNode satisfies nodeevents.DB interface.
func (m *Mockdb) TestUnvetNode(ctx context.Context, nodeID storj.NodeID) (err error) {
	panic("implement me")
}

// TestSuspendNodeOffline satisfies nodeevents.DB interface.
func (m *Mockdb) TestSuspendNodeOffline(ctx context.Context, nodeID storj.NodeID, suspendedAt time.Time) (err error) {
	panic("implement me")
}

// TestSetNodeCountryCode satisfies nodeevents.DB interface.
func (m *Mockdb) TestSetNodeCountryCode(ctx context.Context, nodeID storj.NodeID, countryCode string) (err error) {
	panic("implement me")
}

// TestUpdateCheckInDirectUpdate satisfies nodeevents.DB interface.
func (m *Mockdb) TestUpdateCheckInDirectUpdate(ctx context.Context, node NodeCheckInInfo, timestamp time.Time, semVer version.SemVer, walletFeatures string) (updated bool, err error) {
	panic("implement me")
}

// OneTimeFixLastNets satisfies nodeevents.DB interface.
func (m *Mockdb) OneTimeFixLastNets(ctx context.Context) error {
	panic("implement me")
}

// IterateAllContactedNodes satisfies nodeevents.DB interface.
func (m *Mockdb) IterateAllContactedNodes(ctx context.Context, f func(context.Context, *nodeselection.SelectedNode) error) error {
	panic("implement me")
}

// IterateAllNodeDossiers satisfies nodeevents.DB interface.
func (m *Mockdb) IterateAllNodeDossiers(ctx context.Context, f func(context.Context, *NodeDossier) error) error {
	panic("implement me")
}

// UpdateNodeTags satisfies nodeevents.DB interface.
func (m *Mockdb) UpdateNodeTags(ctx context.Context, tags nodeselection.NodeTags) error {
	panic("implement me")
}

// GetNodeTags satisfies nodeevents.DB interface.
func (m *Mockdb) GetNodeTags(ctx context.Context, id storj.NodeID) (nodeselection.NodeTags, error) {
	panic("implement me")
}

// GetLastIPPortByNodeTagNames gets last IP and port from nodes where node exists in node tags with a particular name.
func (m *Mockdb) GetLastIPPortByNodeTagNames(ctx context.Context, ids storj.NodeIDList, tagName []string) (lastIPPorts map[storj.NodeID]*string, err error) {
	panic("implement me")
}

// AccountingNodeInfo gets records for all specified nodes for accounting.
func (m *Mockdb) AccountingNodeInfo(ctx context.Context, nodeIDs storj.NodeIDList) (_ map[storj.NodeID]NodeAccountingInfo, err error) {
	panic("implement me")
}
