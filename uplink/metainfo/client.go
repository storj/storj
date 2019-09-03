// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"bytes"
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

// CreateBucketParams parameters for CreateBucket method
type CreateBucketParams struct {
	Name                        []byte
	PathCipher                  storj.CipherSuite
	PartnerID                   []byte
	DefaultSegmentsSize         int64
	DefaultRedundancyScheme     storj.RedundancyScheme
	DefaultEncryptionParameters storj.EncryptionParameters
}

func (params *CreateBucketParams) toRequest() *pb.BucketCreateRequest {
	defaultRS := params.DefaultRedundancyScheme
	defaultEP := params.DefaultEncryptionParameters

	return &pb.BucketCreateRequest{
		Name:               params.Name,
		PathCipher:         pb.CipherSuite(params.PathCipher),
		PartnerId:          params.PartnerID,
		DefaultSegmentSize: params.DefaultSegmentsSize,
		DefaultRedundancyScheme: &pb.RedundancyScheme{
			Type:             pb.RedundancyScheme_SchemeType(defaultRS.Algorithm),
			MinReq:           int32(defaultRS.RequiredShares),
			Total:            int32(defaultRS.TotalShares),
			RepairThreshold:  int32(defaultRS.RepairShares),
			SuccessThreshold: int32(defaultRS.OptimalShares),
			ErasureShareSize: defaultRS.ShareSize,
		},
		DefaultEncryptionParameters: &pb.EncryptionParameters{
			CipherSuite: pb.CipherSuite(defaultEP.CipherSuite),
			BlockSize:   int64(defaultEP.BlockSize),
		},
	}
}

// BatchItem returns single item for batch request
func (params *CreateBucketParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_BucketCreate{
			BucketCreate: params.toRequest(),
		},
	}
}

// TODO potential names *Response/*Out/*Result

// CreateBucketResponse response for CreateBucket request
type CreateBucketResponse struct {
	Bucket storj.Bucket
}

func newCreateBucketResponse(response *pb.BucketCreateResponse) (CreateBucketResponse, error) {
	bucket, err := convertProtoToBucket(response.Bucket)
	if err != nil {
		return CreateBucketResponse{}, err
	}
	return CreateBucketResponse{
		Bucket: bucket,
	}, nil
}

// CreateBucket creates a new bucket
func (client *Client) CreateBucket(ctx context.Context, params CreateBucketParams) (respBucket storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.CreateBucket(ctx, params.toRequest())
	if err != nil {
		return storj.Bucket{}, Error.Wrap(err)
	}

	respBucket, err = convertProtoToBucket(response.Bucket)
	if err != nil {
		return storj.Bucket{}, Error.Wrap(err)
	}
	return respBucket, nil
}

// GetBucketParams parmaters for GetBucketParams method
type GetBucketParams struct {
	Name []byte
}

func (params *GetBucketParams) toRequest() *pb.BucketGetRequest {
	return &pb.BucketGetRequest{Name: params.Name}
}

// BatchItem returns single item for batch request
func (params *GetBucketParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_BucketGet{
			BucketGet: params.toRequest(),
		},
	}
}

// GetBucketResponse response for GetBucket request
type GetBucketResponse struct {
	Bucket storj.Bucket
}

func newGetBucketResponse(response *pb.BucketGetResponse) (GetBucketResponse, error) {
	bucket, err := convertProtoToBucket(response.Bucket)
	if err != nil {
		return GetBucketResponse{}, err
	}
	return GetBucketResponse{
		Bucket: bucket,
	}, nil
}

// GetBucket returns a bucket
func (client *Client) GetBucket(ctx context.Context, params GetBucketParams) (respBucket storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	resp, err := client.client.GetBucket(ctx, params.toRequest())
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return storj.Bucket{}, storj.ErrBucketNotFound.Wrap(err)
		}
		return storj.Bucket{}, Error.Wrap(err)
	}

	respBucket, err = convertProtoToBucket(resp.Bucket)
	if err != nil {
		return storj.Bucket{}, Error.Wrap(err)
	}
	return respBucket, nil
}

