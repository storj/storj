// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/private/api"
	"storj.io/storj/satellite/overlay"
)

// NodeMinInfo holds minimal information about a storage node for account overview.
type NodeMinInfo struct {
	ID           string    `json:"id"`
	Online       bool      `json:"online"`
	Disqualified bool      `json:"disqualified"`
	CreatedAt    time.Time `json:"createdAt"`
}

// NodeFullInfo holds detailed information about a storage node.
type NodeFullInfo struct {
	ID                     string     `json:"id"`
	Address                string     `json:"address"`
	Email                  string     `json:"email"`
	Wallet                 string     `json:"wallet"`
	WalletFeatures         []string   `json:"walletFeatures"`
	LastContactSuccess     time.Time  `json:"lastContactSuccess"`
	LastContactFailure     time.Time  `json:"lastContactFailure"`
	VettedAt               *time.Time `json:"vettedAt"`
	Disqualified           *time.Time `json:"disqualified"`
	DisqualificationReason *string    `json:"disqualificationReason"`
	FreeDisk               int64      `json:"freeDisk"`
	PieceCount             int64      `json:"pieceCount"`
	CreatedAt              time.Time  `json:"createdAt"`
	Version                string     `json:"version"`
	CountryCode            string     `json:"countryCode"`
	ExitInitiatedAt        *time.Time `json:"exitInitiatedAt"`
	ExitFinishedAt         *time.Time `json:"exitFinishedAt"`
	ExitSuccess            bool       `json:"exitSuccess"`
}

func (s *Service) getNodesByEmail(ctx context.Context, email string) ([]NodeMinInfo, error) {
	var err error
	defer mon.Task()(&ctx)(&err)

	nodes, _, err := s.overlayDB.GetNodesByEmail(ctx, overlay.GetNodesByEmailOptions{Email: email, Limit: 100})
	if err != nil {
		return nil, err
	}

	result := make([]NodeMinInfo, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, NodeMinInfo{
			ID:           node.Id.String(),
			Online:       time.Since(node.Reputation.LastContactSuccess) < 4*time.Hour,
			Disqualified: node.Disqualified != nil,
			CreatedAt:    node.CreatedAt,
		})
	}

	return result, nil
}

// GetNodeInfo returns full information about a specific node by ID.
func (s *Service) GetNodeInfo(ctx context.Context, nodeID string) (*NodeFullInfo, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	apiError := func(status int, err error) (*NodeFullInfo, api.HTTPError) {
		return nil, api.HTTPError{
			Status: status, Err: Error.Wrap(err),
		}
	}

	id, err := storj.NodeIDFromString(nodeID)
	if err != nil {
		return apiError(http.StatusBadRequest, errs.New("invalid node ID"))
	}

	return s.getNodeByID(ctx, id)
}

func (s *Service) getNodeByID(ctx context.Context, nodeID storj.NodeID) (*NodeFullInfo, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)
	apiError := func(status int, err error) (*NodeFullInfo, api.HTTPError) {
		return nil, api.HTTPError{
			Status: status, Err: Error.Wrap(err),
		}
	}

	node, err := s.overlayDB.Get(ctx, nodeID)
	if err != nil {
		if overlay.ErrNodeNotFound.Has(err) {
			return apiError(http.StatusNotFound, errs.New("node not found"))
		}
		return apiError(http.StatusInternalServerError, err)
	}

	var dqReason *string
	if node.DisqualificationReason != nil {
		reason := disqualificationReasonToString(*node.DisqualificationReason)
		dqReason = &reason
	}

	return &NodeFullInfo{
		ID:                     node.Id.String(),
		Address:                node.Address.Address,
		Email:                  node.Operator.Email,
		Wallet:                 node.Operator.Wallet,
		WalletFeatures:         node.Operator.WalletFeatures,
		LastContactSuccess:     node.Reputation.LastContactSuccess,
		LastContactFailure:     node.Reputation.LastContactFailure,
		VettedAt:               node.Reputation.Status.VettedAt,
		Disqualified:           node.Disqualified,
		DisqualificationReason: dqReason,
		FreeDisk:               node.Capacity.FreeDisk,
		PieceCount:             node.PieceCount,
		CreatedAt:              node.CreatedAt,
		Version:                node.Version.Version,
		CountryCode:            node.CountryCode.String(),
		ExitInitiatedAt:        node.ExitStatus.ExitInitiatedAt,
		ExitFinishedAt:         node.ExitStatus.ExitFinishedAt,
		ExitSuccess:            node.ExitStatus.ExitSuccess,
	}, api.HTTPError{}
}

func disqualificationReasonToString(reason overlay.DisqualificationReason) string {
	switch reason {
	case overlay.DisqualificationReasonAuditFailure:
		return "Audit Failure"
	case overlay.DisqualificationReasonSuspension:
		return "Suspension"
	case overlay.DisqualificationReasonNodeOffline:
		return "Node Offline"
	default:
		return fmt.Sprintf("Unknown (%d)", reason)
	}
}
