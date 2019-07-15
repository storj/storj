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
func (client *Client) CreateSegment(ctx context.Context, bucket string, path storj.Path, segmentIndex int64, redundancy *pb.RedundancyScheme, maxEncryptedSegmentSize int64, expiration time.Time) (limits []*pb.AddressedOrderLimit, rootPieceID storj.PieceID, piecePrivateKey storj.PiecePrivateKey, err error) {
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
		return nil, rootPieceID, piecePrivateKey, Error.Wrap(err)
	}

	return response.GetAddressedLimits(), response.RootPieceId, response.PrivateKey, nil
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
func (client *Client) ReadSegment(ctx context.Context, bucket string, path storj.Path, segmentIndex int64) (pointer *pb.Pointer, limits []*pb.AddressedOrderLimit, piecePrivateKey storj.PiecePrivateKey, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.DownloadSegmentOld(ctx, &pb.SegmentDownloadRequestOld{
		Bucket:  []byte(bucket),
		Path:    []byte(path),
		Segment: segmentIndex,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, nil, piecePrivateKey, storage.ErrKeyNotFound.Wrap(err)
		}
		return nil, nil, piecePrivateKey, Error.Wrap(err)
	}

	return response.GetPointer(), sortLimits(response.GetAddressedLimits(), response.GetPointer()), response.PrivateKey, nil
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
func (client *Client) DeleteSegment(ctx context.Context, bucket string, path storj.Path, segmentIndex int64) (limits []*pb.AddressedOrderLimit, piecePrivateKey storj.PiecePrivateKey, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.DeleteSegmentOld(ctx, &pb.SegmentDeleteRequestOld{
		Bucket:  []byte(bucket),
		Path:    []byte(path),
		Segment: segmentIndex,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, piecePrivateKey, storage.ErrKeyNotFound.Wrap(err)
		}
		return nil, piecePrivateKey, Error.Wrap(err)
	}

	return response.GetAddressedLimits(), response.PrivateKey, nil
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

	return Error.Wrap(err)
}

// GetProjectInfo gets the ProjectInfo for the api key associated with the metainfo client.
func (client *Client) GetProjectInfo(ctx context.Context) (resp *pb.ProjectInfoResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	return client.client.ProjectInfo(ctx, &pb.ProjectInfoRequest{})
}

// CreateBucket creates a new bucket
func (client *Client) CreateBucket(ctx context.Context, bucket storj.Bucket) (_ storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	req := convertBucketToProtoRequest(bucket)
	resp, err := client.client.CreateBucket(ctx, &req)
	if err != nil {
		return storj.Bucket{}, Error.Wrap(err)
	}

	return convertProtoToBucket(resp.Bucket), nil
}

// GetBucket returns a bucket
func (client *Client) GetBucket(ctx context.Context, bucketName string) (_ storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	resp, err := client.client.GetBucket(ctx, &pb.BucketGetRequest{Name: []byte(bucketName)})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return storj.Bucket{}, storj.ErrBucketNotFound.Wrap(err)
		}
		return storj.Bucket{}, Error.Wrap(err)
	}
	return convertProtoToBucket(resp.Bucket), nil
}

// DeleteBucket deletes a bucket
func (client *Client) DeleteBucket(ctx context.Context, bucketName string) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = client.client.DeleteBucket(ctx, &pb.BucketDeleteRequest{Name: []byte(bucketName)})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return storj.ErrBucketNotFound.Wrap(err)
		}
		return Error.Wrap(err)
	}
	return nil
}

// ListBuckets lists buckets
func (client *Client) ListBuckets(ctx context.Context, listOpts storj.BucketListOptions) (_ storj.BucketList, err error) {
	defer mon.Task()(&ctx)(&err)
	req := &pb.BucketListRequest{
		Cursor:    []byte(listOpts.Cursor),
		Limit:     int32(listOpts.Limit),
		Direction: int32(listOpts.Direction),
	}
	resp, err := client.client.ListBuckets(ctx, req)
	if err != nil {
		return storj.BucketList{}, Error.Wrap(err)
	}
	resultBucketList := storj.BucketList{
		More: resp.GetMore(),
	}
	resultBucketList.Items = make([]storj.Bucket, len(resp.GetItems()))
	for i, item := range resp.GetItems() {
		resultBucketList.Items[i] = storj.Bucket{
			Name:    string(item.GetName()),
			Created: item.GetCreatedAt(),
		}
	}
	return resultBucketList, nil
}

func convertBucketToProtoRequest(bucket storj.Bucket) pb.BucketCreateRequest {
	rs := bucket.DefaultRedundancyScheme
	return pb.BucketCreateRequest{
		Name:               []byte(bucket.Name),
		PathCipher:         pb.CipherSuite(bucket.PathCipher),
		DefaultSegmentSize: bucket.DefaultSegmentsSize,
		DefaultRedundancyScheme: &pb.RedundancyScheme{
			Type:             pb.RedundancyScheme_SchemeType(rs.Algorithm),
			MinReq:           int32(rs.RequiredShares),
			Total:            int32(rs.TotalShares),
			RepairThreshold:  int32(rs.RepairShares),
			SuccessThreshold: int32(rs.OptimalShares),
			ErasureShareSize: rs.ShareSize,
		},
		DefaultEncryptionParameters: &pb.EncryptionParameters{
			CipherSuite: pb.CipherSuite(bucket.DefaultEncryptionParameters.CipherSuite),
			BlockSize:   int64(bucket.DefaultEncryptionParameters.BlockSize),
		},
	}
}

func convertProtoToBucket(pbBucket *pb.Bucket) storj.Bucket {
	defaultRS := pbBucket.GetDefaultRedundancyScheme()
	defaultEP := pbBucket.GetDefaultEncryptionParameters()
	return storj.Bucket{
		Name:                string(pbBucket.GetName()),
		PathCipher:          storj.CipherSuite(pbBucket.GetPathCipher()),
		Created:             pbBucket.GetCreatedAt(),
		DefaultSegmentsSize: pbBucket.GetDefaultSegmentSize(),
		DefaultRedundancyScheme: storj.RedundancyScheme{
			Algorithm:      storj.RedundancyAlgorithm(defaultRS.GetType()),
			ShareSize:      defaultRS.GetErasureShareSize(),
			RequiredShares: int16(defaultRS.GetMinReq()),
			RepairShares:   int16(defaultRS.GetRepairThreshold()),
			OptimalShares:  int16(defaultRS.GetSuccessThreshold()),
			TotalShares:    int16(defaultRS.GetTotal()),
		},
		DefaultEncryptionParameters: storj.EncryptionParameters{
			CipherSuite: storj.CipherSuite(defaultEP.CipherSuite),
			BlockSize:   int32(defaultEP.BlockSize),
		},
	}
}
