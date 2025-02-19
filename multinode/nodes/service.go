// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package nodes

import (
	"bytes"
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/rpc"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/storj/private/multinodepb"
)

var (
	mon = monkit.Package()

	// Error is an error class for nodes service error.
	Error = errs.Class("nodes")
	// ErrNodeNotReachable is an error class that indicates that we are not able to establish drpc connection with node.
	ErrNodeNotReachable = errs.Class("node is not reachable")
	// ErrNodeAPIKeyInvalid is an error class that indicates that we uses wrong api key.
	ErrNodeAPIKeyInvalid = errs.Class("node api key is invalid")
)

// Service exposes all nodes related logic.
//
// architecture: Service
type Service struct {
	log    *zap.Logger
	dialer rpc.Dialer
	nodes  DB
}

// NewService creates new instance of Service.
func NewService(log *zap.Logger, dialer rpc.Dialer, nodes DB) *Service {
	return &Service{
		log:    log,
		dialer: dialer,
		nodes:  nodes,
	}
}

// Add adds new node to the system.
func (service *Service) Add(ctx context.Context, node Node) (err error) {
	defer mon.Task()(&ctx)(&err)

	// trying to connect to node to check its availability.
	conn, err := service.dialer.DialNodeURL(ctx, storj.NodeURL{
		ID:      node.ID,
		Address: node.PublicAddress,
	})
	if err != nil {
		return ErrNodeNotReachable.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, conn.Close())
	}()

	nodeClient := multinodepb.NewDRPCNodeClient(conn)
	header := &multinodepb.RequestHeader{
		ApiKey: node.APISecret[:],
	}

	// making test request to check node api key.
	_, err = nodeClient.Version(ctx, &multinodepb.VersionRequest{Header: header})
	if err != nil {
		if rpcstatus.Code(err) == rpcstatus.Unauthenticated {
			return ErrNodeAPIKeyInvalid.Wrap(err)
		}
		return Error.Wrap(err)
	}

	return Error.Wrap(service.nodes.Add(ctx, node))
}

// List returns list of all nodes.
func (service *Service) List(ctx context.Context) (_ []Node, err error) {
	defer mon.Task()(&ctx)(&err)

	nodes, err := service.nodes.List(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return nodes, nil
}

// UpdateName will update name of the specified node.
func (service *Service) UpdateName(ctx context.Context, id storj.NodeID, name string) (err error) {
	defer mon.Task()(&ctx)(&err)
	return Error.Wrap(service.nodes.UpdateName(ctx, id, name))
}

// Get retrieves node by id.
func (service *Service) Get(ctx context.Context, id storj.NodeID) (_ Node, err error) {
	defer mon.Task()(&ctx)(&err)

	node, err := service.nodes.Get(ctx, id)
	if err != nil {
		return Node{}, Error.Wrap(err)
	}

	return node, nil
}

// Remove removes node from the system.
func (service *Service) Remove(ctx context.Context, id storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)
	return Error.Wrap(service.nodes.Remove(ctx, id))
}

