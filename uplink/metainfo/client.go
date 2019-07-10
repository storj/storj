// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/auth/grpcauth"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/storage"
)

var (
	mon = monkit.Package()

	// Error is the errs class of standard metainfo errors
	Error = errs.Class("metainfo error")
)

// Client creates a grpcClient
type Client struct {
	client pb.MetainfoClient
	conn   *grpc.ClientConn
}

// ListItem is a single item in a listing
type ListItem struct {
	Path     storj.Path
	Pointer  *pb.Pointer
	IsPrefix bool
}

// New used as a public function
func New(client pb.MetainfoClient) *Client {
	return &Client{
		client: client,
	}
}

// Dial dials to metainfo endpoint with the specified api key.
func Dial(ctx context.Context, tc transport.Client, address string, apiKey string) (*Client, error) {
	apiKeyInjector := grpcauth.NewAPIKeyInjector(apiKey)
	conn, err := tc.DialAddress(
		ctx,
		address,
		grpc.WithUnaryInterceptor(apiKeyInjector),
	)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &Client{
		client: pb.NewMetainfoClient(conn),
		conn:   conn,
	}, nil
}

// Close closes the dialed connection.
func (client *Client) Close() error {
	if client.conn != nil {
		return Error.Wrap(client.conn.Close())
	}
	return nil
}

// CreateSegment requests the order limits for creating a new segment
func (client *Client) CreateSegment(ctx context.Context, bucket string, path storj.Path, segmentIndex int64, redundancy *pb.RedundancyScheme, maxEncryptedSegmentSize int64, expiration time.Time) (limits []*pb.AddressedOrderLimit, rootPieceID storj.PieceID, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.CreateSegmentOld(ctx, &pb.SegmentWriteRequestOld{
		Bucket:                  []byte(bucket),
		Path:                    []byte(path),
		Segment:                 segmentIndex,
		Redundancy:              redundancy,
		MaxEncryptedSegmentSize: maxEncryptedSegmentSize,
		Expiration:              expiration,
	})
	if err != nil {
		return nil, rootPieceID, Error.Wrap(err)
	}

	return response.GetAddressedLimits(), response.RootPieceId, nil
}

// CommitSegment requests to store the pointer for the segment
func (client *Client) CommitSegment(ctx context.Context, bucket string, path storj.Path, segmentIndex int64, pointer *pb.Pointer, originalLimits []*pb.OrderLimit) (savedPointer *pb.Pointer, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.CommitSegmentOld(ctx, &pb.SegmentCommitRequestOld{
		Bucket:         []byte(bucket),
		Path:           []byte(path),
		Segment:        segmentIndex,
		Pointer:        pointer,
		OriginalLimits: originalLimits,
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return response.GetPointer(), nil
}

// SegmentInfo requests the pointer of a segment
func (client *Client) SegmentInfo(ctx context.Context, bucket string, path storj.Path, segmentIndex int64) (pointer *pb.Pointer, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.SegmentInfoOld(ctx, &pb.SegmentInfoRequestOld{
		Bucket:  []byte(bucket),
		Path:    []byte(path),
		Segment: segmentIndex,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, storage.ErrKeyNotFound.Wrap(err)
		}
		return nil, Error.Wrap(err)
	}

	return response.GetPointer(), nil
}

// ReadSegment requests the order limits for reading a segment
func (client *Client) ReadSegment(ctx context.Context, bucket string, path storj.Path, segmentIndex int64) (pointer *pb.Pointer, limits []*pb.AddressedOrderLimit, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.DownloadSegmentOld(ctx, &pb.SegmentDownloadRequestOld{
		Bucket:  []byte(bucket),
		Path:    []byte(path),
		Segment: segmentIndex,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, nil, storage.ErrKeyNotFound.Wrap(err)
		}
		return nil, nil, Error.Wrap(err)
	}

	return response.GetPointer(), sortLimits(response.GetAddressedLimits(), response.GetPointer()), nil
}

// sortLimits sorts order limits and fill missing ones with nil values
func sortLimits(limits []*pb.AddressedOrderLimit, pointer *pb.Pointer) []*pb.AddressedOrderLimit {
	sorted := make([]*pb.AddressedOrderLimit, pointer.GetRemote().GetRedundancy().GetTotal())
	for _, piece := range pointer.GetRemote().GetRemotePieces() {
		sorted[piece.GetPieceNum()] = getLimitByStorageNodeID(limits, piece.NodeId)
	}
	return sorted
}

func getLimitByStorageNodeID(limits []*pb.AddressedOrderLimit, storageNodeID storj.NodeID) *pb.AddressedOrderLimit {
	for _, limit := range limits {
		if limit.GetLimit().StorageNodeId == storageNodeID {
			return limit
		}
	}
	return nil
}

// DeleteSegment requests the order limits for deleting a segment
func (client *Client) DeleteSegment(ctx context.Context, bucket string, path storj.Path, segmentIndex int64) (limits []*pb.AddressedOrderLimit, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.DeleteSegmentOld(ctx, &pb.SegmentDeleteRequestOld{
		Bucket:  []byte(bucket),
		Path:    []byte(path),
		Segment: segmentIndex,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, storage.ErrKeyNotFound.Wrap(err)
		}
		return nil, Error.Wrap(err)
	}

	return response.GetAddressedLimits(), nil
}

// ListSegments lists the available segments
func (client *Client) ListSegments(ctx context.Context, bucket string, prefix, startAfter, endBefore storj.Path, recursive bool, limit int32, metaFlags uint32) (items []ListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.ListSegmentsOld(ctx, &pb.ListSegmentsRequestOld{
		Bucket:     []byte(bucket),
		Prefix:     []byte(prefix),
		StartAfter: []byte(startAfter),
		EndBefore:  []byte(endBefore),
		Recursive:  recursive,
		Limit:      limit,
		MetaFlags:  metaFlags,
	})
	if err != nil {
		return nil, false, Error.Wrap(err)
	}

	list := response.GetItems()
	items = make([]ListItem, len(list))
	for i, item := range list {
		items[i] = ListItem{
			Path:     storj.Path(item.GetPath()),
			Pointer:  item.GetPointer(),
			IsPrefix: item.IsPrefix,
		}
	}

	return items, response.GetMore(), nil
}

// SetAttribution tries to set the attribution information on the bucket.
func (client *Client) SetAttribution(ctx context.Context, bucket string, partnerID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = client.client.SetAttributionOld(ctx, &pb.SetAttributionRequestOld{
		PartnerId:  partnerID[:], // TODO: implement storj.UUID that can be sent using pb
		BucketName: []byte(bucket),
	})

	return err
}

// GetProjectInfo gets the ProjectInfo for the api key associated with the metainfo client.
func (client *Client) GetProjectInfo(ctx context.Context) (resp *pb.ProjectInfoResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	return client.client.ProjectInfo(ctx, &pb.ProjectInfoRequest{})
}