// DeleteBucketParams parmaters for DeleteBucket method
type DeleteBucketParams struct {
	Name []byte
}

func (params *DeleteBucketParams) toRequest() *pb.BucketDeleteRequest {
	return &pb.BucketDeleteRequest{Name: params.Name}
}

// BatchItem returns single item for batch request
func (params *DeleteBucketParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_BucketDelete{
			BucketDelete: params.toRequest(),
		},
	}
}

// DeleteBucket deletes a bucket
func (client *Client) DeleteBucket(ctx context.Context, params DeleteBucketParams) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = client.client.DeleteBucket(ctx, params.toRequest())
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return storj.ErrBucketNotFound.Wrap(err)
		}
		return Error.Wrap(err)
	}
	return nil
}

// ListBucketsParams parmaters for ListBucketsParams method
type ListBucketsParams struct {
	ListOpts storj.BucketListOptions
}

func (params *ListBucketsParams) toRequest() *pb.BucketListRequest {
	return &pb.BucketListRequest{
		Cursor:    []byte(params.ListOpts.Cursor),
		Limit:     int32(params.ListOpts.Limit),
		Direction: int32(params.ListOpts.Direction),
	}
}

// BatchItem returns single item for batch request
func (params *ListBucketsParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_BucketList{
			BucketList: params.toRequest(),
		},
	}
}

// ListBucketsResponse response for ListBucket request
type ListBucketsResponse struct {
	BucketList storj.BucketList
}

func newListBucketsResponse(response *pb.BucketListResponse) ListBucketsResponse {
	bucketList := storj.BucketList{
		More: response.More,
	}
	bucketList.Items = make([]storj.Bucket, len(response.Items))
	for i, item := range response.GetItems() {
		bucketList.Items[i] = storj.Bucket{
			Name:    string(item.Name),
			Created: item.CreatedAt,
		}
	}
	return ListBucketsResponse{
		BucketList: bucketList,
	}
}

// ListBuckets lists buckets
func (client *Client) ListBuckets(ctx context.Context, params ListBucketsParams) (_ storj.BucketList, err error) {
	defer mon.Task()(&ctx)(&err)

	resp, err := client.client.ListBuckets(ctx, params.toRequest())
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

// SetBucketAttributionParams parameters for SetBucketAttribution method
type SetBucketAttributionParams struct {
	Bucket    string
	PartnerID uuid.UUID
}

func (params *SetBucketAttributionParams) toRequest() *pb.BucketSetAttributionRequest {
	return &pb.BucketSetAttributionRequest{
		Name:      []byte(params.Bucket),
		PartnerId: params.PartnerID[:],
	}
}

// BatchItem returns single item for batch request
func (params *SetBucketAttributionParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_BucketSetAttribution{
			BucketSetAttribution: params.toRequest(),
		},
	}
}

// SetBucketAttribution tries to set the attribution information on the bucket.
func (client *Client) SetBucketAttribution(ctx context.Context, params SetBucketAttributionParams) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = client.client.SetBucketAttribution(ctx, params.toRequest())

	return Error.Wrap(err)
}

// BeginObjectParams parmaters for BeginObject method
type BeginObjectParams struct {
	Bucket               []byte
	EncryptedPath        []byte
	Version              int32
	Redundancy           storj.RedundancyScheme
	EncryptionParameters storj.EncryptionParameters
	ExpiresAt            time.Time
}

func (params *BeginObjectParams) toRequest() *pb.ObjectBeginRequest {
	return &pb.ObjectBeginRequest{
		Bucket:        params.Bucket,
		EncryptedPath: params.EncryptedPath,
		Version:       params.Version,
		ExpiresAt:     params.ExpiresAt,
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
	}
}

// BatchItem returns single item for batch request
func (params *BeginObjectParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_ObjectBegin{
			ObjectBegin: params.toRequest(),
		},
	}
}

// BeginObjectResponse response for BeginObject request
type BeginObjectResponse struct {
	StreamID storj.StreamID
}

func newBeginObjectResponse(response *pb.ObjectBeginResponse) BeginObjectResponse {
	return BeginObjectResponse{
		StreamID: response.StreamId,
	}
}