// ListInfos queries node basic info from all nodes via rpc.
func (service *Service) ListInfos(ctx context.Context) (_ []NodeInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	nodes, err := service.nodes.List(ctx)
	if err != nil {
		if ErrNoNode.Has(err) {
			return []NodeInfo{}, nil
		}
		return nil, Error.Wrap(err)
	}

	var infos []NodeInfo
	for _, node := range nodes {
		info := func() NodeInfo {
			nodeInfo := NodeInfo{
				ID:   node.ID,
				Name: node.Name,
			}
			conn, err := service.dialer.DialNodeURL(ctx, storj.NodeURL{
				ID:      node.ID,
				Address: node.PublicAddress,
			})
			nodeStatus, nodeVersion, lastContact := service.FetchNodeMeta(ctx, node)
			if nodeStatus != StatusOnline {
				nodeInfo.Status = nodeStatus
				return nodeInfo
			}

			defer func() {
				err = errs.Combine(err, conn.Close())
			}()

			storageClient := multinodepb.NewDRPCStorageClient(conn)
			bandwidthClient := multinodepb.NewDRPCBandwidthClient(conn)
			payoutClient := multinodepb.NewDRPCPayoutClient(conn)

			header := &multinodepb.RequestHeader{
				ApiKey: node.APISecret[:],
			}

			diskSpace, err := storageClient.DiskSpace(ctx, &multinodepb.DiskSpaceRequest{Header: header})
			if err != nil {
				nodeInfo.Status = StatusStorageNodeInternalError
				return nodeInfo
			}

			earned, err := payoutClient.Earned(ctx, &multinodepb.EarnedRequest{Header: header})
			if err != nil {
				nodeInfo.Status = StatusStorageNodeInternalError
				return nodeInfo
			}

			bandwidthSummaryRequest := &multinodepb.BandwidthMonthSummaryRequest{
				Header: header,
			}
			bandwidthSummary, err := bandwidthClient.MonthSummary(ctx, bandwidthSummaryRequest)
			if err != nil {
				nodeInfo.Status = StatusStorageNodeInternalError
				return nodeInfo
			}

			nodeInfo.Version = nodeVersion.Version
			nodeInfo.LastContact = lastContact.LastContact
			nodeInfo.DiskSpaceUsed = diskSpace.GetUsedPieces() + diskSpace.GetUsedTrash()
			nodeInfo.DiskSpaceLeft = diskSpace.GetAvailable()
			nodeInfo.BandwidthUsed = bandwidthSummary.GetUsed()
			nodeInfo.TotalEarned = earned.Total
			nodeInfo.Status = nodeStatus

			return nodeInfo
		}()

		infos = append(infos, info)
	}

	return infos, nil
}

// ListInfosSatellite queries node satellite specific info from all nodes via rpc.
func (service *Service) ListInfosSatellite(ctx context.Context, satelliteID storj.NodeID) (_ []NodeInfoSatellite, err error) {
	defer mon.Task()(&ctx)(&err)

	nodes, err := service.nodes.List(ctx)
	if err != nil {
		if ErrNoNode.Has(err) {
			return []NodeInfoSatellite{}, nil
		}
		return nil, Error.Wrap(err)
	}

	var infos []NodeInfoSatellite
	for _, node := range nodes {
		info := func() NodeInfoSatellite {
			nodeInfoSatellite := NodeInfoSatellite{
				ID:   node.ID,
				Name: node.Name,
			}
			conn, err := service.dialer.DialNodeURL(ctx, storj.NodeURL{
				ID:      node.ID,
				Address: node.PublicAddress,
			})

			nodeStatus, nodeVersion, lastContact := service.FetchNodeMeta(ctx, node)
			if nodeStatus != StatusOnline {
				nodeInfoSatellite.Status = nodeStatus
				return nodeInfoSatellite
			}

			defer func() {
				err = errs.Combine(err, conn.Close())
			}()

			nodeClient := multinodepb.NewDRPCNodeClient(conn)
			payoutClient := multinodepb.NewDRPCPayoutClient(conn)

			header := &multinodepb.RequestHeader{
				ApiKey: node.APISecret[:],
			}

			rep, err := nodeClient.Reputation(ctx, &multinodepb.ReputationRequest{
				Header:      header,
				SatelliteId: satelliteID,
			})
			if err != nil {
				nodeInfoSatellite.Status = StatusStorageNodeInternalError
				return nodeInfoSatellite
			}

			earned, err := payoutClient.Earned(ctx, &multinodepb.EarnedRequest{Header: header})
			if err != nil {
				nodeInfoSatellite.Status = StatusStorageNodeInternalError
				return nodeInfoSatellite
			}

			nodeInfoSatellite.Version = nodeVersion.Version
			nodeInfoSatellite.LastContact = lastContact.LastContact
			nodeInfoSatellite.OnlineScore = rep.Online.Score
			nodeInfoSatellite.AuditScore = rep.Audit.Score
			nodeInfoSatellite.SuspensionScore = rep.Audit.SuspensionScore
			nodeInfoSatellite.TotalEarned = earned.Total
			nodeInfoSatellite.Status = nodeStatus

			return nodeInfoSatellite
		}()

		infos = append(infos, info)
	}

	return infos, nil
}

