package mirroring

import (
	"context"
	"github.com/minio/minio/pkg/hash"
	"io"
	"storj.io/mirroring/utils"

	minio "github.com/minio/minio/cmd"
)

//MirroringObjectLayer is
type MirroringObjectLayer struct {
	minio.GatewayUnsupported
	Prime  minio.ObjectLayer
	Alter  minio.ObjectLayer
	Logger utils.Logger
}

//ObjectLayer interface---------------------------------------------------------------------------------------------------------------------

func (m *MirroringObjectLayer) Shutdown(ctx context.Context) error {
	return nil
}

func (m *MirroringObjectLayer) StorageInfo(ctx context.Context) (storageInfo minio.StorageInfo) {
	return storageInfo
}

func (m *MirroringObjectLayer) MakeBucketWithLocation(ctx context.Context, bucket string, location string) error {

	h := NewMakeBucketHandler(m, ctx, bucket, location)

	return h.Process()
}

// Returns bucket name and creation date of the bucket.
// Parameters:
// ctx    - current context.
// bucket - bucket name.
func (m *MirroringObjectLayer) GetBucketInfo(ctx context.Context, bucket string) (bucketInfo minio.BucketInfo, err error) {

	bucketInfo, errPrime := m.Prime.GetBucketInfo(ctx, bucket)

	if errPrime == nil {
		return
	}

	bucketInfo, err = m.Alter.GetBucketInfo(ctx, bucket)

	//m.Logger.Err = utils.CombineErrors([]error{errPrime, err})

	if err != nil {
		err = utils.CombineErrors([]error{errPrime, err})
	}

	return
}

// Returns a list of all buckets.
// Parameters:
// ctx - current context.
func (m *MirroringObjectLayer) ListBuckets(ctx context.Context) (buckets []minio.BucketInfo, err error) {

	primeBuckets, errPrime := m.Prime.ListBuckets(ctx)
	alterBuckets, errAlter := m.Alter.ListBuckets(ctx)

	return m.processBucketList(primeBuckets, alterBuckets, errPrime, errAlter)
}

// Deletes the bucket named in the URI.
// All objects (including all object versions and delete markers) in the bucket
// must be deleted before the bucket itself can be deleted.
// Parameters:
// ctx    - current context.
// bucket - bucket name.
func (m *MirroringObjectLayer) DeleteBucket(ctx context.Context, bucket string) error {

	h := NewDeleteBucketHandler(m, ctx, bucket)

	return h.Process()
}

// ListObjects is a paginated operation.
// Multiple API calls may be issued in order to retrieve the entire data set of results.
// You can disable pagination by providing the --no-paginate argument.
// Returns some or all (up to 1000) of the objects in a bucket.
// You can use the request parameters as selection criteria to return a subset of the objects in a bucket.
// Parameters:
// ctx       - current context.
// bucket    - bucket name.
// prefix    - Limits the response to keys that begin with the specified prefix.
// marker    - Specifies the key to start with when listing objects in a bucket.
// 			   Amazon S3 returns object keys in UTF-8 binary order, starting with key after the marker in order.
// delimiter - is a character you use to group keys.
// maxKeys   - Sets the maximum number of keys returned in the response body.
// 			   If you want to retrieve fewer than the default 1,000 keys, you can add this to your request.
//             Default value is 1000
func (m *MirroringObjectLayer) ListObjects(ctx context.Context,
	bucket string,
	prefix string,
	marker string,
	delimiter string,
	maxKeys int) (loi minio.ListObjectsInfo, err error) {

	primeObjects, errPrime := m.Prime.ListObjects(ctx, bucket, prefix, marker, delimiter, maxKeys)
	alterObjects, errAlter := m.Alter.ListObjects(ctx, bucket, prefix, marker, delimiter, maxKeys)

	result := minio.ListObjectsInfo{}

	obj, err := m.processObjList(primeObjects.Objects, alterObjects.Objects, errPrime, errAlter)

	if err != nil {
		return result, err
	}

	result = initializeListObjectsInfo(primeObjects, alterObjects, obj, errPrime)

	result.Objects = obj
	return result, nil
}