// BeginObject begins object creation
func (client *Client) BeginObject(ctx context.Context, params BeginObjectParams) (_ storj.StreamID, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.BeginObject(ctx, params.toRequest())
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return response.StreamId, nil
}

// CommitObjectParams parmaters for CommitObject method
type CommitObjectParams struct {
	StreamID storj.StreamID

	EncryptedMetadataNonce storj.Nonce
	EncryptedMetadata      []byte
}

func (params *CommitObjectParams) toRequest() *pb.ObjectCommitRequest {
	return &pb.ObjectCommitRequest{
		StreamId:               params.StreamID,
		EncryptedMetadataNonce: params.EncryptedMetadataNonce,
		EncryptedMetadata:      params.EncryptedMetadata,
	}
}

// BatchItem returns single item for batch request
func (params *CommitObjectParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_ObjectCommit{
			ObjectCommit: params.toRequest(),
		},
	}
}

// CommitObject commits created object
func (client *Client) CommitObject(ctx context.Context, params CommitObjectParams) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = client.client.CommitObject(ctx, params.toRequest())

	return Error.Wrap(err)
}

// GetObjectParams parameters for GetObject method
type GetObjectParams struct {
	Bucket        []byte
	EncryptedPath []byte
	Version       int32
}

func (params *GetObjectParams) toRequest() *pb.ObjectGetRequest {
	return &pb.ObjectGetRequest{
		Bucket:        params.Bucket,
		EncryptedPath: params.EncryptedPath,
		Version:       params.Version,
	}
}

// BatchItem returns single item for batch request
func (params *GetObjectParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_ObjectGet{
			ObjectGet: params.toRequest(),
		},
	}
}

// GetObjectResponse response for GetObject request
type GetObjectResponse struct {
	Info storj.ObjectInfo
}

func newGetObjectResponse(response *pb.ObjectGetResponse) GetObjectResponse {
	object := storj.ObjectInfo{
		Bucket: string(response.Object.Bucket),
		Path:   storj.Path(response.Object.EncryptedPath),

		StreamID: response.Object.StreamId,

		Created:  response.Object.CreatedAt,
		Modified: response.Object.CreatedAt,
		Expires:  response.Object.ExpiresAt,
		Metadata: response.Object.EncryptedMetadata,
		Stream: storj.Stream{
			Size: response.Object.TotalSize,
			EncryptionParameters: storj.EncryptionParameters{
				CipherSuite: storj.CipherSuite(response.Object.EncryptionParameters.CipherSuite),
				BlockSize:   int32(response.Object.EncryptionParameters.BlockSize),
			},
		},
	}

	pbRS := response.Object.RedundancyScheme
	if pbRS != nil {
		object.Stream.RedundancyScheme = storj.RedundancyScheme{
			Algorithm:      storj.RedundancyAlgorithm(pbRS.Type),
			ShareSize:      pbRS.ErasureShareSize,
			RequiredShares: int16(pbRS.MinReq),
			RepairShares:   int16(pbRS.RepairThreshold),
			OptimalShares:  int16(pbRS.SuccessThreshold),
			TotalShares:    int16(pbRS.Total),
		}
	}
	return GetObjectResponse{
		Info: object,
	}
}

// GetObject gets single object
func (client *Client) GetObject(ctx context.Context, params GetObjectParams) (_ storj.ObjectInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.GetObject(ctx, params.toRequest())

	if err != nil {
		if status.Code(err) == codes.NotFound {
			return storj.ObjectInfo{}, storj.ErrObjectNotFound.Wrap(err)
		}
		return storj.ObjectInfo{}, Error.Wrap(err)
	}

	getResponse := newGetObjectResponse(response)
	return getResponse.Info, nil
}

// BeginDeleteObjectParams parameters for BeginDeleteObject method
type BeginDeleteObjectParams struct {
	Bucket        []byte
	EncryptedPath []byte
	Version       int32
}

