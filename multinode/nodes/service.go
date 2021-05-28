// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package nodes

import (
	"bytes"
	"context"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/storj/private/multinodepb"
)

var (
	mon = monkit.Package()

	// Error is an error class for nodes service error.
	Error = errs.Class("nodes")
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
func (service *Service) Add(ctx context.Context, id storj.NodeID, apiSecret []byte, publicAddress string) (err error) {
	defer mon.Task()(&ctx)(&err)
	return Error.Wrap(service.nodes.Add(ctx, id, apiSecret, publicAddress))
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
		info, err := func() (_ NodeInfo, err error) {
			conn, err := service.dialer.DialNodeURL(ctx, storj.NodeURL{
				ID:      node.ID,
				Address: node.PublicAddress,
			})
			if err != nil {
				return NodeInfo{}, Error.Wrap(err)
			}

			defer func() {
				err = errs.Combine(err, conn.Close())
			}()

			nodeClient := multinodepb.NewDRPCNodeClient(conn)
			storageClient := multinodepb.NewDRPCStorageClient(conn)
			bandwidthClient := multinodepb.NewDRPCBandwidthClient(conn)
			payoutClient := multinodepb.NewDRPCPayoutClient(conn)

			header := &multinodepb.RequestHeader{
				ApiKey: node.APISecret,
			}

			nodeVersion, err := nodeClient.Version(ctx, &multinodepb.VersionRequest{Header: header})
			if err != nil {
				return NodeInfo{}, Error.Wrap(err)
			}

			lastContact, err := nodeClient.LastContact(ctx, &multinodepb.LastContactRequest{Header: header})
			if err != nil {
				return NodeInfo{}, Error.Wrap(err)
			}

			diskSpace, err := storageClient.DiskSpace(ctx, &multinodepb.DiskSpaceRequest{Header: header})
			if err != nil {
				return NodeInfo{}, Error.Wrap(err)
			}

			earned, err := payoutClient.Earned(ctx, &multinodepb.EarnedRequest{Header: header})
			if err != nil {
				return NodeInfo{}, Error.Wrap(err)
			}

			bandwidthSummaryRequest := &multinodepb.BandwidthMonthSummaryRequest{
				Header: header,
			}
			bandwidthSummary, err := bandwidthClient.MonthSummary(ctx, bandwidthSummaryRequest)
			if err != nil {
				return NodeInfo{}, Error.Wrap(err)
			}

			return NodeInfo{
				ID:            node.ID,
				Name:          node.Name,
				Version:       nodeVersion.Version,
				LastContact:   lastContact.LastContact,
				DiskSpaceUsed: diskSpace.GetUsedPieces() + diskSpace.GetUsedTrash(),
				DiskSpaceLeft: diskSpace.GetAvailable(),
				BandwidthUsed: bandwidthSummary.GetUsed(),
				TotalEarned:   earned.Total,
			}, nil
		}()
		if err != nil {
			return nil, Error.Wrap(err)
		}

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
		info, err := func() (_ NodeInfoSatellite, err error) {
			conn, err := service.dialer.DialNodeURL(ctx, storj.NodeURL{
				ID:      node.ID,
				Address: node.PublicAddress,
			})
			if err != nil {
				return NodeInfoSatellite{}, Error.Wrap(err)
			}

			defer func() {
				err = errs.Combine(err, conn.Close())
			}()

			nodeClient := multinodepb.NewDRPCNodeClient(conn)
			payoutClient := multinodepb.NewDRPCPayoutClient(conn)

			header := &multinodepb.RequestHeader{
				ApiKey: node.APISecret,
			}

			nodeVersion, err := nodeClient.Version(ctx, &multinodepb.VersionRequest{Header: header})
			if err != nil {
				return NodeInfoSatellite{}, Error.Wrap(err)
			}

			lastContact, err := nodeClient.LastContact(ctx, &multinodepb.LastContactRequest{Header: header})
			if err != nil {
				return NodeInfoSatellite{}, Error.Wrap(err)
			}

			rep, err := nodeClient.Reputation(ctx, &multinodepb.ReputationRequest{
				Header:      header,
				SatelliteId: satelliteID,
			})
			if err != nil {
				return NodeInfoSatellite{}, Error.Wrap(err)
			}

			earned, err := payoutClient.Earned(ctx, &multinodepb.EarnedRequest{Header: header})
			if err != nil {
				return NodeInfoSatellite{}, Error.Wrap(err)
			}

			return NodeInfoSatellite{
				ID:              node.ID,
				Name:            node.Name,
				Version:         nodeVersion.Version,
				LastContact:     lastContact.LastContact,
				OnlineScore:     rep.Online.Score,
				AuditScore:      rep.Audit.Score,
				SuspensionScore: rep.Audit.SuspensionScore,
				TotalEarned:     earned.Total,
			}, nil
		}()
		if err != nil {
			return nil, Error.Wrap(err)
		}

		infos = append(infos, info)
	}

	return infos, nil
}

// TrustedSatellites returns list of unique trusted satellites node urls.
func (service *Service) TrustedSatellites(ctx context.Context) (_ storj.NodeURLs, err error) {
	defer mon.Task()(&ctx)(&err)

	nodes, err := service.nodes.List(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	var trustedSatellites storj.NodeURLs
	for _, node := range nodes {
		nodeURLs, err := service.trustedSatellites(ctx, node)
		if err != nil {
			return nil, Error.Wrap(err)
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
		return nil, Error.Wrap(err)
	}

	defer func() {
		err = errs.Combine(err, conn.Close())
	}()

	nodeClient := multinodepb.NewDRPCNodeClient(conn)

	header := &multinodepb.RequestHeader{
		ApiKey: node.APISecret,
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