// This implementation of the GET operation returns some or all (up to 1,000) of the objects in a bucket.
// You can use the request parameters as selection criteria to return a subset of the objects in a bucket.
// A 200 OK response can contain valid or invalid XML.
// Make sure to design your application to parse the contents of the response and handle it appropriately.
// Parameters:
// ctx 	   - current context.
// bucket  - bucket name.
// prefix  - Limits the response to keys that begin with the specified prefix.
// cntnTkn - when the response to this API call is truncated (that is, the IsTruncated response element value is true),
// 			 the response also includes the NextContinuationToken element.
// 			 To list the next set of objects, you can use the NextContinuationToken element in the next request as the continuation-token.
// 			 Amazon S3 returns object keys in UTF-8 binary order, starting with key after the marker in order.
// delim   - is a character you use to group keys.
// maxKeys - Sets the maximum number of keys returned in the response body.
// 		     If you want to retrieve fewer than the default 1,000 keys, you can add this to your request.
//           Default value is 1000.
func (m *MirroringObjectLayer) ListObjectsV2(ctx        context.Context,
											 bucket     string,
											 prefix     string,
											 cntnTkn    string,
											 delim      string,
											 maxKeys    int,
											 fetchOwner bool,
											 startAfter string) (minio.ListObjectsV2Info, error) {

	primeObjects, errPrime := m.Prime.ListObjectsV2(ctx, bucket, prefix, cntnTkn, delim, maxKeys, fetchOwner, startAfter)
	alterObjects, errAlter := m.Alter.ListObjectsV2(ctx, bucket, prefix, cntnTkn, delim, maxKeys, fetchOwner, startAfter)

	result := minio.ListObjectsV2Info{}

	obj, err := m.processObjList(primeObjects.Objects, alterObjects.Objects, errPrime, errAlter)

	if err != nil {
		return result, err
	}

	result = initializeListObjectsV2Info(primeObjects, alterObjects, obj, errPrime)

	result.Objects = obj

	return result, nil
}

// Retrieves an object
// Parameters:
// ctx         - current context.
// bucket      - bucket name.
// object      - object name.
// startOffset - indicates the starting read location of the object.
// length      - indicates the total length of the object.
// etag        - An ETag is an opaque identifier assigned by a web server
// 			     to a specific version of a resource found at a URL
// opts        -
func (m *MirroringObjectLayer) GetObject(ctx 		 context.Context,
										 bucket 	 string,
										 object      string,
										 startOffset int64,
										 length 	 int64,
										 writer 	 io.Writer,
									     etag 	     string,
										 opts 		 minio.ObjectOptions) (err error) {

	return m.error(m.Prime.GetObject(ctx, bucket, object, startOffset, length, writer, etag, opts),
		m.Alter.GetObject(ctx, bucket, object, startOffset, length, writer, etag, opts))
}

// Returns information about object.
// Parameters:
// ctx    - current context.
// bucket - bucket name.
// object - object name.
func (m *MirroringObjectLayer) GetObjectInfo(ctx    context.Context,
											 bucket string,
											 object string,
											 opts   minio.ObjectOptions) (objInfo minio.ObjectInfo, err error) {

	objInfo, errPrime := m.Prime.GetObjectInfo(ctx, bucket, object, opts)

	if errPrime == nil {
		return
	}

	objInfo, err = m.Alter.GetObjectInfo(ctx, bucket, object, opts)

	// m.Logger.Err = utils.CombineErrors([]error{errPrime, err})

	if err != nil {
		err = utils.CombineErrors([]error{errPrime, err})
	}

	return
}

// PutObject adds an object to a bucket.
// Parameters:
// ctx         - current context.
// bucket      - bucket name.
// object      - object name.
// metadata    - A map of metadata to store with the object.
func (m *MirroringObjectLayer) PutObject(ctx context.Context, bucket string, object string, data *hash.Reader, metadata map[string]string, opts minio.ObjectOptions) (objInfo minio.ObjectInfo, err error) {
	//TODO: decide prime and alter based on config
	h := newPutHandler(m.Prime, m.Alter, m.Logger)
	return h.process(ctx, bucket, object, data, metadata, opts)
}

// Creates a cp of an object that is already stored in a bucket.
// Parameters:
// srcBucket  - The name of the source bucket
// srcObject  - Key name of the source object
// destBucket - The name of the destination bucket
// destObject - Key name of the destination object
// srcInfo    - represents object metadata
func (m *MirroringObjectLayer) CopyObject(ctx 		 context.Context,
										  srcBucket  string,
										  srcObject  string,
										  destBucket string,
										  destObject string,
										  srcInfo 	 minio.ObjectInfo,
										  srcOpts 	 minio.ObjectOptions,
										  destOpts 	 minio.ObjectOptions) (minio.ObjectInfo, error) {

	h := NewCopyObjectHandler(m, ctx, srcBucket, srcObject, destBucket, destObject, srcInfo, srcOpts, destOpts)

	return h.Process()
}

