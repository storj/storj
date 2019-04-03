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
	"storj.io/storj/satellite/orders"
	"storj.io/storj/uplink/piecestore"
)

var (
	mon = monkit.Package()
)

// Share represents required information about an audited share
type Share struct {
	Error    error
	PieceNum int
	Data     []byte
}

// Verifier helps verify the correctness of a given stripe
type Verifier struct {
	orders  *orders.Service
	auditor *identity.PeerIdentity

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

	minBytesPerSecond memory.Size
}

// newDefaultDownloader creates a defaultDownloader
func newDefaultDownloader(log *zap.Logger, transport transport.Client, overlay *overlay.Cache, id *identity.FullIdentity, minBytesPerSecond memory.Size) *defaultDownloader {
	return &defaultDownloader{log: log, transport: transport, overlay: overlay, minBytesPerSecond: minBytesPerSecond}
}

// NewVerifier creates a Verifier
func NewVerifier(log *zap.Logger, transport transport.Client, overlay *overlay.Cache, orders *orders.Service, id *identity.FullIdentity, minBytesPerSecond memory.Size) *Verifier {
	return &Verifier{downloader: newDefaultDownloader(log, transport, overlay, id, minBytesPerSecond), orders: orders, auditor: id.PeerIdentity()}
}

// Verify downloads shares then verifies the data correctness at the given stripe
func (verifier *Verifier) Verify(ctx context.Context, stripe *Stripe) (verifiedNodes *RecordAuditsInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	pointer := stripe.Segment
	shareSize := pointer.GetRemote().GetRedundancy().GetErasureShareSize()
	bucketID := createBucketID(stripe.SegmentPath)

	orderLimits, err := verifier.orders.CreateAuditOrderLimits(ctx, verifier.auditor, bucketID, pointer)
	if err != nil {
		return nil, err
	}

	shares, nodes, err := verifier.downloader.DownloadShares(ctx, orderLimits, stripe.Index, shareSize)
	if err != nil {
		return nil, err
	}

	var offlineNodes storj.NodeIDList
	var failedNodes storj.NodeIDList
	sharesToAudit := make(map[int]Share)

	for pieceNum, share := range shares {
		if shares[pieceNum].Error != nil {
			if shares[pieceNum].Error == context.DeadlineExceeded ||
				!transport.Error.Has(shares[pieceNum].Error) {
				failedNodes = append(failedNodes, nodes[pieceNum])
			} else {
				offlineNodes = append(offlineNodes, nodes[pieceNum])
			}
		} else {
			sharesToAudit[pieceNum] = share
		}
	}

	required := int(pointer.Remote.Redundancy.GetMinReq())
	total := int(pointer.Remote.Redundancy.GetTotal())

	if len(sharesToAudit) < required {
		return &RecordAuditsInfo{
			SuccessNodeIDs: nil,
			FailNodeIDs:    failedNodes,
			OfflineNodeIDs: offlineNodes,
		}, nil
	}

	pieceNums, err := auditShares(ctx, required, total, sharesToAudit)
	if err != nil {
		return nil, err
	}

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

// getShare use piece store client to download shares from nodes
func (d *defaultDownloader) getShare(ctx context.Context, limit *pb.AddressedOrderLimit, stripeIndex int64, shareSize int32, pieceNum int) (share Share, err error) {
	defer mon.Task()(&ctx)(&err)

	bandwidthMsgSize := shareSize

	// determines number of seconds allotted for receiving data from a storage node
	timedCtx := ctx
	if d.minBytesPerSecond > 0 {
		maxTransferTime := time.Duration(int64(time.Second) * int64(bandwidthMsgSize) / d.minBytesPerSecond.Int64())
		var cancel func()
		timedCtx, cancel = context.WithTimeout(ctx, maxTransferTime)
		defer cancel()
	}

	storageNodeID := limit.GetLimit().StorageNodeId

	conn, err := d.transport.DialNode(timedCtx, &pb.Node{
		Id:      storageNodeID,
		Address: limit.GetStorageNodeAddress(),
		Type:    pb.NodeType_STORAGE,
	})
	if err != nil {
		return Share{}, err
	}
	ps := piecestore.NewClient(
		d.log.Named(storageNodeID.String()),
		signing.SignerFromFullIdentity(d.transport.Identity()),
		conn,
		piecestore.DefaultConfig,
	)
	defer func() {
		err := ps.Close()
		if err != nil {
			d.log.Error("audit verifier failed to close conn to node: %+v", zap.Error(err))
		}
	}()

	offset := int64(shareSize) * stripeIndex

	downloader, err := ps.Download(timedCtx, limit.GetLimit(), offset, int64(shareSize))
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

// makeCopies takes in a map of audit Shares and deep copies their data to a slice of infectious Shares
func makeCopies(ctx context.Context, originals map[int]Share) (copies []infectious.Share, err error) {
	defer mon.Task()(&ctx)(&err)
	copies = make([]infectious.Share, 0, len(originals))
	for _, original := range originals {
		copies = append(copies, infectious.Share{
			Data:   append([]byte{}, original.Data...),
			Number: original.PieceNum})
	}
	return copies, nil
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

func createBucketID(path storj.Path) []byte {
	comps := storj.SplitPath(path)
	if len(comps) < 3 {
		return nil
	}
	// project_id/bucket_name
	return []byte(storj.JoinPaths(comps[0], comps[2]))
}
