// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package miniogw

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"testing"
	"time"

	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/storage"

	"github.com/golang/mock/gomock"
	minio "github.com/minio/minio/cmd"
	"github.com/minio/minio/pkg/hash"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage/buckets"
	mock_buckets "storj.io/storj/pkg/storage/buckets/mocks"
	"storj.io/storj/pkg/storage/objects"
)

var (
	ctx = context.Background()
)

func TestGetObject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBS := mock_buckets.NewMockStore(ctrl)
	b := Storj{bs: mockBS}

	mockOS := NewMockStore(ctrl)

	storjObj := storjObjects{storj: &b}

	meta := objects.Meta{}

	for i, example := range []struct {
		bucket, object string
		data           string
		offset, length int64
		substr         string
		err            error
		errString      string
	}{
		// happy scenario
		{"mybucket", "myobject1", "abcdef", 0, 5, "abcde", nil, ""},
		// error returned by the ranger in the code
		{"mybucket", "myobject1", "abcdef", -1, 7, "abcde", nil, "ranger error: negative offset"},
		{"mybucket", "myobject1", "abcdef", 0, -1, "abcde", nil, "ranger error: negative length"},
		{"mybucket", "myobject1", "abcdef", 1, 7, "bcde", nil, "ranger error: buffer runoff"},
		// error returned by the objects.Get()
		{"mybucket", "myobject1", "abcdef", 0, 6, "abcdef", errors.New("some err"), "some err"},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		rr := ranger.NopCloser(ranger.ByteRanger([]byte(example.data)))

		mockBS.EXPECT().GetObjectStore(gomock.Any(), example.bucket).Return(mockOS, nil)
		mockOS.EXPECT().Get(gomock.Any(), paths.New(example.object)).Return(rr, meta, example.err)

		var buf bytes.Buffer
		iowriter := io.Writer(&buf)
		err := storjObj.GetObject(ctx, example.bucket, example.object, example.offset, example.length, iowriter, "etag")

		if err != nil {
			assert.EqualError(t, err, example.errString, errTag)
		} else {
			assert.Equal(t, example.substr, buf.String(), errTag)
			assert.NoError(t, err, errTag)
		}
	}
}

func TestDeleteObject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBS := mock_buckets.NewMockStore(ctrl)
	b := Storj{bs: mockBS}

	mockOS := NewMockStore(ctrl)

	storjObj := storjObjects{storj: &b}

	for i, example := range []struct {
		bucket, object string
		err            error
		errString      string
	}{
		// happy scenario
		{"mybucket", "myobject1", nil, ""},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		mockBS.EXPECT().GetObjectStore(gomock.Any(), example.bucket).Return(mockOS, nil)
		mockOS.EXPECT().Delete(gomock.Any(), paths.New(example.object)).Return(example.err)

		err := storjObj.DeleteObject(ctx, example.bucket, example.object)
		assert.NoError(t, err, errTag)
	}
}

func TestPutObject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBS := mock_buckets.NewMockStore(ctrl)
	b := Storj{bs: mockBS}

	mockOS := NewMockStore(ctrl)

	storjObj := storjObjects{storj: &b}

	data, err := hash.NewReader(bytes.NewReader([]byte("abcdefgiiuweriiwyrwyiywrywhti")),
		int64(len("abcdefgiiuweriiwyrwyiywrywhti")),
		"e2fc714c4727ee9395f324cd2e7f331f",
		"88d4266fd4e6338d13b845fcf289579d209c897823b9217da3e161936f031589")
	if err != nil {
		t.Fatal(err)
	}

	for i, example := range []struct {
		bucket, object string
		err            error // used by mock function
		errString      string
	}{
		// happy scenario
		{"mybucket", "myobject1", nil, ""},
		// emulating objects.Put() returning err
		{"mybucket", "myobject1", Error.New("some non nil error"), "Storj Gateway error: some non nil error"},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		metadata := map[string]string{
			"content-type": "media/foo",
			"userdef_key1": "userdef_val1",
			"userdef_key2": "userdef_val2",
		}

		serMeta := objects.SerializableMeta{
			ContentType: metadata["content-type"],
			UserDefined: map[string]string{
				"userdef_key1": metadata["userdef_key1"],
				"userdef_key2": metadata["userdef_key2"],
			},
		}

		meta := objects.Meta{
			SerializableMeta: serMeta,
			Modified:         time.Now(),
			Expiration:       time.Time{},
			Size:             1234,
			Checksum:         "test-checksum",
		}

		mockBS.EXPECT().GetObjectStore(gomock.Any(), example.bucket).Return(mockOS, nil)
		mockOS.EXPECT().Put(gomock.Any(), paths.New(example.object), data, serMeta, time.Time{}).Return(meta, example.err)

		objInfo, err := storjObj.PutObject(ctx, example.bucket, example.object, data, metadata)
		if err != nil {
			assert.EqualError(t, err, example.errString, errTag)
		} else {
			assert.NoError(t, err, errTag)
		}

		assert.NotNil(t, objInfo, errTag)
		assert.Equal(t, example.bucket, objInfo.Bucket, errTag)
		assert.Equal(t, example.object, objInfo.Name, errTag)
		assert.Equal(t, meta.Modified, objInfo.ModTime, errTag)
		assert.Equal(t, meta.Size, objInfo.Size, errTag)
		assert.Equal(t, meta.Checksum, objInfo.ETag, errTag)
		assert.Equal(t, meta.UserDefined, objInfo.UserDefined, errTag)

	}
}

func TestGetObjectInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBS := mock_buckets.NewMockStore(ctrl)
	b := Storj{bs: mockBS}

	mockOS := NewMockStore(ctrl)

	storjObj := storjObjects{storj: &b}

	meta := objects.Meta{
		Modified:   time.Now(),
		Expiration: time.Time{},
		Size:       1234,
		Checksum:   "test-checksum",
		SerializableMeta: objects.SerializableMeta{
			ContentType: "media/foo",
			UserDefined: map[string]string{
				"userdef_key1": "userdef_val1",
				"userdef_key2": "userdef_val2",
			},
		},
	}

	for i, example := range []struct {
		bucket, object string
		err            error
		errString      string
	}{
		// happy scenario
		{"mybucket", "myobject1", nil, ""},
		// mock object.Meta function to return error
		{"mybucket", "myobject1", Error.New("mock error"), "Storj Gateway error: mock error"},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		mockBS.EXPECT().GetObjectStore(gomock.Any(), example.bucket).Return(mockOS, nil)
		mockOS.EXPECT().Meta(gomock.Any(), paths.New(example.object)).Return(meta, example.err)

		objInfo, err := storjObj.GetObjectInfo(ctx, example.bucket, example.object)
		if err != nil {
			assert.EqualError(t, err, example.errString, errTag)
			if example.err != nil {
				assert.Empty(t, objInfo, errTag)
			}
		} else {
			assert.NoError(t, err, errTag)
			assert.NotNil(t, objInfo, errTag)
			assert.Equal(t, example.bucket, objInfo.Bucket, errTag)
			assert.Equal(t, example.object, objInfo.Name, errTag)
			assert.Equal(t, meta.Modified, objInfo.ModTime, errTag)
			assert.Equal(t, meta.Size, objInfo.Size, errTag)
			assert.Equal(t, meta.Checksum, objInfo.ETag, errTag)
			assert.Equal(t, meta.UserDefined, objInfo.UserDefined, errTag)
		}
	}
}

func TestListObjects(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBS := mock_buckets.NewMockStore(ctrl)
	b := Storj{bs: mockBS}

	mockOS := NewMockStore(ctrl)

	storjObj := storjObjects{storj: &b}

	bucket := "test-bucket"
	prefix := "test-prefix"
	delimiter := "test-delimiter"
	maxKeys := 123

	items := []objects.ListItem{
		objects.ListItem{
			Path: paths.New(prefix, "test-file-1.txt"),
		},
		objects.ListItem{
			Path: paths.New(prefix, "test-file-2.txt"),
		},
	}

	objInfos := []minio.ObjectInfo{
		minio.ObjectInfo{
			Bucket: bucket,
			Name:   path.Join(prefix, "test-file-1.txt"),
		},
		minio.ObjectInfo{
			Bucket: bucket,
			Name:   path.Join(prefix, "test-file-2.txt"),
		},
	}

	for i, example := range []struct {
		more       bool
		startAfter string
		nextMarker string
		err        error
		errString  string
	}{
		{false, "", "", nil, ""},
		{true, "test-start-after", "test-file-2.txt", nil, ""},
		// mock returning non-nil error
		{false, "", "", Error.New("error"), "Storj Gateway error: error"},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		mockBS.EXPECT().GetObjectStore(gomock.Any(), bucket).Return(mockOS, nil)
		mockOS.EXPECT().List(gomock.Any(), paths.New(prefix), paths.New(example.startAfter),
			nil, true, maxKeys, meta.All).Return(items, example.more, example.err)

		listInfo, err := storjObj.ListObjects(ctx, bucket, prefix, example.startAfter, delimiter, maxKeys)

		if err != nil {
			assert.EqualError(t, err, example.errString, errTag)
			if example.err != nil {
				assert.Empty(t, listInfo, errTag)
			}
		} else {
			assert.NoError(t, err, errTag)
			assert.NotNil(t, listInfo, errTag)
			assert.Equal(t, example.more, listInfo.IsTruncated, errTag)
			assert.Equal(t, example.nextMarker, listInfo.NextMarker, errTag)
			assert.Equal(t, objInfos, listInfo.Objects, errTag)
			assert.Nil(t, listInfo.Prefixes, errTag)
		}
	}
}

