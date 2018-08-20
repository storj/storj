// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package logging

import (
	"bytes"
	"context"
	"errors"
	"io/ioutil"
	"testing"
	"time"

	"github.com/minio/minio/pkg/madmin"
	policy "github.com/minio/minio/pkg/policy"

	"github.com/golang/mock/gomock"
	minio "github.com/minio/minio/cmd"
	"github.com/minio/minio/pkg/auth"
	"github.com/minio/minio/pkg/hash"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

const (
	testError  = "test error"
	bucket     = "test-bucket"
	object     = "test-object"
	destBucket = "dest-bucket"
	destObject = "dest-object"
	prefix     = "test-prefix"
	marker     = "test-marker"
	partMarker = 42
	delimiter  = "test-delimiter"
	maxKeys    = 1234
	offset     = int64(12)
	length     = int64(34)
	uploadID   = "test-upload-id"
	partID     = 9876
	dryRun     = true
	duration   = 12 * time.Second
	n          = "test-n"
)

var (
	ctx      = context.Background()
	ErrTest  = errors.New(testError)
	ErrMinio = minio.BucketNotFound{}
	metadata = map[string]string{"key": "value"}
)

var (
	bucketInfo  = minio.BucketInfo{Name: bucket}
	bucketList  = []minio.BucketInfo{bucketInfo}
	objInfo     = minio.ObjectInfo{Bucket: bucket, Name: object}
	objList     = minio.ListObjectsInfo{Objects: []minio.ObjectInfo{objInfo}}
	objListV2   = minio.ListObjectsV2Info{Objects: []minio.ObjectInfo{objInfo}}
	destObjInfo = minio.ObjectInfo{Bucket: destBucket, Name: destObject}
	partInfo    = minio.PartInfo{PartNumber: partID}
	partList    = minio.ListPartsInfo{Parts: []minio.PartInfo{partInfo}}
	healItem    = madmin.HealResultItem{Bucket: bucket, Object: object}
	healList    = []madmin.HealResultItem{healItem}
	lockList    = []minio.VolumeLockInfo{minio.VolumeLockInfo{Bucket: bucket, Object: object}}
	plcy        = &policy.Policy{ID: n}
)

func TestGateway(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	gw := NewMockGateway(mockCtrl)
	lgw := Gateway(gw)

	// Test Name()
	name := "GatewayName"
	gw.EXPECT().Name().Return(name)
	assert.Equal(t, name, lgw.Name())

	// Test Production()
	production := true
	gw.EXPECT().Production().Return(production)
	assert.Equal(t, production, lgw.Production())

	// Test NewGatewayLayer() returning without error
	creds := auth.Credentials{
		AccessKey: "test-access-key",
		SecretKey: "test-secret-key",
	}
	mol := NewMockObjectLayer(mockCtrl)
	gw.EXPECT().NewGatewayLayer(creds).Return(mol, nil)
	ol, err := lgw.NewGatewayLayer(creds)
	assert.NoError(t, err)
	olw, ok := ol.(*olLogWrap)
	assert.True(t, ok)
	assert.Equal(t, mol, olw.ol)
	assert.Equal(t, zap.S(), olw.logger)

	// Test NewGatewayLayer() returning error
	gw.EXPECT().NewGatewayLayer(creds).Return(nil, ErrTest)
	_, err = lgw.NewGatewayLayer(creds)
	assert.Error(t, err, testError)
}

func TestShutdown(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger, mol, ol := initMocks(mockCtrl)

	// No error returned
	mol.EXPECT().Shutdown(ctx).Return(nil)
	err := ol.Shutdown(ctx)
	assert.NoError(t, err)

	// Error returned
	mol.EXPECT().Shutdown(ctx).Return(ErrTest)
	logger.EXPECT().Errorf(errTemplate, ErrTest)
	err = ol.Shutdown(ctx)
	assert.Error(t, err, ErrTest.Error())

	// Minio error returned
	mol.EXPECT().Shutdown(ctx).Return(ErrMinio)
	err = ol.Shutdown(ctx)
	assert.Error(t, err, ErrMinio.Error())
}

func TestStorageInfo(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	_, mol, ol := initMocks(mockCtrl)
	storageInfo := minio.StorageInfo{}

	mol.EXPECT().StorageInfo(ctx).Return(storageInfo)
	info := ol.StorageInfo(ctx)
	assert.Equal(t, storageInfo, info)
}

func TestMakeBucketWithLocation(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger, mol, ol := initMocks(mockCtrl)

	location := "test-location"

	// No error returned
	mol.EXPECT().MakeBucketWithLocation(ctx, bucket, location).Return(nil)
	err := ol.MakeBucketWithLocation(ctx, bucket, location)
	assert.NoError(t, err)

	// Error returned
	mol.EXPECT().MakeBucketWithLocation(ctx, bucket, location).Return(ErrTest)
	logger.EXPECT().Errorf(errTemplate, ErrTest)
	err = ol.MakeBucketWithLocation(ctx, bucket, location)
	assert.Error(t, err, ErrTest.Error())

	// Minio error returned
	mol.EXPECT().MakeBucketWithLocation(ctx, bucket, location).Return(ErrMinio)
	err = ol.MakeBucketWithLocation(ctx, bucket, location)
	assert.Error(t, err, ErrMinio.Error())
}

func TestGetBucketInfo(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger, mol, ol := initMocks(mockCtrl)

	// No error returned
	mol.EXPECT().GetBucketInfo(ctx, bucket).Return(bucketInfo, nil)
	info, err := ol.GetBucketInfo(ctx, bucket)
	assert.NoError(t, err)
	assert.Equal(t, bucketInfo, info)

	// Error returned
	mol.EXPECT().GetBucketInfo(ctx, bucket).Return(minio.BucketInfo{}, ErrTest)
	logger.EXPECT().Errorf(errTemplate, ErrTest)
	info, err = ol.GetBucketInfo(ctx, bucket)
	assert.Error(t, err, ErrTest.Error())
	assert.Equal(t, minio.BucketInfo{}, info)

	// Minio error returned
	mol.EXPECT().GetBucketInfo(ctx, bucket).Return(minio.BucketInfo{}, ErrMinio)
	info, err = ol.GetBucketInfo(ctx, bucket)
	assert.Error(t, err, ErrMinio.Error())
	assert.Equal(t, minio.BucketInfo{}, info)
}

func TestListBuckets(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger, mol, ol := initMocks(mockCtrl)

	// No error returned
	mol.EXPECT().ListBuckets(ctx).Return(bucketList, nil)
	list, err := ol.ListBuckets(ctx)
	assert.NoError(t, err)
	assert.Equal(t, bucketList, list)

	// Error returned
	mol.EXPECT().ListBuckets(ctx).Return([]minio.BucketInfo{}, ErrTest)
	logger.EXPECT().Errorf(errTemplate, ErrTest)
	list, err = ol.ListBuckets(ctx)
	assert.Error(t, err, ErrTest.Error())
	assert.Equal(t, []minio.BucketInfo{}, list)

	// Minio error returned
	mol.EXPECT().ListBuckets(ctx).Return([]minio.BucketInfo{}, ErrMinio)
	list, err = ol.ListBuckets(ctx)
	assert.Error(t, err, ErrMinio.Error())
	assert.Equal(t, []minio.BucketInfo{}, list)
}

func TestDeleteBucket(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger, mol, ol := initMocks(mockCtrl)

	// No error returned
	mol.EXPECT().DeleteBucket(ctx, bucket).Return(nil)
	err := ol.DeleteBucket(ctx, bucket)
	assert.NoError(t, err)

	// Error returned
	mol.EXPECT().DeleteBucket(ctx, bucket).Return(ErrTest)
	logger.EXPECT().Errorf(errTemplate, ErrTest)
	err = ol.DeleteBucket(ctx, bucket)
	assert.Error(t, err, ErrTest.Error())

	// Minio error returned
	mol.EXPECT().DeleteBucket(ctx, bucket).Return(ErrMinio)
	err = ol.DeleteBucket(ctx, bucket)
	assert.Error(t, err, ErrMinio.Error())
}

func TestListObjects(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger, mol, ol := initMocks(mockCtrl)

	// No error returned
	mol.EXPECT().ListObjects(ctx, bucket, prefix, marker, delimiter, maxKeys).
		Return(objList, nil)
	list, err := ol.ListObjects(ctx, bucket, prefix, marker, delimiter, maxKeys)
	assert.NoError(t, err)
	assert.Equal(t, objList, list)

	// Error returned
	mol.EXPECT().ListObjects(ctx, bucket, prefix, marker, delimiter, maxKeys).
		Return(minio.ListObjectsInfo{}, ErrTest)
	logger.EXPECT().Errorf(errTemplate, ErrTest)
	list, err = ol.ListObjects(ctx, bucket, prefix, marker, delimiter, maxKeys)
	assert.Error(t, err, ErrTest.Error())
	assert.Equal(t, minio.ListObjectsInfo{}, list)

	// Minio error returned
	mol.EXPECT().ListObjects(ctx, bucket, prefix, marker, delimiter, maxKeys).
		Return(minio.ListObjectsInfo{}, ErrMinio)
	list, err = ol.ListObjects(ctx, bucket, prefix, marker, delimiter, maxKeys)
	assert.Error(t, err, ErrMinio.Error())
	assert.Equal(t, minio.ListObjectsInfo{}, list)
}

func TestListObjectsV2(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger, mol, ol := initMocks(mockCtrl)

	token := "test-token"
	owner := true
	startAfter := "test-after"

	// No error returned
	mol.EXPECT().ListObjectsV2(ctx, bucket, prefix, token, delimiter, maxKeys,
		owner, startAfter).Return(objListV2, nil)
	list, err := ol.ListObjectsV2(ctx, bucket, prefix, token, delimiter,
		maxKeys, owner, startAfter)
	assert.NoError(t, err)
	assert.Equal(t, objListV2, list)

	// Error returned
	mol.EXPECT().ListObjectsV2(ctx, bucket, prefix, marker, delimiter, maxKeys,
		owner, startAfter).Return(minio.ListObjectsV2Info{}, ErrTest)
	logger.EXPECT().Errorf(errTemplate, ErrTest)
	list, err = ol.ListObjectsV2(ctx, bucket, prefix, marker, delimiter,
		maxKeys, owner, startAfter)
	assert.Error(t, err, ErrTest.Error())
	assert.Equal(t, minio.ListObjectsV2Info{}, list)

	// Minio error returned
	mol.EXPECT().ListObjectsV2(ctx, bucket, prefix, marker, delimiter, maxKeys,
		owner, startAfter).Return(minio.ListObjectsV2Info{}, ErrMinio)
	list, err = ol.ListObjectsV2(ctx, bucket, prefix, marker, delimiter,
		maxKeys, owner, startAfter)
	assert.Error(t, err, ErrMinio.Error())
	assert.Equal(t, minio.ListObjectsV2Info{}, list)
}

func TestGetObject(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger, mol, ol := initMocks(mockCtrl)

	w := ioutil.Discard
	etag := "test-etag"

	// No error returned
	mol.EXPECT().GetObject(ctx, bucket, object, offset, length, w, etag).Return(nil)
	err := ol.GetObject(ctx, bucket, object, offset, length, w, etag)
	assert.NoError(t, err)

	// Error returned
	mol.EXPECT().GetObject(ctx, bucket, object, offset, length, w, etag).Return(ErrTest)
	logger.EXPECT().Errorf(errTemplate, ErrTest)
	err = ol.GetObject(ctx, bucket, object, offset, length, w, etag)
	assert.Error(t, err, ErrTest.Error())

	// Minio error returned
	mol.EXPECT().GetObject(ctx, bucket, object, offset, length, w, etag).Return(ErrMinio)
	err = ol.GetObject(ctx, bucket, object, offset, length, w, etag)
	assert.Error(t, err, ErrMinio.Error())
}

func TestGetObjectInfo(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger, mol, ol := initMocks(mockCtrl)

	// No error returned
	mol.EXPECT().GetObjectInfo(ctx, bucket, object).Return(objInfo, nil)
	info, err := ol.GetObjectInfo(ctx, bucket, object)
	assert.NoError(t, err)
	assert.Equal(t, objInfo, info)

	// Error returned
	mol.EXPECT().GetObjectInfo(ctx, bucket, object).Return(minio.ObjectInfo{}, ErrTest)
	logger.EXPECT().Errorf(errTemplate, ErrTest)
	info, err = ol.GetObjectInfo(ctx, bucket, object)
	assert.Error(t, err, ErrTest.Error())
	assert.Equal(t, minio.ObjectInfo{}, info)

	// Minio error returned
	mol.EXPECT().GetObjectInfo(ctx, bucket, object).Return(minio.ObjectInfo{}, ErrMinio)
	info, err = ol.GetObjectInfo(ctx, bucket, object)
	assert.Error(t, err, ErrMinio.Error())
	assert.Equal(t, minio.ObjectInfo{}, info)
}

func TestPutObject(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger, mol, ol := initMocks(mockCtrl)

	data := initHashReader(t)

	// No error returned
	mol.EXPECT().PutObject(ctx, bucket, object, data, metadata).Return(objInfo, nil)
	info, err := ol.PutObject(ctx, bucket, object, data, metadata)
	assert.NoError(t, err)
	assert.Equal(t, objInfo, info)

	// Error returned
	mol.EXPECT().PutObject(ctx, bucket, object, data, metadata).Return(minio.ObjectInfo{}, ErrTest)
	logger.EXPECT().Errorf(errTemplate, ErrTest)
	info, err = ol.PutObject(ctx, bucket, object, data, metadata)
	assert.Error(t, err, ErrTest.Error())
	assert.Equal(t, minio.ObjectInfo{}, info)

	// Minio error returned
	mol.EXPECT().PutObject(ctx, bucket, object, data, metadata).Return(minio.ObjectInfo{}, ErrMinio)
	info, err = ol.PutObject(ctx, bucket, object, data, metadata)
	assert.Error(t, err, ErrMinio.Error())
	assert.Equal(t, minio.ObjectInfo{}, info)
}

func TestCopyObject(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger, mol, ol := initMocks(mockCtrl)

	// No error returned
	mol.EXPECT().CopyObject(ctx, bucket, object, destBucket, destObject, objInfo).
		Return(destObjInfo, nil)
	info, err := ol.CopyObject(ctx, bucket, object, destBucket, destObject, objInfo)
	assert.NoError(t, err)
	assert.Equal(t, destObjInfo, info)

	// Error returned
	mol.EXPECT().CopyObject(ctx, bucket, object, destBucket, destObject, objInfo).
		Return(minio.ObjectInfo{}, ErrTest)
	logger.EXPECT().Errorf(errTemplate, ErrTest)
	info, err = ol.CopyObject(ctx, bucket, object, destBucket, destObject, objInfo)
	assert.Error(t, err, ErrTest.Error())
	assert.Equal(t, minio.ObjectInfo{}, info)

	// Minio error returned
	mol.EXPECT().CopyObject(ctx, bucket, object, destBucket, destObject, objInfo).
		Return(minio.ObjectInfo{}, ErrMinio)
	info, err = ol.CopyObject(ctx, bucket, object, destBucket, destObject, objInfo)
	assert.Error(t, err, ErrMinio.Error())
	assert.Equal(t, minio.ObjectInfo{}, info)
}

func TestDeleteObject(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger, mol, ol := initMocks(mockCtrl)

	// No error returned
	mol.EXPECT().DeleteObject(ctx, bucket, object).Return(nil)
	err := ol.DeleteObject(ctx, bucket, object)
	assert.NoError(t, err)

	// Error returned
	mol.EXPECT().DeleteObject(ctx, bucket, object).Return(ErrTest)
	logger.EXPECT().Errorf(errTemplate, ErrTest)
	err = ol.DeleteObject(ctx, bucket, object)
	assert.Error(t, err, ErrTest.Error())

	// Minio error returned
	mol.EXPECT().DeleteObject(ctx, bucket, object).Return(ErrMinio)
	err = ol.DeleteObject(ctx, bucket, object)
	assert.Error(t, err, ErrMinio.Error())
}

func TestListMultipartUploads(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger, mol, ol := initMocks(mockCtrl)

	uidMarker := "test-upload-id-marker"
	listMultiParts := minio.ListMultipartsInfo{
		Uploads: []minio.MultipartInfo{minio.MultipartInfo{Object: object}}}

	// No error returned
	mol.EXPECT().ListMultipartUploads(ctx, bucket, prefix, marker, uidMarker,
		delimiter, maxKeys).Return(listMultiParts, nil)
	list, err := ol.ListMultipartUploads(ctx, bucket, prefix, marker,
		uidMarker, delimiter, maxKeys)
	assert.NoError(t, err)
	assert.Equal(t, listMultiParts, list)

	// Error returned
	mol.EXPECT().ListMultipartUploads(ctx, bucket, prefix, marker, uidMarker,
		delimiter, maxKeys).Return(minio.ListMultipartsInfo{}, ErrTest)
	logger.EXPECT().Errorf(errTemplate, ErrTest)
	list, err = ol.ListMultipartUploads(ctx, bucket, prefix, marker, uidMarker,
		delimiter, maxKeys)
	assert.Error(t, err, ErrTest.Error())
	assert.Equal(t, minio.ListMultipartsInfo{}, list)

	// Minio error returned
	mol.EXPECT().ListMultipartUploads(ctx, bucket, prefix, marker, uidMarker,
		delimiter, maxKeys).Return(minio.ListMultipartsInfo{}, ErrMinio)
	list, err = ol.ListMultipartUploads(ctx, bucket, prefix, marker, uidMarker,
		delimiter, maxKeys)
	assert.Error(t, err, ErrMinio.Error())
	assert.Equal(t, minio.ListMultipartsInfo{}, list)
}

func TestNewMultipartUpload(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger, mol, ol := initMocks(mockCtrl)

	// No error returned
	mol.EXPECT().NewMultipartUpload(ctx, bucket, object, metadata).Return(uploadID, nil)
	id, err := ol.NewMultipartUpload(ctx, bucket, object, metadata)
	assert.NoError(t, err)
	assert.Equal(t, uploadID, id)

	// Error returned
	mol.EXPECT().NewMultipartUpload(ctx, bucket, object, metadata).Return("", ErrTest)
	logger.EXPECT().Errorf(errTemplate, ErrTest)
	id, err = ol.NewMultipartUpload(ctx, bucket, object, metadata)
	assert.Error(t, err, ErrTest.Error())
	assert.Equal(t, "", id)

	// Minio error returned
	mol.EXPECT().NewMultipartUpload(ctx, bucket, object, metadata).Return("", ErrMinio)
	id, err = ol.NewMultipartUpload(ctx, bucket, object, metadata)
	assert.Error(t, err, ErrMinio.Error())
	assert.Equal(t, "", id)
}

func TestCopyObjectPart(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger, mol, ol := initMocks(mockCtrl)

	// No error returned
	mol.EXPECT().CopyObjectPart(ctx, bucket, object, destBucket, destObject,
		uploadID, partID, offset, length, objInfo).Return(partInfo, nil)
	info, err := ol.CopyObjectPart(ctx, bucket, object, destBucket, destObject,
		uploadID, partID, offset, length, objInfo)
	assert.NoError(t, err)
	assert.Equal(t, partInfo, info)

	// Error returned
	mol.EXPECT().CopyObjectPart(ctx, bucket, object, destBucket, destObject,
		uploadID, partID, offset, length, objInfo).Return(minio.PartInfo{}, ErrTest)
	logger.EXPECT().Errorf(errTemplate, ErrTest)
	info, err = ol.CopyObjectPart(ctx, bucket, object, destBucket, destObject,
		uploadID, partID, offset, length, objInfo)
	assert.Error(t, err, ErrTest.Error())
	assert.Equal(t, minio.PartInfo{}, info)

	// Minio error returned
	mol.EXPECT().CopyObjectPart(ctx, bucket, object, destBucket, destObject,
		uploadID, partID, offset, length, objInfo).Return(minio.PartInfo{}, ErrMinio)
	info, err = ol.CopyObjectPart(ctx, bucket, object, destBucket, destObject,
		uploadID, partID, offset, length, objInfo)
	assert.Error(t, err, ErrMinio.Error())
	assert.Equal(t, minio.PartInfo{}, info)
}

func TestPutObjectPart(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger, mol, ol := initMocks(mockCtrl)

	data := initHashReader(t)

	// No error returned
	mol.EXPECT().PutObjectPart(ctx, bucket, object, uploadID, partID, data).
		Return(partInfo, nil)
	info, err := ol.PutObjectPart(ctx, bucket, object, uploadID, partID, data)
	assert.NoError(t, err)
	assert.Equal(t, partInfo, info)

	// Error returned
	mol.EXPECT().PutObjectPart(ctx, bucket, object, uploadID, partID, data).
		Return(minio.PartInfo{}, ErrTest)
	logger.EXPECT().Errorf(errTemplate, ErrTest)
	info, err = ol.PutObjectPart(ctx, bucket, object, uploadID, partID, data)
	assert.Error(t, err, ErrTest.Error())
	assert.Equal(t, minio.PartInfo{}, info)

	// Minio error returned
	mol.EXPECT().PutObjectPart(ctx, bucket, object, uploadID, partID, data).
		Return(minio.PartInfo{}, ErrMinio)
	info, err = ol.PutObjectPart(ctx, bucket, object, uploadID, partID, data)
	assert.Error(t, err, ErrMinio.Error())
	assert.Equal(t, minio.PartInfo{}, info)
}

func TestListObjectParts(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger, mol, ol := initMocks(mockCtrl)

	// No error returned
	mol.EXPECT().ListObjectParts(ctx, bucket, object, uploadID, partMarker, maxKeys).
		Return(partList, nil)
	list, err := ol.ListObjectParts(ctx, bucket, object, uploadID, partMarker, maxKeys)
	assert.NoError(t, err)
	assert.Equal(t, partList, list)

	// Error returned
	mol.EXPECT().ListObjectParts(ctx, bucket, object, uploadID, partMarker, maxKeys).
		Return(minio.ListPartsInfo{}, ErrTest)
	logger.EXPECT().Errorf(errTemplate, ErrTest)
	list, err = ol.ListObjectParts(ctx, bucket, object, uploadID, partMarker, maxKeys)
	assert.Error(t, err, ErrTest.Error())
	assert.Equal(t, minio.ListPartsInfo{}, list)

	// Minio error returned
	mol.EXPECT().ListObjectParts(ctx, bucket, object, uploadID, partMarker, maxKeys).
		Return(minio.ListPartsInfo{}, ErrMinio)
	list, err = ol.ListObjectParts(ctx, bucket, object, uploadID, partMarker, maxKeys)
	assert.Error(t, err, ErrMinio.Error())
	assert.Equal(t, minio.ListPartsInfo{}, list)
}

func TestAbortMultipartUpload(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger, mol, ol := initMocks(mockCtrl)

	// No error returned
	mol.EXPECT().AbortMultipartUpload(ctx, bucket, object, uploadID).Return(nil)
	err := ol.AbortMultipartUpload(ctx, bucket, object, uploadID)
	assert.NoError(t, err)

	// Error returned
	mol.EXPECT().AbortMultipartUpload(ctx, bucket, object, uploadID).Return(ErrTest)
	logger.EXPECT().Errorf(errTemplate, ErrTest)
	err = ol.AbortMultipartUpload(ctx, bucket, object, uploadID)
	assert.Error(t, err, ErrTest.Error())

	// Minio error returned
	mol.EXPECT().AbortMultipartUpload(ctx, bucket, object, uploadID).Return(ErrMinio)
	err = ol.AbortMultipartUpload(ctx, bucket, object, uploadID)
	assert.Error(t, err, ErrMinio.Error())
}

func TestCompleteMultipartUpload(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger, mol, ol := initMocks(mockCtrl)

	parts := []minio.CompletePart{minio.CompletePart{PartNumber: partID}}

	// No error returned
	mol.EXPECT().CompleteMultipartUpload(ctx, bucket, object, uploadID, parts).
		Return(objInfo, nil)
	info, err := ol.CompleteMultipartUpload(ctx, bucket, object, uploadID, parts)
	assert.NoError(t, err)
	assert.Equal(t, objInfo, info)

	// Error returned
	mol.EXPECT().CompleteMultipartUpload(ctx, bucket, object, uploadID, parts).
		Return(minio.ObjectInfo{}, ErrTest)
	logger.EXPECT().Errorf(errTemplate, ErrTest)
	info, err = ol.CompleteMultipartUpload(ctx, bucket, object, uploadID, parts)
	assert.Error(t, err, ErrTest.Error())
	assert.Equal(t, minio.ObjectInfo{}, info)

	// Minio error returned
	mol.EXPECT().CompleteMultipartUpload(ctx, bucket, object, uploadID, parts).
		Return(minio.ObjectInfo{}, ErrMinio)
	info, err = ol.CompleteMultipartUpload(ctx, bucket, object, uploadID, parts)
	assert.Error(t, err, ErrMinio.Error())
	assert.Equal(t, minio.ObjectInfo{}, info)
}

func TestReloadFormat(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger, mol, ol := initMocks(mockCtrl)

	// No error returned
	mol.EXPECT().ReloadFormat(ctx, dryRun).Return(nil)
	err := ol.ReloadFormat(ctx, dryRun)
	assert.NoError(t, err)

	// Error returned
	mol.EXPECT().ReloadFormat(ctx, dryRun).Return(ErrTest)
	logger.EXPECT().Errorf(errTemplate, ErrTest)
	err = ol.ReloadFormat(ctx, dryRun)
	assert.Error(t, err, ErrTest.Error())

	// Minio error returned
	mol.EXPECT().ReloadFormat(ctx, dryRun).Return(ErrMinio)
	err = ol.ReloadFormat(ctx, dryRun)
	assert.Error(t, err, ErrMinio.Error())
}

func TestHealFormat(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger, mol, ol := initMocks(mockCtrl)

	// No error returned
	mol.EXPECT().HealFormat(ctx, dryRun).Return(healItem, nil)
	item, err := ol.HealFormat(ctx, dryRun)
	assert.NoError(t, err)
	assert.Equal(t, healItem, item)

	// Error returned
	mol.EXPECT().HealFormat(ctx, dryRun).Return(madmin.HealResultItem{}, ErrTest)
	logger.EXPECT().Errorf(errTemplate, ErrTest)
	item, err = ol.HealFormat(ctx, dryRun)
	assert.Error(t, err, ErrTest.Error())
	assert.Equal(t, madmin.HealResultItem{}, item)

	// Minio error returned
	mol.EXPECT().HealFormat(ctx, dryRun).Return(madmin.HealResultItem{}, ErrMinio)
	item, err = ol.HealFormat(ctx, dryRun)
	assert.Error(t, err, ErrMinio.Error())
	assert.Equal(t, madmin.HealResultItem{}, item)
}

func TestHealBucket(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger, mol, ol := initMocks(mockCtrl)

	// No error returned
	mol.EXPECT().HealBucket(ctx, bucket, dryRun).Return(healList, nil)
	list, err := ol.HealBucket(ctx, bucket, dryRun)
	assert.NoError(t, err)
	assert.Equal(t, healList, list)

	// Error returned
	mol.EXPECT().HealBucket(ctx, bucket, dryRun).Return([]madmin.HealResultItem{}, ErrTest)
	logger.EXPECT().Errorf(errTemplate, ErrTest)
	list, err = ol.HealBucket(ctx, bucket, dryRun)
	assert.Error(t, err, ErrTest.Error())
	assert.Equal(t, []madmin.HealResultItem{}, list)

	// Minio error returned
	mol.EXPECT().HealBucket(ctx, bucket, dryRun).Return([]madmin.HealResultItem{}, ErrMinio)
	list, err = ol.HealBucket(ctx, bucket, dryRun)
	assert.Error(t, err, ErrMinio.Error())
	assert.Equal(t, []madmin.HealResultItem{}, list)
}

func TestHealObject(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger, mol, ol := initMocks(mockCtrl)

	// No error returned
	mol.EXPECT().HealObject(ctx, bucket, object, dryRun).Return(healItem, nil)
	item, err := ol.HealObject(ctx, bucket, object, dryRun)
	assert.NoError(t, err)
	assert.Equal(t, healItem, item)

	// Error returned
	mol.EXPECT().HealObject(ctx, bucket, object, dryRun).Return(madmin.HealResultItem{}, ErrTest)
	logger.EXPECT().Errorf(errTemplate, ErrTest)
	item, err = ol.HealObject(ctx, bucket, object, dryRun)
	assert.Error(t, err, ErrTest.Error())
	assert.Equal(t, madmin.HealResultItem{}, item)

	// Minio error returned
	mol.EXPECT().HealObject(ctx, bucket, object, dryRun).Return(madmin.HealResultItem{}, ErrMinio)
	item, err = ol.HealObject(ctx, bucket, object, dryRun)
	assert.Error(t, err, ErrMinio.Error())
	assert.Equal(t, madmin.HealResultItem{}, item)
}

func TestListBucketsHeal(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger, mol, ol := initMocks(mockCtrl)

	// No error returned
	mol.EXPECT().ListBucketsHeal(ctx).Return(bucketList, nil)
	list, err := ol.ListBucketsHeal(ctx)
	assert.NoError(t, err)
	assert.Equal(t, bucketList, list)

	// Error returned
	mol.EXPECT().ListBucketsHeal(ctx).Return([]minio.BucketInfo{}, ErrTest)
	logger.EXPECT().Errorf(errTemplate, ErrTest)
	list, err = ol.ListBucketsHeal(ctx)
	assert.Error(t, err, ErrTest.Error())
	assert.Equal(t, []minio.BucketInfo{}, list)

	// Minio error returned
	mol.EXPECT().ListBucketsHeal(ctx).Return([]minio.BucketInfo{}, ErrMinio)
	list, err = ol.ListBucketsHeal(ctx)
	assert.Error(t, err, ErrMinio.Error())
	assert.Equal(t, []minio.BucketInfo{}, list)
}

func TestListObjectsHeal(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger, mol, ol := initMocks(mockCtrl)

	// No error returned
	mol.EXPECT().ListObjectsHeal(ctx, bucket, prefix, marker, delimiter, maxKeys).
		Return(objList, nil)
	list, err := ol.ListObjectsHeal(ctx, bucket, prefix, marker, delimiter, maxKeys)
	assert.NoError(t, err)
	assert.Equal(t, objList, list)

	// Error returned
	mol.EXPECT().ListObjectsHeal(ctx, bucket, prefix, marker, delimiter, maxKeys).
		Return(minio.ListObjectsInfo{}, ErrTest)
	logger.EXPECT().Errorf(errTemplate, ErrTest)
	list, err = ol.ListObjectsHeal(ctx, bucket, prefix, marker, delimiter, maxKeys)
	assert.Error(t, err, ErrTest.Error())
	assert.Equal(t, minio.ListObjectsInfo{}, list)

	// Minio error returned
	mol.EXPECT().ListObjectsHeal(ctx, bucket, prefix, marker, delimiter, maxKeys).
		Return(minio.ListObjectsInfo{}, ErrMinio)
	list, err = ol.ListObjectsHeal(ctx, bucket, prefix, marker, delimiter, maxKeys)
	assert.Error(t, err, ErrMinio.Error())
	assert.Equal(t, minio.ListObjectsInfo{}, list)
}

func TestListLocks(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger, mol, ol := initMocks(mockCtrl)

	// No error returned
	mol.EXPECT().ListLocks(ctx, bucket, prefix, duration).Return(lockList, nil)
	list, err := ol.ListLocks(ctx, bucket, prefix, duration)
	assert.NoError(t, err)
	assert.Equal(t, lockList, list)

	// Error returned
	mol.EXPECT().ListLocks(ctx, bucket, prefix, duration).
		Return([]minio.VolumeLockInfo{}, ErrTest)
	logger.EXPECT().Errorf(errTemplate, ErrTest)
	list, err = ol.ListLocks(ctx, bucket, prefix, duration)
	assert.Error(t, err, ErrTest.Error())
	assert.Equal(t, []minio.VolumeLockInfo{}, list)

	// Minio error returned
	mol.EXPECT().ListLocks(ctx, bucket, prefix, duration).
		Return([]minio.VolumeLockInfo{}, ErrMinio)
	list, err = ol.ListLocks(ctx, bucket, prefix, duration)
	assert.Error(t, err, ErrMinio.Error())
	assert.Equal(t, []minio.VolumeLockInfo{}, list)
}

func TestClearLocks(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger, mol, ol := initMocks(mockCtrl)

	// No error returned
	mol.EXPECT().ClearLocks(ctx, lockList).Return(nil)
	err := ol.ClearLocks(ctx, lockList)
	assert.NoError(t, err)

	// Error returned
	mol.EXPECT().ClearLocks(ctx, lockList).Return(ErrTest)
	logger.EXPECT().Errorf(errTemplate, ErrTest)
	err = ol.ClearLocks(ctx, lockList)
	assert.Error(t, err, ErrTest.Error())

	// Minio error returned
	mol.EXPECT().ClearLocks(ctx, lockList).Return(ErrMinio)
	err = ol.ClearLocks(ctx, lockList)
	assert.Error(t, err, ErrMinio.Error())
}

func TestSetBucketPolicy(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger, mol, ol := initMocks(mockCtrl)

	// No error returned
	mol.EXPECT().SetBucketPolicy(ctx, n, plcy).Return(nil)
	err := ol.SetBucketPolicy(ctx, n, plcy)
	assert.NoError(t, err)

	// Error returned
	mol.EXPECT().SetBucketPolicy(ctx, n, plcy).Return(ErrTest)
	logger.EXPECT().Errorf(errTemplate, ErrTest)
	err = ol.SetBucketPolicy(ctx, n, plcy)
	assert.Error(t, err, ErrTest.Error())

	// Minio error returned
	mol.EXPECT().SetBucketPolicy(ctx, n, plcy).Return(ErrMinio)
	err = ol.SetBucketPolicy(ctx, n, plcy)
	assert.Error(t, err, ErrMinio.Error())
}

func TestGetBucketPolicy(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger, mol, ol := initMocks(mockCtrl)

	// No error returned
	mol.EXPECT().GetBucketPolicy(ctx, n).Return(plcy, nil)
	p, err := ol.GetBucketPolicy(ctx, n)
	assert.NoError(t, err)
	assert.Equal(t, plcy, p)

	// Error returned
	mol.EXPECT().GetBucketPolicy(ctx, n).Return(nil, ErrTest)
	logger.EXPECT().Errorf(errTemplate, ErrTest)
	p, err = ol.GetBucketPolicy(ctx, n)
	assert.Error(t, err, ErrTest.Error())
	assert.Nil(t, p)

	// Minio error returned
	mol.EXPECT().GetBucketPolicy(ctx, n).Return(nil, ErrMinio)
	p, err = ol.GetBucketPolicy(ctx, n)
	assert.Error(t, err, ErrMinio.Error())
	assert.Nil(t, p)
}

func TestDeleteBucketPolicy(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger, mol, ol := initMocks(mockCtrl)

	// No error returned
	mol.EXPECT().DeleteBucketPolicy(ctx, n).Return(nil)
	err := ol.DeleteBucketPolicy(ctx, n)
	assert.NoError(t, err)

	// Error returned
	mol.EXPECT().DeleteBucketPolicy(ctx, n).Return(ErrTest)
	logger.EXPECT().Errorf(errTemplate, ErrTest)
	err = ol.DeleteBucketPolicy(ctx, n)
	assert.Error(t, err, ErrTest.Error())

	// Minio error returned
	mol.EXPECT().DeleteBucketPolicy(ctx, n).Return(ErrMinio)
	err = ol.DeleteBucketPolicy(ctx, n)
	assert.Error(t, err, ErrMinio.Error())
}

func TestIsNotificationSupported(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	_, mol, ol := initMocks(mockCtrl)

	mol.EXPECT().IsNotificationSupported().Return(true)
	assert.True(t, ol.IsNotificationSupported())

	mol.EXPECT().IsNotificationSupported().Return(false)
	assert.False(t, ol.IsNotificationSupported())
}

func TestIsEncryptionSupported(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	_, mol, ol := initMocks(mockCtrl)

	mol.EXPECT().IsEncryptionSupported().Return(true)
	assert.True(t, ol.IsEncryptionSupported())

	mol.EXPECT().IsEncryptionSupported().Return(false)
	assert.False(t, ol.IsEncryptionSupported())
}

func initMocks(mockCtrl *gomock.Controller) (*MockErrorLogger, *MockObjectLayer, olLogWrap) {
	logger := NewMockErrorLogger(mockCtrl)
	mol := NewMockObjectLayer(mockCtrl)
	ol := olLogWrap{ol: mol, logger: logger}
	return logger, mol, ol
}

func initHashReader(t *testing.T) *hash.Reader {
	data, err := hash.NewReader(bytes.NewReader([]byte("test")), 4,
		"d8e8fca2dc0f896fd7cb4cb0031ba249",
		"f2ca1bb6c7e907d06dafe4687e579fce76b37e4e93b7605022da52e6ccc26fd2")
	assert.NoError(t, err)
	return data
}
