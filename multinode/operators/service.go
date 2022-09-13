// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package operators

import (
	"context"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/storj/multinode/nodes"
	"storj.io/storj/private/multinodepb"
)

// MaxOperatorsOnPage defines maximum limit on operators page.
const MaxOperatorsOnPage = 5

var (
	mon = monkit.Package()
	// Error is an error class for operators service error.
	Error = errs.Class("operators")
)

// Service exposes all operators related logic.
//
// architecture: Service
type Service struct {
	log    *zap.Logger
	dialer rpc.Dialer
	nodes  nodes.DB
}

// NewService creates new instance of Service.
func NewService(log *zap.Logger, dialer rpc.Dialer, nodes nodes.DB) *Service {
	return &Service{
		log:    log,
		dialer: dialer,
		nodes:  nodes,
	}
}

// ListPaginated returns paginated list of operators.
func (service *Service) ListPaginated(ctx context.Context, cursor Cursor) (_ Page, err error) {
	defer mon.Task()(&ctx)(&err)
	if cursor.Limit > MaxOperatorsOnPage {
		cursor.Limit = MaxOperatorsOnPage
	}
	if cursor.Limit < 1 {
		cursor.Limit = 1
	}
	if cursor.Page == 0 {
		return Page{}, Error.Wrap(errs.New("page can not be 0"))
	}
	page, err := service.nodes.ListPaged(ctx, nodes.Cursor{
		Limit: cursor.Limit,
		Page:  cursor.Page,
	})
	if err != nil {
		return Page{}, Error.Wrap(err)
	}

	var operators []Operator
	for _, node := range page.Nodes {
		operator, err := service.GetOperator(ctx, node)
		if err != nil {
			if nodes.ErrNodeNotReachable.Has(err) {
				continue
			}

			return Page{}, Error.Wrap(err)
		}
		operators = append(operators, operator)
	}

	return Page{
		Operators:   operators,
		Offset:      page.Offset,
		Limit:       page.Limit,
		CurrentPage: page.CurrentPage,
		PageCount:   page.PageCount,
		TotalCount:  page.TotalCount,
	}, nil
}

// GetOperator retrieves operator form node via rpc.
func (service *Service) GetOperator(ctx context.Context, node nodes.Node) (_ Operator, err error) {
	defer mon.Task()(&ctx)(&err)

	conn, err := service.dialer.DialNodeURL(ctx, storj.NodeURL{
		ID:      node.ID,
		Address: node.PublicAddress,
	})
	if err != nil {
		return Operator{}, nodes.ErrNodeNotReachable.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, conn.Close())
	}()

	nodeClient := multinodepb.NewDRPCNodeClient(conn)
	payoutClient := multinodepb.NewDRPCPayoutClient(conn)
	header := &multinodepb.RequestHeader{
		ApiKey: node.APISecret[:],
	}

	operatorResponse, err := nodeClient.Operator(ctx, &multinodepb.OperatorRequest{Header: header})
	if err != nil {
		return Operator{}, Error.Wrap(err)
	}
	undistributedResponse, err := payoutClient.Undistributed(ctx, &multinodepb.UndistributedRequest{Header: header})
	if err != nil {
		return Operator{}, Error.Wrap(err)
	}

	return Operator{
		NodeID:         node.ID,
		Email:          operatorResponse.Email,
		Wallet:         operatorResponse.Wallet,
		WalletFeatures: operatorResponse.WalletFeatures,
		Undistributed:  undistributedResponse.Total,
	}, nil
}
