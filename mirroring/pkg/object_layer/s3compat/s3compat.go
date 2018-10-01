package s3compat

import (
	"context"
	"fmt"
	miniogo "github.com/minio/minio-go"
	minio "github.com/minio/minio/cmd"
	"github.com/minio/minio/pkg/hash"
	"io"
	"math/rand"
	"time"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyz01234569"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

// randString generates random names and prepends them with a known prefix.
func randString(n int, src rand.Source, prefix string) string {
	b := make([]byte, n)
	// A rand.Int63() generates 63 random bits, enough for letterIdxMax letters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}

		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}

		cache >>= letterIdxBits
		remain--
	}

	return prefix + string(b[0:30-len(prefix)])
}

type s3Compat struct {
	minio.GatewayUnsupported
	Client *miniogo.Core
}

func NewS3Compat(url, accessKey, secretKey string) (*s3Compat, error) {
	if url == "" {
		return nil, fmt.Errorf("No url provided for initializing s3compat instance")
	}

	endpoint, secure, err := minio.ParseGatewayEndpoint(url)
	if err != nil {
		return nil, err
	}

	clnt, err := miniogo.NewV4(endpoint, accessKey, secretKey, secure)
	if err != nil {
		return nil, err
	}

	probeBucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "probe-bucket-sign-")

	if _, err = clnt.BucketExists(probeBucketName); err != nil {
		clnt, err = miniogo.NewV2(endpoint, accessKey, secretKey, secure)

		if err != nil {
			return nil, err
		}

		if _, err = clnt.BucketExists(probeBucketName); err != nil {
			return nil, err
		}
	}

	core := &miniogo.Core{Client: clnt}
	return &s3Compat{Client: core}, nil
}


//Storage operations
func (s *s3Compat) Shutdown(ctx context.Context) error {
	return nil
}

func (s *s3Compat) StorageInfo(ctx context.Context) (storageInfo minio.StorageInfo) {
	return storageInfo
}

func (s *s3Compat) MakeBucketWithLocation(ctx context.Context, bucket string, location string) error {
	err := s.Client.MakeBucket(bucket, location)
	if err != nil {
		return minio.ErrorRespToObjectError(err, bucket)
	}

	return err
}

func (s *s3Compat) GetBucketInfo(ctx context.Context, bucket string) (bucketInfo minio.BucketInfo, err error) {
	buckets, err := s.Client.ListBuckets()
	if err != nil {
		return bucketInfo, minio.ErrorRespToObjectError(err, bucket)
	}

	for _, bi := range buckets {
		if bi.Name != bucket {
			continue
		}

		return minio.BucketInfo{
			Name:    bi.Name,
			Created: bi.CreationDate,
		}, nil
	}

	err = minio.BucketNotFound{Bucket: bucket}
	return
}

func (s *s3Compat) ListBuckets(ctx context.Context) ([]minio.BucketInfo, error) {
	buckets, err := s.Client.ListBuckets()
	if err != nil {
		return nil, minio.ErrorRespToObjectError(err)
	}

	b := make([]minio.BucketInfo, len(buckets))
	for i, bi := range buckets {
		b[i] = minio.BucketInfo{
			Name:    bi.Name,
			Created: bi.CreationDate,
		}
	}

	return b, err
}

func (s *s3Compat) DeleteBucket(ctx context.Context, bucket string) error {
	err := s.Client.RemoveBucket(bucket)
	if err != nil {
		return minio.ErrorRespToObjectError(err, bucket)
	}

	return nil
}

func (s *s3Compat) ListObjects(ctx context.Context, bucket, prefix, marker, delimiter string, maxKeys int) (loi minio.ListObjectsInfo, err error) {
	result, err := s.Client.ListObjects(bucket, prefix, marker, delimiter, maxKeys)
	if err != nil {
		return loi, minio.ErrorRespToObjectError(err, bucket)
	}

	return minio.FromMinioClientListBucketResult(bucket, result), nil
}

func (s *s3Compat) ListObjectsV2(ctx context.Context, bucket, prefix, continuationToken, delimiter string, maxKeys int, fetchOwner bool, startAfter string) (loi minio.ListObjectsV2Info, err error) {
	result, err := s.Client.ListObjectsV2(bucket, prefix, continuationToken, fetchOwner, delimiter, maxKeys, startAfter)
	if err != nil {
		return loi, minio.ErrorRespToObjectError(err, bucket)
	}

	return minio.FromMinioClientListBucketV2Result(bucket, result), nil
}

