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
	"storj.io/storj/satellite/admin/auditlogger"
	"storj.io/storj/satellite/admin/changehistory"
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

// UndisqualifyNodeRequest represents a request to un-disqualify a storage node.
type UndisqualifyNodeRequest struct {
	Reason string `json:"reason"`
}

// DisqualifyNodeRequest represents a request to disqualify a storage node.
type DisqualifyNodeRequest struct {
	DisqualificationReason string `json:"disqualificationReason"` // one of: "audit_failure", "suspension", "node_offline", "unknown"
	Reason                 string `json:"reason"`
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

	if s.tenantID != nil {
		return apiError(http.StatusForbidden, errs.New("not available for tenant-scoped admin"))
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

// validateNodeModifyRequest checks auth, reason, tenant scope, parses the node ID, and fetches the node.
func (s *Service) validateNodeModifyRequest(ctx context.Context, authInfo *AuthInfo, reason, nodeID string) (*NodeFullInfo, storj.NodeID, api.HTTPError) {
	if authInfo == nil {
		return nil, storj.NodeID{}, api.HTTPError{Status: http.StatusUnauthorized, Err: Error.New("not authorized")}
	}

	if reason == "" {
		return nil, storj.NodeID{}, api.HTTPError{Status: http.StatusBadRequest, Err: Error.New("reason is required")}
	}

	if s.tenantID != nil {
		return nil, storj.NodeID{}, api.HTTPError{Status: http.StatusForbidden, Err: errs.New("not available for tenant-scoped admin")}
	}

	id, err := storj.NodeIDFromString(nodeID)
	if err != nil {
		return nil, storj.NodeID{}, api.HTTPError{Status: http.StatusBadRequest, Err: Error.Wrap(errs.New("invalid node ID"))}
	}

	node, apiErr := s.getNodeByID(ctx, id)
	if apiErr.Err != nil {
		return nil, storj.NodeID{}, apiErr
	}

	return node, id, api.HTTPError{}
}

// UndisqualifyNode clears the disqualification status of a storage node by its ID.
func (s *Service) UndisqualifyNode(ctx context.Context, authInfo *AuthInfo, nodeID string, request UndisqualifyNodeRequest) api.HTTPError {
	var err error
	defer mon.Task()(&ctx)(&err)

	node, id, apiErr := s.validateNodeModifyRequest(ctx, authInfo, request.Reason, nodeID)
	if apiErr.Err != nil {
		return apiErr
	}

	if node.Disqualified == nil {
		return api.HTTPError{Status: http.StatusConflict, Err: Error.New("node is not disqualified")}
	}

	err = s.overlayDB.UndisqualifyNode(ctx, id)
	if err != nil {
		return api.HTTPError{Status: http.StatusInternalServerError, Err: Error.Wrap(err)}
	}

	after := *node
	after.Disqualified = nil
	after.DisqualificationReason = nil

	s.auditLogger.EnqueueChangeEvent(auditlogger.Event{
		Action:     "undisqualify_node",
		AdminEmail: authInfo.Email,
		ItemType:   changehistory.ItemTypeUser,
		Reason:     request.Reason,
		Before:     node,
		After:      &after,
		Timestamp:  s.nowFn(),
	})

	return api.HTTPError{}
}

// DisqualifyNode sets the disqualification status of a storage node by its ID.
func (s *Service) DisqualifyNode(ctx context.Context, authInfo *AuthInfo, nodeID string, request DisqualifyNodeRequest) api.HTTPError {
	var err error
	defer mon.Task()(&ctx)(&err)

	node, id, apiErr := s.validateNodeModifyRequest(ctx, authInfo, request.Reason, nodeID)
	if apiErr.Err != nil {
		return apiErr
	}

	if node.Disqualified != nil {
		return api.HTTPError{Status: http.StatusConflict, Err: Error.New("node is already disqualified")}
	}

	dqReason := disqualificationReasonFromString(request.DisqualificationReason)
	dqAt := s.nowFn()
	_, err = s.overlayDB.DisqualifyNode(ctx, id, dqAt, dqReason)
	if err != nil {
		return api.HTTPError{Status: http.StatusInternalServerError, Err: Error.Wrap(err)}
	}

	dqReasonStr := disqualificationReasonToString(dqReason)
	after := *node
	after.Disqualified = &dqAt
	after.DisqualificationReason = &dqReasonStr

	s.auditLogger.EnqueueChangeEvent(auditlogger.Event{
		Action:     "disqualify_node",
		AdminEmail: authInfo.Email,
		ItemType:   changehistory.ItemTypeUser,
		Reason:     request.Reason,
		Before:     node,
		After:      &after,
		Timestamp:  dqAt,
	})

	return api.HTTPError{}
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

func disqualificationReasonFromString(s string) overlay.DisqualificationReason {
	switch s {
	case "audit_failure":
		return overlay.DisqualificationReasonAuditFailure
	case "suspension":
		return overlay.DisqualificationReasonSuspension
	case "node_offline":
		return overlay.DisqualificationReasonNodeOffline
	default:
		return overlay.DisqualificationReasonUnknown
	}
}