func (params *BeginDeleteObjectParams) toRequest() *pb.ObjectBeginDeleteRequest {
	return &pb.ObjectBeginDeleteRequest{
		Bucket:        params.Bucket,
		EncryptedPath: params.EncryptedPath,
		Version:       params.Version,
	}
}

// BatchItem returns single item for batch request
func (params *BeginDeleteObjectParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_ObjectBeginDelete{
			ObjectBeginDelete: params.toRequest(),
		},
	}
}

// BeginDeleteObjectResponse response for BeginDeleteObject request
type BeginDeleteObjectResponse struct {
	StreamID storj.StreamID
}

func newBeginDeleteObjectResponse(response *pb.ObjectBeginDeleteResponse) BeginDeleteObjectResponse {
	return BeginDeleteObjectResponse{
		StreamID: response.StreamId,
	}
}

// BeginDeleteObject begins object deletion process
func (client *Client) BeginDeleteObject(ctx context.Context, params BeginDeleteObjectParams) (_ storj.StreamID, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.BeginDeleteObject(ctx, params.toRequest())
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return storj.StreamID{}, storj.ErrObjectNotFound.Wrap(err)
		}
		return storj.StreamID{}, Error.Wrap(err)
	}

	return response.StreamId, nil
}

// FinishDeleteObjectParams parameters for FinishDeleteObject method
type FinishDeleteObjectParams struct {
	StreamID storj.StreamID
}

func (params *FinishDeleteObjectParams) toRequest() *pb.ObjectFinishDeleteRequest {
	return &pb.ObjectFinishDeleteRequest{
		StreamId: params.StreamID,
	}
}

// BatchItem returns single item for batch request
func (params *FinishDeleteObjectParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_ObjectFinishDelete{
			ObjectFinishDelete: params.toRequest(),
		},
	}
}

// FinishDeleteObject finishes object deletion process
func (client *Client) FinishDeleteObject(ctx context.Context, params FinishDeleteObjectParams) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = client.client.FinishDeleteObject(ctx, params.toRequest())

	return Error.Wrap(err)
}

// ListObjectsParams parameters for ListObjects method
type ListObjectsParams struct {
	Bucket          []byte
	EncryptedPrefix []byte
	EncryptedCursor []byte
	Limit           int32
	IncludeMetadata bool
	Recursive       bool
}

func (params *ListObjectsParams) toRequest() *pb.ObjectListRequest {
	return &pb.ObjectListRequest{
		Bucket:          params.Bucket,
		EncryptedPrefix: params.EncryptedPrefix,
		EncryptedCursor: params.EncryptedCursor,
		Limit:           params.Limit,
		ObjectIncludes: &pb.ObjectListItemIncludes{
			Metadata: params.IncludeMetadata,
		},
		Recursive: params.Recursive,
	}
}

// BatchItem returns single item for batch request
func (params *ListObjectsParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_ObjectList{
			ObjectList: params.toRequest(),
		},
	}
}

// ListObjectsResponse response for ListObjects request
type ListObjectsResponse struct {
	Items []storj.ObjectListItem
	More  bool
}

func newListObjectsResponse(response *pb.ObjectListResponse, encryptedPrefix []byte, recursive bool) ListObjectsResponse {
	objects := make([]storj.ObjectListItem, len(response.Items))
	for i, object := range response.Items {
		encryptedPath := object.EncryptedPath
		isPrefix := false
		if !recursive && len(encryptedPath) != 0 && encryptedPath[len(encryptedPath)-1] == '/' && !bytes.Equal(encryptedPath, encryptedPrefix) {
			isPrefix = true
		}

		objects[i] = storj.ObjectListItem{
			EncryptedPath:          object.EncryptedPath,
			Version:                object.Version,
			Status:                 int32(object.Status),
			StatusAt:               object.StatusAt,
			CreatedAt:              object.CreatedAt,
			ExpiresAt:              object.ExpiresAt,
			EncryptedMetadataNonce: object.EncryptedMetadataNonce,
			EncryptedMetadata:      object.EncryptedMetadata,

			IsPrefix: isPrefix,
		}
	}

	return ListObjectsResponse{
		Items: objects,
		More:  response.More,
	}
}

