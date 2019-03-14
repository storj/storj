// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"bytes"
	"context"
	"io"
	"time"

	"github.com/vivint/infectious"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/uplink/piecestore"
)

var (
	mon = monkit.Package()

	// todo(nat): make this configurable
	// this is minimum bytes per second acceptable transfer rate
	// from a storage node to a satellite
	submissionSpeed = 128 * memory.B
)

// Share represents required information about an audited share
type Share struct {
	Error    error
	PieceNum int
	Data     []byte
}

// Verifier helps verify the correctness of a given stripe
type Verifier struct {
	downloader downloader
}

type downloader interface {
	DownloadShares(ctx context.Context, limits []*pb.AddressedOrderLimit, stripeIndex int64, shareSize int32) (shares map[int]Share, nodes map[int]storj.NodeID, err error)
}

// defaultDownloader downloads shares from networked storage nodes
type defaultDownloader struct {
	log       *zap.Logger
	transport transport.Client
	overlay   *overlay.Cache
	reporter
}

// newDefaultDownloader creates a defaultDownloader
func newDefaultDownloader(log *zap.Logger, transport transport.Client, overlay *overlay.Cache, id *identity.FullIdentity) *defaultDownloader {
	return &defaultDownloader{log: log, transport: transport, overlay: overlay}
}

// NewVerifier creates a Verifier
func NewVerifier(log *zap.Logger, transport transport.Client, overlay *overlay.Cache, id *identity.FullIdentity) *Verifier {
	return &Verifier{downloader: newDefaultDownloader(log, transport, overlay, id)}
}

// getShare use piece store client to download shares from nodes
func (d *defaultDownloader) getShare(ctx context.Context, limit *pb.AddressedOrderLimit, stripeIndex int64, shareSize int32, pieceNum int) (share Share, err error) {
	defer mon.Task()(&ctx)(&err)

	storageNodeID := limit.GetLimit().StorageNodeId

	conn, err := d.transport.DialNode(ctx, &pb.Node{
		Id:      storageNodeID,
		Address: limit.GetStorageNodeAddress(),
		Type:    pb.NodeType_STORAGE,
	})
	if err != nil {
		return Share{}, err
	}

	bandwidthMsgSize := shareSize
	seconds := bandwidthMsgSize / submissionSpeed.Int32()

	allottedTime := time.Now().Local().Add(time.Second * time.Duration(seconds))

	newCtx, cancel := context.WithDeadline(ctx, allottedTime)
	defer cancel()

	ps := piecestore.NewClient(
		d.log.Named(storageNodeID.String()),
		signing.SignerFromFullIdentity(d.transport.Identity()),
		conn,
		piecestore.DefaultConfig,
	)

	offset := int64(shareSize) * stripeIndex

	downloader, err := ps.Download(newCtx, limit.GetLimit(), offset, int64(shareSize))
	if err != nil {
		return Share{}, err
	}
	defer func() { err = errs.Combine(err, downloader.Close()) }()

	buf := make([]byte, shareSize)
	_, err = io.ReadFull(downloader, buf)
	if err != nil {
		return Share{}, err
	}

	return Share{
		Error:    nil,
		PieceNum: pieceNum,
		Data:     buf,
	}, nil
}

// Download Shares downloads shares from the nodes where remote pieces are located
func (d *defaultDownloader) DownloadShares(ctx context.Context, limits []*pb.AddressedOrderLimit, stripeIndex int64, shareSize int32) (shares map[int]Share, nodes map[int]storj.NodeID, err error) {
	defer mon.Task()(&ctx)(&err)

	shares = make(map[int]Share, len(limits))
	nodes = make(map[int]storj.NodeID, len(limits))

	for i, limit := range limits {
		if limit == nil {
			continue
		}

		share, err := d.getShare(ctx, limit, stripeIndex, shareSize, i)
		if err != nil {
			share = Share{
				Error:    err,
				PieceNum: i,
				Data:     nil,
			}
		}

		shares[share.PieceNum] = share
		nodes[share.PieceNum] = limit.GetLimit().StorageNodeId
	}

	return shares, nodes, nil
}

func makeCopies(ctx context.Context, originals map[int]Share) (copies []infectious.Share, err error) {
	defer mon.Task()(&ctx)(&err)
	copies = make([]infectious.Share, 0, len(originals))
	for _, original := range originals {
		if original.Error != nil {
			continue
		}
		copies = append(copies, infectious.Share{
			Data:   append([]byte{}, original.Data...),
			Number: original.PieceNum})
	}
	return copies, nil
}

// auditShares takes the downloaded shares and uses infectious's Correct function to check that they
// haven't been altered. auditShares returns a slice containing the piece numbers of altered shares.
func auditShares(ctx context.Context, required, total int, originals map[int]Share) (pieceNums []int, err error) {
	defer mon.Task()(&ctx)(&err)
	f, err := infectious.NewFEC(required, total)
	if err != nil {
		return nil, err
	}

	copies, err := makeCopies(ctx, originals)
	if err != nil {
		return nil, err
	}

	err = f.Correct(copies)
	if err != nil {
		return nil, err
	}
	for _, share := range copies {
		if !bytes.Equal(originals[share.Number].Data, share.Data) {
			pieceNums = append(pieceNums, share.Number)
		}
	}
	return pieceNums, nil
}

// Verify downloads shares then verifies the data correctness at the given stripe
func (verifier *Verifier) Verify(ctx context.Context, stripe *Stripe) (verifiedNodes *RecordAuditsInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	pointer := stripe.Segment
	shareSize := pointer.GetRemote().GetRedundancy().GetErasureShareSize()

	shares, nodes, err := verifier.downloader.DownloadShares(ctx, stripe.OrderLimits, stripe.Index, shareSize)
	if err != nil {
		return nil, err
	}

	var offlineNodes storj.NodeIDList
	for pieceNum := range shares {
		if shares[pieceNum].Error != nil {
			offlineNodes = append(offlineNodes, nodes[pieceNum])
		}
	}

	required := int(pointer.Remote.Redundancy.GetMinReq())
	total := int(pointer.Remote.Redundancy.GetTotal())
	pieceNums, err := auditShares(ctx, required, total, shares)
	if err != nil {
		return nil, err
	}

	var failedNodes storj.NodeIDList
	for _, pieceNum := range pieceNums {
		failedNodes = append(failedNodes, nodes[pieceNum])
	}

	successNodes := getSuccessNodes(ctx, nodes, failedNodes, offlineNodes)

	return &RecordAuditsInfo{
		SuccessNodeIDs: successNodes,
		FailNodeIDs:    failedNodes,
		OfflineNodeIDs: offlineNodes,
	}, nil
}

// getSuccessNodes uses the failed nodes and offline nodes arrays to determine which nodes passed the audit
func getSuccessNodes(ctx context.Context, nodes map[int]storj.NodeID, failedNodes, offlineNodes storj.NodeIDList) (successNodes storj.NodeIDList) {
	fails := make(map[storj.NodeID]bool)
	for _, fail := range failedNodes {
		fails[fail] = true
	}
	for _, offline := range offlineNodes {
		fails[offline] = true
	}

	for _, node := range nodes {
		if !fails[node] {
			successNodes = append(successNodes, node)
		}
	}

	return successNodes
}