// Deletes the bucket named in the URI.
// Parameters:
// ctx    - current context.
// bucket - bucket name.
// object - object name
func (m *MirroringObjectLayer) DeleteObject(ctx context.Context, bucket, object string) error {

	h := NewDeleteObjectHandler(m, ctx, bucket, object)

	return h.Process()
}

//PRIVATE METHODS-------------------------------------------------------------------------------------------------------------------------

// This method combines both errors, creates new error,
// depends on error messages of previous error and place it in Logger.
// Returns nil if at least 1 error is nil.
func (m *MirroringObjectLayer) error(errPrime, errAlter error) error {
	// m.Logger.Err = utils.CombineErrors([]error{errPrime, errAlter})

	if errPrime != nil && errAlter != nil {
		return utils.CombineErrors([]error{errPrime, errAlter})
	}

	return nil
}

// This method combines both errors, creates new error,
// depends on error messages of previous error and place it in Logger.
// Returns nil as error if at least 1 error is nil.
// Also checks both minio.ObjectInfo args and returns first non nil.
func (m *MirroringObjectLayer) errorWithResult(objInfoPrime minio.ObjectInfo,
	objInfoAlter minio.ObjectInfo,
	errPrime error,
	errAlter error) (objInfo minio.ObjectInfo, err error) {

	// m.Logger.Err = utils.CombineErrors([]error{errPrime, errAlter})

	if errPrime != nil && errAlter != nil {
		err = utils.CombineErrors([]error{errPrime, errAlter})
		return
	}

	if errPrime != nil {
		objInfo = objInfoAlter
	} else {
		objInfo = objInfoPrime
	}

	return
}

// This method combines both errors, creates new error,
// depends on error messages of previous error and place it in Logger.
// Returns nil as error if at least 1 error is nil.
// Also checks both []minio.ObjectInfo and returns distinct union of both.
func (m *MirroringObjectLayer) processObjList(primeObjects []minio.ObjectInfo,
	alterObjects []minio.ObjectInfo,
	errPrime error,
	errAlter error) ([]minio.ObjectInfo, error) {

	// m.Logger.Err = utils.CombineErrors([]error{errPrime, errAlter})

	if errPrime != nil && errAlter != nil {
		return nil, utils.CombineErrors([]error{errPrime, errAlter})
	}

	//TODO: place this in the logger somehow
	utils.ListObjectsWithDifference(primeObjects, alterObjects)

	return utils.CombineObjectsDistinct(primeObjects, alterObjects), nil
}

// This method combines both errors, creates new error,
// depends on error messages of previous error and place it in Logger.
// Returns nil as error if at least 1 error is nil.
// Also checks both []minio.BucketInfo and returns distinct union of both.
func (m *MirroringObjectLayer) processBucketList(primeBuckets []minio.BucketInfo,
	alterBuckets []minio.BucketInfo,
	errPrime error,
	errAlter error) ([]minio.BucketInfo, error) {

	// m.Logger.Err = utils.CombineErrors([]error{errPrime, errAlter})

	if errPrime != nil && errAlter != nil {
		return nil, utils.CombineErrors([]error{errPrime, errAlter})
	}

	//TODO: place this in the logger somehow
	utils.ListBucketsWithDifference(primeBuckets, alterBuckets)

	return utils.CombineBucketsDistinct(primeBuckets, alterBuckets), nil
}

func initializeListObjectsV2Info(prime minio.ListObjectsV2Info,
	alter minio.ListObjectsV2Info,
	objects []minio.ObjectInfo,
	errPrime error) (result minio.ListObjectsV2Info) {

	result = minio.ListObjectsV2Info{}

	if errPrime != nil {
		result = prime
	} else {
		result = alter
	}

	result.Objects = objects

	return
}

func initializeListObjectsInfo(prime minio.ListObjectsInfo,
	alter minio.ListObjectsInfo,
	objects []minio.ObjectInfo,
	errPrime error) (result minio.ListObjectsInfo) {

	result = minio.ListObjectsInfo{}

	if errPrime != nil {
		result = prime
	} else {
		result = alter
	}

	result.Objects = objects

	return
}