// ListObjects lists objects according to specific parameters
func (client *Client) ListObjects(ctx context.Context, params ListObjectsParams) (_ []storj.ObjectListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.ListObjects(ctx, params.toRequest())
	if err != nil {
		return []storj.ObjectListItem{}, false, Error.Wrap(err)
	}

	listResponse := newListObjectsResponse(response, params.EncryptedPrefix, params.Recursive)
	return listResponse.Items, listResponse.More, Error.Wrap(err)
}

// BeginSegmentParams parameters for BeginSegment method
type BeginSegmentParams struct {
	StreamID     storj.StreamID
	Position     storj.SegmentPosition
	MaxOderLimit int64
}

func (params *BeginSegmentParams) toRequest() *pb.SegmentBeginRequest {
	return &pb.SegmentBeginRequest{
		StreamId: params.StreamID,
		Position: &pb.SegmentPosition{
			PartNumber: params.Position.PartNumber,
			Index:      params.Position.Index,
		},
		MaxOrderLimit: params.MaxOderLimit,
	}
}

// BatchItem returns single item for batch request
func (params *BeginSegmentParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_SegmentBegin{
			SegmentBegin: params.toRequest(),
		},
	}
}

// BeginSegmentResponse response for BeginSegment request
type BeginSegmentResponse struct {
	SegmentID       storj.SegmentID
	Limits          []*pb.AddressedOrderLimit
	PiecePrivateKey storj.PiecePrivateKey
}

func newBeginSegmentResponse(response *pb.SegmentBeginResponse) BeginSegmentResponse {
	return BeginSegmentResponse{
		SegmentID:       response.SegmentId,
		Limits:          response.AddressedLimits,
		PiecePrivateKey: response.PrivateKey,
	}
}

// BeginSegment begins segment upload
func (client *Client) BeginSegment(ctx context.Context, params BeginSegmentParams) (_ storj.SegmentID, limits []*pb.AddressedOrderLimit, piecePrivateKey storj.PiecePrivateKey, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.BeginSegment(ctx, params.toRequest())
	if err != nil {
		return storj.SegmentID{}, nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	return response.SegmentId, response.AddressedLimits, response.PrivateKey, nil
}

// CommitSegmentParams parameters for CommitSegment method
type CommitSegmentParams struct {
	SegmentID         storj.SegmentID
	Encryption        storj.SegmentEncryption
	SizeEncryptedData int64

	UploadResult []*pb.SegmentPieceUploadResult
}

func (params *CommitSegmentParams) toRequest() *pb.SegmentCommitRequest {
	return &pb.SegmentCommitRequest{
		SegmentId: params.SegmentID,

		EncryptedKeyNonce: params.Encryption.EncryptedKeyNonce,
		EncryptedKey:      params.Encryption.EncryptedKey,
		SizeEncryptedData: params.SizeEncryptedData,
		UploadResult:      params.UploadResult,
	}
}

// BatchItem returns single item for batch request
func (params *CommitSegmentParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_SegmentCommit{
			SegmentCommit: params.toRequest(),
		},
	}
}

// CommitSegmentNew commits segment after upload
func (client *Client) CommitSegmentNew(ctx context.Context, params CommitSegmentParams) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = client.client.CommitSegment(ctx, params.toRequest())

	return Error.Wrap(err)
}

// MakeInlineSegmentParams parameters for MakeInlineSegment method
type MakeInlineSegmentParams struct {
	StreamID            storj.StreamID
	Position            storj.SegmentPosition
	Encryption          storj.SegmentEncryption
	EncryptedInlineData []byte
}

func (params *MakeInlineSegmentParams) toRequest() *pb.SegmentMakeInlineRequest {
	return &pb.SegmentMakeInlineRequest{
		StreamId: params.StreamID,
		Position: &pb.SegmentPosition{
			PartNumber: params.Position.PartNumber,
			Index:      params.Position.Index,
		},
		EncryptedKeyNonce:   params.Encryption.EncryptedKeyNonce,
		EncryptedKey:        params.Encryption.EncryptedKey,
		EncryptedInlineData: params.EncryptedInlineData,
	}
}