// TrustedSatellites returns list of unique trusted satellites node urls.
func (service *Service) TrustedSatellites(ctx context.Context) (_ storj.NodeURLs, err error) {
	defer mon.Task()(&ctx)(&err)

	listNodes, err := service.nodes.List(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	var trustedSatellites storj.NodeURLs
	for _, node := range listNodes {
		nodeStatus, _, _ := service.FetchNodeMeta(ctx, node)
		if nodeStatus != StatusOnline {
			continue
		}
		nodeURLs, err := service.trustedSatellites(ctx, node)
		if err != nil {
			service.log.Error("Failed to fetch satellite", zap.Error(err))
			continue
		}

		trustedSatellites = appendUniqueNodeURLs(trustedSatellites, nodeURLs)
	}

	return trustedSatellites, nil
}

// trustedSatellites retrieves list of trusted satellites node urls for a node.
func (service *Service) trustedSatellites(ctx context.Context, node Node) (_ storj.NodeURLs, err error) {
	defer mon.Task()(&ctx)(&err)

	conn, err := service.dialer.DialNodeURL(ctx, storj.NodeURL{
		ID:      node.ID,
		Address: node.PublicAddress,
	})
	if err != nil {
		return storj.NodeURLs{}, ErrNodeNotReachable.Wrap(err)
	}

	defer func() {
		err = errs.Combine(err, conn.Close())
	}()

	nodeClient := multinodepb.NewDRPCNodeClient(conn)

	header := &multinodepb.RequestHeader{
		ApiKey: node.APISecret[:],
	}

	resp, err := nodeClient.TrustedSatellites(ctx, &multinodepb.TrustedSatellitesRequest{Header: header})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	var nodeURLs storj.NodeURLs
	for _, url := range resp.TrustedSatellites {
		nodeURLs = append(nodeURLs, storj.NodeURL{
			ID:      url.NodeId,
			Address: url.GetAddress(),
		})
	}

	return nodeURLs, nil
}

// FetchNodeMeta information about a node status, version, and last contact.
func (service *Service) FetchNodeMeta(ctx context.Context, node Node) (_ Status, _ *multinodepb.VersionResponse, _ *multinodepb.LastContactResponse) {
	conn, err := service.dialer.DialNodeURL(ctx, storj.NodeURL{
		ID:      node.ID,
		Address: node.PublicAddress,
	})
	if err != nil {
		service.log.Error("Failed to dial the node URL:", zap.Error(err))
		return StatusNotReachable, nil, nil
	}

	defer func() {
		err = errs.Combine(err, conn.Close())
	}()

	nodeClient := multinodepb.NewDRPCNodeClient(conn)

	header := &multinodepb.RequestHeader{
		ApiKey: node.APISecret[:],
	}

	nodeVersion, err := nodeClient.Version(ctx, &multinodepb.VersionRequest{Header: header})
	if err != nil {
		if rpcstatus.Code(err) == rpcstatus.Unauthenticated {
			return StatusUnauthorized, nil, nil
		}

		service.log.Error("Could not fetch the version of the node:", zap.Error(err))
		return StatusStorageNodeInternalError, nil, nil
	}

	lastContact, err := nodeClient.LastContact(ctx, &multinodepb.LastContactRequest{Header: header})
	if err != nil {
		// TODO: since rpcstatus.Unauthenticated was checked in nodeVersion this sort of error can be caused
		// only if new apikey was issued during ListInfos method call.
		service.log.Error("Could not fetch the lastcontact with the node:", zap.Error(err))
		return StatusStorageNodeInternalError, nodeVersion, nil
	}

	now := time.Now().UTC()

	if now.Sub(lastContact.LastContact) < time.Hour*3 {
		return StatusOnline, nodeVersion, lastContact
	}

	return StatusOffline, nodeVersion, lastContact
}

// appendUniqueNodeURLs appends unique node urls from incoming slice.
func appendUniqueNodeURLs(slice storj.NodeURLs, nodeURLs storj.NodeURLs) storj.NodeURLs {
	for _, nodeURL := range nodeURLs {
		slice = appendUniqueNodeURL(slice, nodeURL)
	}

	return slice
}

// appendUniqueNodeURL appends node url if it is unique.
func appendUniqueNodeURL(slice storj.NodeURLs, nodeURL storj.NodeURL) storj.NodeURLs {
	for _, existing := range slice {
		if bytes.Equal(existing.ID.Bytes(), nodeURL.ID.Bytes()) {
			return slice
		}
	}

	slice = append(slice, nodeURL)
	return slice
}