//Object operations
func (s *s3Compat) GetObject(ctx context.Context, bucket, object string, startOffset int64, length int64, writer io.Writer, etag string, opts minio.ObjectOptions) (err error) {
	if length < 0 && length != -1 {
		return minio.ErrorRespToObjectError(minio.InvalidRange{}, bucket, object)
	}

	var getObjectOptions = miniogo.GetObjectOptions{}
	if startOffset >= 0 && length >= 0 {
		if err := getObjectOptions.SetRange(startOffset, startOffset+length-1); err != nil {
			return minio.ErrorRespToObjectError(err, bucket, object)
		}
	}

	reader, objInfo, err := s.Client.GetObject(bucket, object, getObjectOptions)
	if err != nil {
		//fmt.Printf("obj %s url %s Error: %s\n",object, err)
		return minio.ErrorRespToObjectError(err, bucket, object)
	}

	defer reader.Close()
	//fmt.Printf("ObjectInfo: %s\n", objInfo)

	var b = make([]byte, objInfo.Size)

	_, err = io.ReadFull(reader, b)
	if err != nil {
		//fmt.Printf("Error: %s\n", err)
		return
	}

	_, err = writer.Write(b)
	return
}

func (s *s3Compat) GetObjectInfo(ctx context.Context, bucket, object string, opts minio.ObjectOptions) (objInfo minio.ObjectInfo, err error) {
	oi, err := s.Client.StatObject(bucket, object, miniogo.StatObjectOptions{})
	if err != nil {
		err = minio.ErrorRespToObjectError(err, bucket, object)
		return
	}

	objInfo = minio.FromMinioClientObjectInfo(bucket, oi)
	return
}

func (s *s3Compat) PutObject(ctx context.Context, bucket, object string, data *hash.Reader, metadata map[string]string, opts minio.ObjectOptions) (objInfo minio.ObjectInfo, err error) {
	oi, err := s.Client.PutObject(bucket, object, data, data.Size(), data.MD5Base64String(), data.SHA256HexString(), minio.ToMinioClientMetadata(metadata), opts.ServerSideEncryption)
	if err != nil {
		return objInfo, minio.ErrorRespToObjectError(err, bucket, object)
	}

	return minio.FromMinioClientObjectInfo(bucket, oi), nil
}

func (s *s3Compat) CopyObject(ctx context.Context, srcBucket, srcObject, dstBucket, dstObject string, srcInfo minio.ObjectInfo, srcOpts minio.ObjectOptions, dstOpts minio.ObjectOptions) (objInfo minio.ObjectInfo, err error) {
	// Set this header such that following CopyObject() always sets the right metadata on the destination.
	// metadata input is already a trickled down value from interpreting x-amz-metadata-directive at
	// handler layer. So what we have right now is supposed to be applied on the destination object anyways.
	// So preserve it by adding "REPLACE" directive to save all the metadata set by CopyObject API.
	srcInfo.UserDefined["x-amz-metadata-directive"] = "REPLACE"
	srcInfo.UserDefined["x-amz-copy-source-if-match"] = srcInfo.ETag

	_, err = s.Client.CopyObject(srcBucket, srcObject, dstBucket, dstObject, srcInfo.UserDefined)
	if err != nil {
		return objInfo, minio.ErrorRespToObjectError(err, srcBucket, srcObject)
	}

	return s.GetObjectInfo(ctx, dstBucket, dstObject, dstOpts)
}

func (s *s3Compat) DeleteObject(ctx context.Context, bucket, object string) error {
	err := s.Client.RemoveObject(bucket, object)
	if err != nil {
		return minio.ErrorRespToObjectError(err, bucket, object)
	}

	return nil
}

//// Multipart operations.
//ListMultipartUploads(ctx context.Context, bucket, prefix, keyMarker, uploadIDMarker, delimiter string, maxUploads int) (result ListMultipartsInfo, err error)
//NewMultipartUpload(ctx context.Context, bucket, object string, metadata map[string]string) (uploadID string, err error)
//CopyObjectPart(ctx context.Context, srcBucket, srcObject, destBucket, destObject string, uploadID string, partID int,
//startOffset int64, length int64, srcInfo ObjectInfo) (info PartInfo, err error)
//PutObjectPart(ctx context.Context, bucket, object, uploadID string, partID int, data *hash.Reader) (info PartInfo, err error)
//ListObjectParts(ctx context.Context, bucket, object, uploadID string, partNumberMarker int, maxParts int) (result ListPartsInfo, err error)
//AbortMultipartUpload(ctx context.Context, bucket, object, uploadID string) error
//CompleteMultipartUpload(ctx context.Context, bucket, object, uploadID string, uploadedParts []CompletePart) (objInfo ObjectInfo, err error)
//
//// Healing operations.
//ReloadFormat(ctx context.Context, dryRun bool) error
//HealFormat(ctx context.Context, dryRun bool) (madmin.HealResultItem, error)
//HealBucket(ctx context.Context, bucket string, dryRun bool) ([]madmin.HealResultItem, error)
//HealObject(ctx context.Context, bucket, object string, dryRun bool) (madmin.HealResultItem, error)
//ListBucketsHeal(ctx context.Context) (buckets []BucketInfo, err error)
//ListObjectsHeal(ctx context.Context, bucket, prefix, marker, delimiter string, maxKeys int) (ListObjectsInfo, error)
//
//// Policy operations
//SetBucketPolicy(context.Context, string, *policy.Policy) error
//GetBucketPolicy(context.Context, string) (*policy.Policy, error)
//DeleteBucketPolicy(context.Context, string) error
//
//// Supported operations check
//IsNotificationSupported() bool
//IsEncryptionSupported() bool