func TestDeleteBucket(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOS := NewMockStore(ctrl)
	mockBS := mock_buckets.NewMockStore(ctrl)
	b := Storj{bs: mockBS}

	storjObj := storjObjects{storj: &b}

	itemsInBucket := make([]objects.ListItem, 1)
	itemsInBucket[0] = objects.ListItem{Path: paths.New("path1"), Meta: objects.Meta{}}

	var exp time.Time
	exp = time.Unix(0, 0).UTC()

	var noItemsInBucket []objects.ListItem

	for i, example := range []struct {
		bucket       string
		items        []objects.ListItem
		bucketStatus error
		err          error
		errString    string
	}{
		{"mybucket", noItemsInBucket, nil, nil, ""},
		{"mybucket", noItemsInBucket, storage.ErrKeyNotFound.New("mybucket"), nil, "Bucket not found: mybucket"},
		{"mybucket", itemsInBucket, nil, minio.BucketNotEmpty{Bucket: "mybucket"}, "Bucket not empty: mybucket"},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		mockBS.EXPECT().Get(gomock.Any(), gomock.Any()).Return(buckets.Meta{Created: exp}, example.bucketStatus)
		if !storage.ErrKeyNotFound.Has(example.bucketStatus) {
			mockBS.EXPECT().GetObjectStore(gomock.Any(), example.bucket).Return(mockOS, nil)
			mockOS.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
				gomock.Any(), gomock.Any(), gomock.Any()).Return(example.items, false, example.err)
			if len(example.items) == 0 {
				mockBS.EXPECT().Delete(gomock.Any(), example.bucket).Return(example.err)
			}
		}

		err := storjObj.DeleteBucket(ctx, example.bucket)
		if err != nil {
			assert.EqualError(t, err, example.errString, errTag)
		} else {
			assert.NoError(t, err, errTag)
		}
	}
}

func TestGetBucketInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBS := mock_buckets.NewMockStore(ctrl)
	b := Storj{bs: mockBS}

	storjObj := storjObjects{storj: &b}

	var exp time.Time
	exp = time.Unix(0, 0).UTC()

	for i, example := range []struct {
		bucket    string
		meta      time.Time
		err       error
		errString string
	}{
		// happy scenario
		{"mybucket", exp, nil, ""},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		mockBS.EXPECT().Get(gomock.Any(), example.bucket).Return(buckets.Meta{Created: example.meta}, example.err)

		_, err := storjObj.GetBucketInfo(ctx, example.bucket)
		assert.NoError(t, err, errTag)
	}
}

func TestMakeBucketWithLocation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBS := mock_buckets.NewMockStore(ctrl)
	b := Storj{bs: mockBS}

	storjObj := storjObjects{storj: &b}

	var exp time.Time
	exp = time.Unix(0, 0).UTC()

	for i, example := range []struct {
		bucket       string
		meta         time.Time
		retErr       error
		bucketStatus error
	}{
		{"mybucket", exp, minio.BucketAlreadyExists{Bucket: "mybucket"}, nil},
		{"mybucket", exp, nil, storage.ErrKeyNotFound.New("mybucket")},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		mockBS.EXPECT().Get(gomock.Any(), gomock.Any()).Return(buckets.Meta{Created: exp}, example.bucketStatus)
		if storage.ErrKeyNotFound.Has(example.bucketStatus) {
			mockBS.EXPECT().Put(gomock.Any(), example.bucket).Return(buckets.Meta{Created: example.meta}, nil)
		}

		err := storjObj.MakeBucketWithLocation(ctx, example.bucket, "location")
		if example.retErr != nil {
			assert.NotNil(t, err, errTag)
			assert.Equal(t, example.retErr, err, errTag)
		} else {
			assert.Nil(t, err, errTag)
		}
	}
}

func TestListBuckets(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBS := mock_buckets.NewMockStore(ctrl)
	b := Storj{bs: mockBS}

	storjObj := storjObjects{storj: &b}

	var exp time.Time
	exp = time.Unix(0, 0).UTC()

	for i, example := range []struct {
		bucket    string
		meta      time.Time
		more      bool
		err       error
		errString string
	}{
		// happy scenario
		{"mybucket", exp, false, nil, ""},
		{"mybucket", exp, true, nil, ""},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		b := make([]buckets.ListItem, 5)
		for i, item := range b {
			item.Bucket = fmt.Sprintf("bucket %d", i)
			item.Meta = buckets.Meta{Created: exp}
		}
		mockBS.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any(), 0).Return(b, example.more, example.err).AnyTimes()

		_, err := storjObj.ListBuckets(ctx)
		assert.NoError(t, err, errTag)
	}
}
