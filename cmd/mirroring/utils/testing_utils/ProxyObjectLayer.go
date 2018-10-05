package testing_utils


import (
	"context"
	"github.com/minio/minio/pkg/hash"
	"io"

	minio "github.com/minio/minio/cmd"
)

// Creates new instance of proxyObjectLayer,
// initializes all callbacks with default values
// and returns pointer to it.
func NewProxyObjectLayer() *proxyObjectLayer {

	n := proxyObjectLayer{}

	n.ShutdownFunc = func (context.Context) error {
		return nil
	}
	n.StorageInfoFunc = func (context.Context) (info minio.StorageInfo) {
		return
	}

	n.MakeBucketWithLocationFunc = func (ctx context.Context, bucket string, location string) (err error) {
		return
	}
	n.GetBucketInfoFunc = func (ctx context.Context, bucket string) (bucketInfo minio.BucketInfo, err error) {
		return
	}
	n.ListBucketsFunc = func (ctx context.Context) (buckets []minio.BucketInfo, err error) {
		return
	}
	n.DeleteBucketFunc = func (ctx context.Context, bucket string) (err error) {
		return
	}
	n.ListObjectsFunc = func (ctx context.Context, bucket, prefix, marker, delimiter string, maxKeys int) (result minio.ListObjectsInfo, err error) {
		return
	}
	n.ListObjectsV2Func = func (ctx context.Context, bucket, prefix, continuationToken, delimiter string, maxKeys int, fetchOwner bool, startAfter string) (result minio.ListObjectsV2Info, err error) {
		return
	}


	n.GetObjectFunc = func (ctx context.Context, bucket, object string, startOffset int64, length int64, writer io.Writer, etag string, opts minio.ObjectOptions) (err error) {
		return
	}
	n.GetObjectInfoFunc = func (ctx context.Context, bucket, object string, opts minio.ObjectOptions) (objInfo minio.ObjectInfo, err error) {
		return
	}
	n.PutObjectFunc = func (ctx context.Context, bucket, object string, data *hash.Reader, metadata map[string]string, opts minio.ObjectOptions) (objInfo minio.ObjectInfo, err error) {
		return
	}
	n.CopyObjectFunc = func (ctx context.Context, srcBucket, srcObject, destBucket, destObject string, srcInfo minio.ObjectInfo, srcOpts, dstOpts minio.ObjectOptions) (objInfo minio.ObjectInfo, err error) {
		return
	}
	n.DeleteObjectFunc = func (ctx context.Context, bucket, object string) (err error) {
		return
	}

	return &n
}

// Type proxyObjectLayer implements ObjectLayer interface
// Also this struct has stub-implementation for each function from ObjectLayer interface.
// Each ObjectLayer interface function will execute and return needed stub function
// So, to create needed behavior for some function from ObjectLayer interface, f.e. ListBuckets -
// we only need to reimplement ListBucketsFunc field of proxyObjectLayer struct
type proxyObjectLayer struct {
	minio.GatewayUnsupported

	ShutdownFunc func (context.Context) error
	StorageInfoFunc func (context.Context) minio.StorageInfo

	// Bucket operations.
	MakeBucketWithLocationFunc func (ctx context.Context, bucket string, location string) error
	GetBucketInfoFunc func (ctx context.Context, bucket string) (bucketInfo minio.BucketInfo, err error)
	ListBucketsFunc func (ctx context.Context) (buckets []minio.BucketInfo, err error)
	DeleteBucketFunc func (ctx context.Context, bucket string) error
	ListObjectsFunc func (ctx context.Context, bucket, prefix, marker, delimiter string, maxKeys int) (result minio.ListObjectsInfo, err error)
	ListObjectsV2Func func (ctx context.Context, bucket, prefix, continuationToken, delimiter string, maxKeys int, fetchOwner bool, startAfter string) (result minio.ListObjectsV2Info, err error)

	// Object operations.

	GetObjectFunc func (ctx context.Context, bucket, object string, startOffset int64, length int64, writer io.Writer, etag string, opts minio.ObjectOptions) (err error)
	GetObjectInfoFunc func (ctx context.Context, bucket, object string, opts minio.ObjectOptions) (objInfo minio.ObjectInfo, err error)
	PutObjectFunc func (ctx context.Context, bucket, object string, data *hash.Reader, metadata map[string]string, opts minio.ObjectOptions) (objInfo minio.ObjectInfo, err error)
	CopyObjectFunc func (ctx context.Context, srcBucket, srcObject, destBucket, destObject string, srcInfo minio.ObjectInfo, srcOpts, dstOpts minio.ObjectOptions) (objInfo minio.ObjectInfo, err error)
	DeleteObjectFunc func (ctx context.Context, bucket, object string) error
}


