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
func Dial(ctx context.Context, tc transport.Client, address string, apikey string) (*Client, error) {
	conn, err := tc.DialAddress(
		ctx,
		address,
		grpc.WithPerRPCCredentials(grpcauth.NewAPIKeyCredentials(apikey)),
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
func (client *Client) CreateBucket(ctx context.Context, bucket storj.Bucket) (respBucket storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	req, err := convertBucketToProtoRequest(bucket)
	if err != nil {
		return respBucket, Error.Wrap(err)
	}
	resp, err := client.client.CreateBucket(ctx, &req)
	if err != nil {
		return storj.Bucket{}, Error.Wrap(err)
	}

	respBucket, err = convertProtoToBucket(resp.Bucket)
	if err != nil {
		return respBucket, Error.Wrap(err)
	}
	return respBucket, nil
}

// GetBucket returns a bucket
func (client *Client) GetBucket(ctx context.Context, bucketName string) (respBucket storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	resp, err := client.client.GetBucket(ctx, &pb.BucketGetRequest{Name: []byte(bucketName)})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return storj.Bucket{}, storj.ErrBucketNotFound.Wrap(err)
		}
		return storj.Bucket{}, Error.Wrap(err)
	}

	respBucket, err = convertProtoToBucket(resp.Bucket)
	if err != nil {
		return respBucket, Error.Wrap(err)
	}
	return respBucket, nil
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

func convertBucketToProtoRequest(bucket storj.Bucket) (bucketReq pb.BucketCreateRequest, err error) {
	rs := bucket.DefaultRedundancyScheme
	partnerID, err := bucket.PartnerID.MarshalJSON()
	if err != nil {
		return bucketReq, Error.Wrap(err)
	}
	return pb.BucketCreateRequest{
		Name:               []byte(bucket.Name),
		PathCipher:         pb.CipherSuite(bucket.PathCipher),
		PartnerId:          partnerID,
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
	}, nil
}

func convertProtoToBucket(pbBucket *pb.Bucket) (bucket storj.Bucket, err error) {
	defaultRS := pbBucket.GetDefaultRedundancyScheme()
	defaultEP := pbBucket.GetDefaultEncryptionParameters()
	var partnerID uuid.UUID
	err = partnerID.UnmarshalJSON(pbBucket.GetPartnerId())
	if err != nil && !partnerID.IsZero() {
		return bucket, errs.New("Invalid uuid")
	}
	return storj.Bucket{
		Name:                string(pbBucket.GetName()),
		PartnerID:           partnerID,
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
	}, nil
}

// BeginObjectParams parmaters for BeginObject method
type BeginObjectParams struct {
	Bucket                 []byte
	EncryptedPath          []byte
	Version                int32
	Redundancy             storj.RedundancyScheme
	EncryptionParameters   storj.EncryptionParameters
	ExpiresAt              time.Time
	EncryptedMetadataNonce storj.Nonce
	EncryptedMetadata      []byte
}

// BeginObject begins object creation
func (client *Client) BeginObject(ctx context.Context, params BeginObjectParams) (_ storj.StreamID, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.BeginObject(ctx, &pb.ObjectBeginRequest{
		Bucket:                 params.Bucket,
		EncryptedPath:          params.EncryptedPath,
		Version:                params.Version,
		ExpiresAt:              params.ExpiresAt,
		EncryptedMetadataNonce: params.EncryptedMetadataNonce,
		EncryptedMetadata:      params.EncryptedMetadata,
		RedundancyScheme: &pb.RedundancyScheme{
			Type:             pb.RedundancyScheme_SchemeType(params.Redundancy.Algorithm),
			ErasureShareSize: params.Redundancy.ShareSize,
			MinReq:           int32(params.Redundancy.RequiredShares),
			RepairThreshold:  int32(params.Redundancy.RepairShares),
			SuccessThreshold: int32(params.Redundancy.OptimalShares),
			Total:            int32(params.Redundancy.TotalShares),
		},
		EncryptionParameters: &pb.EncryptionParameters{
			CipherSuite: pb.CipherSuite(params.EncryptionParameters.CipherSuite),
			BlockSize:   int64(params.EncryptionParameters.BlockSize),
		},
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return response.StreamId, nil
}

// CommitObject commits created object
func (client *Client) CommitObject(ctx context.Context, streamID storj.StreamID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = client.client.CommitObject(ctx, &pb.ObjectCommitRequest{
		StreamId: streamID,
	})
	return Error.Wrap(err)
}

// GetObjectParams parameters for GetObject method
type GetObjectParams struct {
	Bucket        []byte
	EncryptedPath []byte
	Version       int32
}

// GetObject gets single object
func (client *Client) GetObject(ctx context.Context, params GetObjectParams) (_ storj.Object, _ storj.StreamID, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.GetObject(ctx, &pb.ObjectGetRequest{
		Bucket:        params.Bucket,
		EncryptedPath: params.EncryptedPath,
		Version:       params.Version,
	})
	if err != nil {
		return storj.Object{}, storj.StreamID{}, Error.Wrap(err)
	}

	object := storj.Object{
		Bucket: storj.Bucket{
			Name: string(response.Object.Bucket),
		},
		Path:    storj.Path(response.Object.EncryptedPath),
		Created: response.Object.CreatedAt,
		Expires: response.Object.ExpiresAt,
		// TODO custom type for response object or modify storj.Object
	}

	return object, response.Object.StreamId, nil
}

// BeginDeleteObjectParams parameters for BeginDeleteObject method
type BeginDeleteObjectParams struct {
	Bucket        []byte
	EncryptedPath []byte
	Version       int32
}

// BeginDeleteObject begins object deletion process
func (client *Client) BeginDeleteObject(ctx context.Context, params BeginDeleteObjectParams) (_ storj.StreamID, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.BeginDeleteObject(ctx, &pb.ObjectBeginDeleteRequest{
		Bucket:        params.Bucket,
		EncryptedPath: params.EncryptedPath,
		Version:       params.Version,
	})
	if err != nil {
		return storj.StreamID{}, Error.Wrap(err)
	}

	return response.StreamId, nil
}

// FinishDeleteObject finishes object deletion process
func (client *Client) FinishDeleteObject(ctx context.Context, streamID storj.StreamID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = client.client.FinishDeleteObject(ctx, &pb.ObjectFinishDeleteRequest{
		StreamId: streamID,
	})
	return Error.Wrap(err)
}

// ListObjectsParams parameters for ListObjects method
type ListObjectsParams struct {
	Bucket          []byte
	EncryptedPrefix []byte
	EncryptedCursor []byte
	Limit           int32
	IncludeMetadata bool
}

// ListObjects lists objects according to specific parameters
func (client *Client) ListObjects(ctx context.Context, params ListObjectsParams) (_ []storj.ObjectListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.ListObjects(ctx, &pb.ObjectListRequest{
		Bucket:          params.Bucket,
		EncryptedPrefix: params.EncryptedPrefix,
		EncryptedCursor: params.EncryptedCursor,
		Limit:           params.Limit,
		ObjectIncludes: &pb.ObjectListItemIncludes{
			Metadata: params.IncludeMetadata,
		},
	})
	if err != nil {
		return []storj.ObjectListItem{}, false, Error.Wrap(err)
	}

	objects := make([]storj.ObjectListItem, len(response.Items))
	for i, object := range response.Items {
		objects[i] = storj.ObjectListItem{
			EncryptedPath:          object.EncryptedPath,
			Version:                object.Version,
			Status:                 int32(object.Status),
			StatusAt:               object.StatusAt,
			CreatedAt:              object.CreatedAt,
			ExpiresAt:              object.ExpiresAt,
			EncryptedMetadataNonce: object.EncryptedMetadataNonce,
			EncryptedMetadata:      object.EncryptedMetadata,
		}
	}

	return objects, response.More, Error.Wrap(err)
}

// BeginSegmentParams parameters for BeginSegment method
type BeginSegmentParams struct {
	StreamID     storj.StreamID
	Position     storj.SegmentPosition
	MaxOderLimit int64
}

// BeginSegment begins segment upload
func (client *Client) BeginSegment(ctx context.Context, params BeginSegmentParams) (_ storj.SegmentID, limits []*pb.AddressedOrderLimit, piecePrivateKey storj.PiecePrivateKey, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.BeginSegment(ctx, &pb.SegmentBeginRequest{
		StreamId: params.StreamID,
		Position: &pb.SegmentPosition{
			PartNumber: params.Position.PartNumber,
			Index:      params.Position.Index,
		},
		MaxOrderLimit: params.MaxOderLimit,
	})
	if err != nil {
		return storj.SegmentID{}, nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	return response.SegmentId, response.AddressedLimits, response.PrivateKey, nil
}

// CommitSegmentParams parameters for CommitSegment method
type CommitSegmentParams struct {
	SegmentID         storj.SegmentID
	EncryptedKeyNonce storj.Nonce
	EncryptedKey      []byte
	SizeEncryptedData int64
	// TODO find better way for this
	UploadResult []*pb.SegmentPieceUploadResult
}

// CommitSegment2 commits segment after upload
func (client *Client) CommitSegment2(ctx context.Context, params CommitSegmentParams) (err error) {
	// TODO method name will be changed when new methods will be fully integrated with client side
	defer mon.Task()(&ctx)(&err)

	_, err = client.client.CommitSegment(ctx, &pb.SegmentCommitRequest{
		SegmentId:         params.SegmentID,
		EncryptedKeyNonce: params.EncryptedKeyNonce,
		EncryptedKey:      params.EncryptedKey,
		SizeEncryptedData: params.SizeEncryptedData,
		UploadResult:      params.UploadResult,
	})
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// MakeInlineSegmentParams parameters for MakeInlineSegment method
type MakeInlineSegmentParams struct {
	StreamID            storj.StreamID
	Position            storj.SegmentPosition
	EncryptedKeyNonce   storj.Nonce
	EncryptedKey        []byte
	EncryptedInlineData []byte
}

// MakeInlineSegment commits segment after upload
func (client *Client) MakeInlineSegment(ctx context.Context, params MakeInlineSegmentParams) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = client.client.MakeInlineSegment(ctx, &pb.SegmentMakeInlineRequest{
		StreamId: params.StreamID,
		Position: &pb.SegmentPosition{
			PartNumber: params.Position.PartNumber,
			Index:      params.Position.Index,
		},
		EncryptedKeyNonce:   params.EncryptedKeyNonce,
		EncryptedKey:        params.EncryptedKey,
		EncryptedInlineData: params.EncryptedInlineData,
	})
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// BeginDeleteSegmentParams parameters for BeginDeleteSegment method
type BeginDeleteSegmentParams struct {
	StreamID storj.StreamID
	Position storj.SegmentPosition
}

// BeginDeleteSegment begins segment upload process
func (client *Client) BeginDeleteSegment(ctx context.Context, params BeginDeleteSegmentParams) (_ storj.SegmentID, limits []*pb.AddressedOrderLimit, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.BeginDeleteSegment(ctx, &pb.SegmentBeginDeleteRequest{
		StreamId: params.StreamID,
		Position: &pb.SegmentPosition{
			PartNumber: params.Position.PartNumber,
			Index:      params.Position.Index,
		},
	})
	if err != nil {
		return storj.SegmentID{}, nil, Error.Wrap(err)
	}

	return response.SegmentId, response.AddressedLimits, nil
}

// FinishDeleteSegmentParams parameters for FinishDeleteSegment method
type FinishDeleteSegmentParams struct {
	SegmentID storj.SegmentID
	// TODO find better way to pass this
	DeleteResults []*pb.SegmentPieceDeleteResult
}

// FinishDeleteSegment finishes segment upload process
func (client *Client) FinishDeleteSegment(ctx context.Context, params FinishDeleteSegmentParams) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = client.client.FinishDeleteSegment(ctx, &pb.SegmentFinishDeleteRequest{
		SegmentId: params.SegmentID,
		Results:   params.DeleteResults,
	})
	return Error.Wrap(err)
}

// DownloadSegmentParams parameters for DownloadSegment method
type DownloadSegmentParams struct {
	StreamID storj.StreamID
	Position storj.SegmentPosition
}

// DownloadSegment gets info for downloading remote segment or data from inline segment
func (client *Client) DownloadSegment(ctx context.Context, params DownloadSegmentParams) (_ storj.SegmentDownloadInfo, _ []*pb.AddressedOrderLimit, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.DownloadSegment(ctx, &pb.SegmentDownloadRequest{
		StreamId: params.StreamID,
		CursorPosition: &pb.SegmentPosition{
			PartNumber: params.Position.PartNumber,
			Index:      params.Position.Index,
		},
	})
	if err != nil {
		return storj.SegmentDownloadInfo{}, nil, Error.Wrap(err)
	}

	info := storj.SegmentDownloadInfo{
		SegmentID:           response.SegmentId,
		EncryptedInlineData: response.EncryptedInlineData,
	}
	if response.Next != nil {
		info.Next = storj.SegmentPosition{
			PartNumber: response.Next.PartNumber,
			Index:      response.Next.Index,
		}
	}

	return info, response.AddressedLimits, nil
}

// ListSegmentsParams parameters for ListSegment method
type ListSegmentsParams struct {
	StreamID       storj.StreamID
	CursorPosition storj.SegmentPosition
	Limit          int32
}

// ListSegments2 lists object segments
func (client *Client) ListSegments2(ctx context.Context, params ListSegmentsParams) (_ []storj.SegmentListItem, more bool, err error) {
	// TODO method name will be changed when new methods will be fully integrated with client side
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.ListSegments(ctx, &pb.SegmentListRequest{
		StreamId: params.StreamID,
		CursorPosition: &pb.SegmentPosition{
			PartNumber: params.CursorPosition.PartNumber,
			Index:      params.CursorPosition.Index,
		},
		Limit: params.Limit,
	})
	if err != nil {
		return []storj.SegmentListItem{}, false, Error.Wrap(err)
	}

	items := make([]storj.SegmentListItem, len(response.Items))
	for i, responseItem := range response.Items {
		items[i] = storj.SegmentListItem{
			Position: storj.SegmentPosition{
				PartNumber: responseItem.Position.PartNumber,
				Index:      responseItem.Position.Index,
			},
		}
	}
	return items, response.More, Error.Wrap(err)
}