// BatchItem returns single item for batch request
func (params *MakeInlineSegmentParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_SegmentMakeInline{
			SegmentMakeInline: params.toRequest(),
		},
	}
}

// MakeInlineSegment commits segment after upload
func (client *Client) MakeInlineSegment(ctx context.Context, params MakeInlineSegmentParams) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = client.client.MakeInlineSegment(ctx, params.toRequest())

	return Error.Wrap(err)
}

// BeginDeleteSegmentParams parameters for BeginDeleteSegment method
type BeginDeleteSegmentParams struct {
	StreamID storj.StreamID
	Position storj.SegmentPosition
}

func (params *BeginDeleteSegmentParams) toRequest() *pb.SegmentBeginDeleteRequest {
	return &pb.SegmentBeginDeleteRequest{
		StreamId: params.StreamID,
		Position: &pb.SegmentPosition{
			PartNumber: params.Position.PartNumber,
			Index:      params.Position.Index,
		},
	}
}

// BatchItem returns single item for batch request
func (params *BeginDeleteSegmentParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_SegmentBeginDelete{
			SegmentBeginDelete: params.toRequest(),
		},
	}
}

// BeginDeleteSegmentResponse response for BeginDeleteSegment request
type BeginDeleteSegmentResponse struct {
	SegmentID       storj.SegmentID
	Limits          []*pb.AddressedOrderLimit
	PiecePrivateKey storj.PiecePrivateKey
}

func newBeginDeleteSegmentResponse(response *pb.SegmentBeginDeleteResponse) BeginDeleteSegmentResponse {
	return BeginDeleteSegmentResponse{
		SegmentID:       response.SegmentId,
		Limits:          response.AddressedLimits,
		PiecePrivateKey: response.PrivateKey,
	}
}

// BeginDeleteSegment begins segment upload process
func (client *Client) BeginDeleteSegment(ctx context.Context, params BeginDeleteSegmentParams) (_ storj.SegmentID, limits []*pb.AddressedOrderLimit, _ storj.PiecePrivateKey, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.BeginDeleteSegment(ctx, params.toRequest())
	if err != nil {
		return storj.SegmentID{}, nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	return response.SegmentId, response.AddressedLimits, response.PrivateKey, nil
}

// FinishDeleteSegmentParams parameters for FinishDeleteSegment method
type FinishDeleteSegmentParams struct {
	SegmentID storj.SegmentID

	DeleteResults []*pb.SegmentPieceDeleteResult
}

func (params *FinishDeleteSegmentParams) toRequest() *pb.SegmentFinishDeleteRequest {
	return &pb.SegmentFinishDeleteRequest{
		SegmentId: params.SegmentID,
		Results:   params.DeleteResults,
	}
}

// BatchItem returns single item for batch request
func (params *FinishDeleteSegmentParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_SegmentFinishDelete{
			SegmentFinishDelete: params.toRequest(),
		},
	}
}

// FinishDeleteSegment finishes segment upload process
func (client *Client) FinishDeleteSegment(ctx context.Context, params FinishDeleteSegmentParams) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = client.client.FinishDeleteSegment(ctx, params.toRequest())

	return Error.Wrap(err)
}

// DownloadSegmentParams parameters for DownloadSegment method
type DownloadSegmentParams struct {
	StreamID storj.StreamID
	Position storj.SegmentPosition
}

func (params *DownloadSegmentParams) toRequest() *pb.SegmentDownloadRequest {
	return &pb.SegmentDownloadRequest{
		StreamId: params.StreamID,
		CursorPosition: &pb.SegmentPosition{
			PartNumber: params.Position.PartNumber,
			Index:      params.Position.Index,
		},
	}
}

// BatchItem returns single item for batch request
func (params *DownloadSegmentParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_SegmentDownload{
			SegmentDownload: params.toRequest(),
		},
	}
}

// DownloadSegmentResponse response for DownloadSegment request
type DownloadSegmentResponse struct {
	Info storj.SegmentDownloadInfo

	Limits []*pb.AddressedOrderLimit
}