func (n *proxyObjectLayer) Shutdown(ctx context.Context) error {
	return n.ShutdownFunc(ctx)
}

func (n *proxyObjectLayer) StorageInfo(ctx context.Context) (info minio.StorageInfo) {
	return n.StorageInfoFunc(ctx)
}

func (n *proxyObjectLayer) MakeBucketWithLocation(ctx context.Context, bucket string, location string) error {
	return n.MakeBucketWithLocationFunc(ctx, bucket, location)
}

func (n *proxyObjectLayer) GetBucketInfo(ctx context.Context, bucket string) (bucketInfo minio.BucketInfo, err error) {
	return n.GetBucketInfoFunc(ctx, bucket)
}

func (n *proxyObjectLayer) ListBuckets(ctx context.Context) (buckets []minio.BucketInfo, err error) {
	return n.ListBucketsFunc(ctx)
}

func (n *proxyObjectLayer) DeleteBucket(ctx context.Context, bucket string) error {
	return n.DeleteBucketFunc(ctx, bucket)
}

func (n *proxyObjectLayer) ListObjects(ctx context.Context, bucket, prefix, marker, delimiter string, maxKeys int) (result minio.ListObjectsInfo, err error) {
	return n.ListObjectsFunc(ctx, bucket, prefix, marker, delimiter, maxKeys)
}

func (n *proxyObjectLayer) ListObjectsV2(ctx context.Context, bucket, prefix, continuationToken, delimiter string, maxKeys int, fetchOwner bool, startAfter string) (result minio.ListObjectsV2Info, err error) {
	return n.ListObjectsV2Func(ctx, bucket, prefix, continuationToken, delimiter, maxKeys, fetchOwner, startAfter)
}

func (n *proxyObjectLayer) GetObject(ctx context.Context, bucket, object string, startOffset int64, length int64, writer io.Writer, etag string, opts minio.ObjectOptions) (err error) {
	return n.GetObjectFunc(ctx, bucket, object, startOffset, length, writer, etag, opts)
}

func (n *proxyObjectLayer) GetObjectInfo(ctx context.Context, bucket, object string, opts minio.ObjectOptions) (objInfo minio.ObjectInfo, err error) {
	return n.GetObjectInfoFunc(ctx, bucket, object, opts)
}

func (n *proxyObjectLayer) PutObject(ctx context.Context, bucket, object string, data *hash.Reader, metadata map[string]string, opts minio.ObjectOptions) (objInfo minio.ObjectInfo, err error) {
	return n.PutObjectFunc(ctx, bucket, object, data, metadata, opts)
}

func (n *proxyObjectLayer) CopyObject(ctx context.Context, srcBucket, srcObject, destBucket, destObject string, srcInfo minio.ObjectInfo, srcOpts, dstOpts minio.ObjectOptions) (objInfo minio.ObjectInfo, err error) {
	return n.CopyObjectFunc(ctx, srcBucket, srcObject, destBucket, destObject, srcInfo, srcOpts, dstOpts)
}

func (n *proxyObjectLayer) DeleteObject(ctx context.Context, bucket, object string) error {
	return n.DeleteObjectFunc(ctx, bucket, object)
}