func newDownloadSegmentResponse(response *pb.SegmentDownloadResponse) DownloadSegmentResponse {
	info := storj.SegmentDownloadInfo{
		SegmentID:           response.SegmentId,
		Size:                response.SegmentSize,
		EncryptedInlineData: response.EncryptedInlineData,
		PiecePrivateKey:     response.PrivateKey,
		SegmentEncryption: storj.SegmentEncryption{
			EncryptedKeyNonce: response.EncryptedKeyNonce,
			EncryptedKey:      response.EncryptedKey,
		},
	}
	if response.Next != nil {
		info.Next = storj.SegmentPosition{
			PartNumber: response.Next.PartNumber,
			Index:      response.Next.Index,
		}
	}

	for i := range response.AddressedLimits {
		if response.AddressedLimits[i].Limit == nil {
			response.AddressedLimits[i] = nil
		}
	}
	return DownloadSegmentResponse{
		Info:   info,
		Limits: response.AddressedLimits,
	}
}

// DownloadSegment gets info for downloading remote segment or data from inline segment
func (client *Client) DownloadSegment(ctx context.Context, params DownloadSegmentParams) (_ storj.SegmentDownloadInfo, _ []*pb.AddressedOrderLimit, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.DownloadSegment(ctx, params.toRequest())
	if err != nil {
		return storj.SegmentDownloadInfo{}, nil, Error.Wrap(err)
	}

	downloadResponse := newDownloadSegmentResponse(response)
	return downloadResponse.Info, downloadResponse.Limits, nil
}

// ListSegmentsParams parameters for ListSegment method
type ListSegmentsParams struct {
	StreamID       storj.StreamID
	CursorPosition storj.SegmentPosition
	Limit          int32
}

// ListSegmentsResponse response for ListSegments request
type ListSegmentsResponse struct {
	Items []storj.SegmentListItem
	More  bool
}

func (params *ListSegmentsParams) toRequest() *pb.SegmentListRequest {
	return &pb.SegmentListRequest{
		StreamId: params.StreamID,
		CursorPosition: &pb.SegmentPosition{
			PartNumber: params.CursorPosition.PartNumber,
			Index:      params.CursorPosition.Index,
		},
		Limit: params.Limit,
	}
}

// BatchItem returns single item for batch request
func (params *ListSegmentsParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_SegmentList{
			SegmentList: params.toRequest(),
		},
	}
}

func newListSegmentsResponse(response *pb.SegmentListResponse) ListSegmentsResponse {
	items := make([]storj.SegmentListItem, len(response.Items))
	for i, responseItem := range response.Items {
		items[i] = storj.SegmentListItem{
			Position: storj.SegmentPosition{
				PartNumber: responseItem.Position.PartNumber,
				Index:      responseItem.Position.Index,
			},
		}
	}
	return ListSegmentsResponse{
		Items: items,
		More:  response.More,
	}
}

// ListSegmentsNew lists object segments
func (client *Client) ListSegmentsNew(ctx context.Context, params ListSegmentsParams) (_ []storj.SegmentListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.ListSegments(ctx, params.toRequest())
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return []storj.SegmentListItem{}, false, storj.ErrObjectNotFound.Wrap(err)
		}
		return []storj.SegmentListItem{}, false, Error.Wrap(err)
	}

	listResponse := newListSegmentsResponse(response)
	return listResponse.Items, listResponse.More, Error.Wrap(err)
}

// Batch sends multiple requests in one batch
func (client *Client) Batch(ctx context.Context, requests ...BatchItem) (resp []BatchResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	batchItems := make([]*pb.BatchRequestItem, len(requests))
	for i, request := range requests {
		batchItems[i] = request.BatchItem()
	}
	response, err := client.client.Batch(ctx, &pb.BatchRequest{
		Requests: batchItems,
	})
	if err != nil {
		return []BatchResponse{}, err
	}

	resp = make([]BatchResponse, len(response.Responses))
	for i, response := range response.Responses {
		resp[i] = BatchResponse{
			pbRequest:  batchItems[i].Request,
			pbResponse: response.Response,
		}
	}

	return resp, nil
}
