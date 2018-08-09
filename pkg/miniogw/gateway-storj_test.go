// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package miniogw

import (
	"bytes"
	"context"
	"fmt"
	io "io"
	"math/rand"
	"testing"
	time "time"

	"github.com/golang/mock/gomock"
	"github.com/minio/minio/pkg/hash"
	"github.com/stretchr/testify/assert"

	paths "storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage/buckets"
	mock_buckets "storj.io/storj/pkg/storage/buckets/mocks"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/storage/objects"
)

func TestGetObject(t *testing.T) {
	mockBucketCtrl := gomock.NewController(t)
	defer mockBucketCtrl.Finish()

	mockObjCtrl := gomock.NewController(t)
	defer mockObjCtrl.Finish()

	mockBS := mock_buckets.NewMockStore(mockBucketCtrl)
	b := Storj{bs: mockBS}

	mockOS := NewMockStore(mockObjCtrl)
	bs := buckets.BucketStore{O: mockOS}

	storjObj := storjObjects{storj: &b}

	meta := objects.Meta{}

	for i, example := range []struct {
		bucket, object       string
		data                 string
		size, offset, length int64
		substr               string
		err                  error
		errString            string
	}{
		// happy scenario
		{"mybucket", "myobject1", "abcdef", 6, 0, 5, "abcde", nil, ""},

		// error returned by the ranger in the code
		{"mybucket", "myobject1", "abcdef", 6, -1, 7, "abcde", nil, "ranger error: negative offset"},
		{"mybucket", "myobject1", "abcdef", 6, 0, -1, "abcde", nil, "ranger error: negative length"},
		{"mybucket", "myobject1", "abcdef", 6, 1, 7, "bcde", nil, "ranger error: buffer runoff"},

		// // error returned by the objects.Get()
		// {"mybucket", "myobject1", "abcdef", 6, 0, 6, "abcdef", errors.New("some err"), "some err"},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		r := ranger.ByteRanger([]byte(example.data))

		rr := ranger.NopCloser(r)

		mockBS.EXPECT().GetObjectStore(gomock.Any(), example.bucket).Return(bs.O, example.err).Times(1)
		mockOS.EXPECT().Get(gomock.Any(), paths.New(example.object)).Return(rr, meta, example.err).Times(1)

		var buf1 bytes.Buffer
		iowriter := io.Writer(&buf1)
		err := storjObj.GetObject(context.Background(), example.bucket, example.object, example.offset, example.length, iowriter, "etag")

		if err != nil {
			assert.EqualError(t, err, example.errString)
		} else {
			assert.Equal(t, example.substr, buf1.String())
			assert.NoError(t, err, errTag)
		}
	}
}

const charset = "abcdefghijklmnopqrstuvwxyz" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" + "0123456789"

var seededRand = rand.New(
	rand.NewSource(time.Now().UnixNano()))

func stringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func checksumGen(length int) string {
	return stringWithCharset(length, charset)
}

func TestPutObject(t *testing.T) {
	mockBucketCtrl := gomock.NewController(t)
	defer mockBucketCtrl.Finish()

	mockObjCtrl := gomock.NewController(t)
	defer mockObjCtrl.Finish()

	mockBS := mock_buckets.NewMockStore(mockBucketCtrl)
	b := Storj{bs: mockBS}

	mockOS := NewMockStore(mockObjCtrl)
	bs := buckets.BucketStore{O: mockOS}

	storjObj := storjObjects{storj: &b}

	for i, example := range []struct {
		bucket, object string
		MetaKey        []string
		MetaVal        []string
		Modified       time.Time
		Expiration     time.Time
		Size           int64
		Checksum       string
		err            error // used by mock function
		errString      string
	}{
		// happy scenario
		{"mybucket", "myobject1", []string{"content-type", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), nil, ""},

		// emulating objects.Put() returning err
		// {"mybucket", "myobject1", []string{"content-type1", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), Error.New("some non nil error"), "Storj Gateway error: some non nil error"},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		var metadata = make(map[string]string)
		for i := 0x00; i < len(example.MetaKey); i++ {
			metadata[example.MetaKey[i]] = example.MetaVal[i]
		}

		//metadata serialized
		serMetaInfo := objects.SerializableMeta{
			ContentType: metadata["content-type"],
			UserDefined: metadata,
		}

		meta1 := objects.Meta{
			SerializableMeta: serMetaInfo,
			Modified:         example.Modified,
			Expiration:       example.Expiration,
			Size:             example.Size,
			Checksum:         example.Checksum,
		}

		if example.err != nil {
			meta1 = objects.Meta{}
		}

		r, err := hash.NewReader(bytes.NewReader([]byte("abcdefgiiuweriiwyrwyiywrywhti")), int64(len("abcdefgiiuweriiwyrwyiywrywhti")), "e2fc714c4727ee9395f324cd2e7f331f", "88d4266fd4e6338d13b845fcf289579d209c897823b9217da3e161936f031589")
		if err != nil {
			t.Fatal(err)
		}

		mockBS.EXPECT().GetObjectStore(gomock.Any(), example.bucket).Return(bs.O, example.err).Times(1)
		/* for valid io.reader only */
		mockOS.EXPECT().Put(gomock.Any(), paths.New(example.object), r, serMetaInfo, example.Expiration).Return(meta1, example.err).Times(1)
		objInfo, err := storjObj.PutObject(context.Background(), example.bucket, example.object, r, metadata)
		if err != nil {
			assert.EqualError(t, err, example.errString)
			if example.err != nil {
				assert.Empty(t, objInfo, errTag)
			}
		} else {
			assert.NoError(t, err, errTag)
			assert.NotNil(t, objInfo, errTag)
		}
	}
}

func TestGetObjectInfo(t *testing.T) {
	mockBucketCtrl := gomock.NewController(t)
	defer mockBucketCtrl.Finish()

	mockObjCtrl := gomock.NewController(t)
	defer mockObjCtrl.Finish()

	mockBS := mock_buckets.NewMockStore(mockBucketCtrl)
	b := Storj{bs: mockBS}

	mockOS := NewMockStore(mockObjCtrl)
	bs := buckets.BucketStore{O: mockOS}

	storjObj := storjObjects{storj: &b}

	for i, example := range []struct {
		bucket, object string
		DataStream     string
		MetaKey        []string
		MetaVal        []string
		Modified       time.Time
		Expiration     time.Time
		Size           int64
		Checksum       string
		err            error
		errString      string
	}{
		// happy scenario
		{"mybucket", "myobject1", "abcdefgiiuweriiwyrwyiywrywhti", []string{"content-type", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), nil, ""},

		// various key-value combinations with empty and non-matching keys
		{"mybucket", "myobject1", "abcdefgiiuweriiwyrwyiywrywhti", []string{"content-type1", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), nil, ""},
		{"mybucket", "myobject1", "abcdefgiiuweriiwyrwyiywrywhti", []string{}, []string{}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), nil, ""},

		// mock object.Meta function to return error
		{"mybucket", "myobject1", "abcdefghti", []string{"content-type1", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), Error.New("mock error"), "Storj Gateway error: mock error"},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		var metadata = make(map[string]string)
		for i := 0x00; i < len(example.MetaKey); i++ {
			metadata[example.MetaKey[i]] = example.MetaVal[i]
		}

		//metadata serialized
		serMetaInfo := objects.SerializableMeta{
			ContentType: metadata["content-type"],
			UserDefined: metadata,
		}

		meta1 := objects.Meta{
			SerializableMeta: serMetaInfo,
			Modified:         example.Modified,
			Expiration:       example.Expiration,
			Size:             example.Size,
			Checksum:         example.Checksum,
		}

		mockBS.EXPECT().GetObjectStore(gomock.Any(), example.bucket).Return(bs.O, nil).Times(1)
		mockOS.EXPECT().Meta(gomock.Any(), paths.New(example.object)).Return(meta1, example.err).Times(1)

		objInfo, err := storjObj.GetObjectInfo(context.Background(), example.bucket, example.object)
		if err != nil {
			assert.EqualError(t, err, example.errString)
			if example.err != nil {
				assert.Empty(t, objInfo, errTag)
			}
		} else {
			assert.NoError(t, err, errTag)
			assert.NotNil(t, objInfo, errTag)
		}
	}
}

func TestListObjects(t *testing.T) {
	mockBucketCtrl := gomock.NewController(t)
	defer mockBucketCtrl.Finish()

	mockObjCtrl := gomock.NewController(t)
	defer mockObjCtrl.Finish()

	mockBS := mock_buckets.NewMockStore(mockBucketCtrl)
	b := Storj{bs: mockBS}

	mockOS := NewMockStore(mockObjCtrl)
	bs := buckets.BucketStore{O: mockOS}

	storjObj := storjObjects{storj: &b}

	for i, example := range []struct {
		bucket, prefix    string
		marker, delimiter string
		maxKeys           int
		recursive         bool
		metaFlags         uint32
		more              bool
		MetaKey           []string
		MetaVal           []string
		Modified          time.Time
		Expiration        time.Time
		Size              int64
		Checksum          string
		NumOfListItems    int
		err               error
		errString         string
	}{
		// empty prefix
		{("bucket_" + checksumGen(10)), "", ("marker_" + checksumGen(10)), "/", 100, true, meta.All, true, []string{"content-type", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), 10, nil, ""},

		// empty marker
		{("bucket_" + checksumGen(10)), ("prefix_" + checksumGen(10)), "", "/", 100, true, meta.All, true, []string{"content-type", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), 10, nil, ""},

		// mock returning non-nil error
		{("bucket_" + checksumGen(10)), ("prefix_" + checksumGen(10)), ("marker_" + checksumGen(10)), "/", 1000, true, meta.All, true, []string{"content-type", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), 1000, Error.New("error"), "Storj Gateway error: error"},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		/* add test code here ... */
		var metadata = make(map[string]string)
		for i := 0x00; i < len(example.MetaKey); i++ {
			metadata[example.MetaKey[i]] = example.MetaVal[i]
		}

		//metadata serialized
		serMetaInfo := objects.SerializableMeta{
			ContentType: metadata["content-type"],
			UserDefined: metadata,
		}

		meta1 := objects.Meta{
			SerializableMeta: serMetaInfo,
			Modified:         example.Modified,
			Expiration:       example.Expiration,
			Size:             example.Size,
			Checksum:         example.Checksum,
		}
		items := make([]objects.ListItem, example.NumOfListItems)

		// set the item[0] to the initialized test case to keep the same starting marker
		if example.NumOfListItems > 0x00 {
			items[0].Path = paths.Path{example.bucket, example.prefix, example.marker}
			for i := 0x01; i < example.NumOfListItems; i++ {
				items[i].Path = paths.Path{example.bucket, example.prefix, ("marker_" + checksumGen(10))}
				items[i].Meta = meta1
			}
		}

		// initialize the necessary mock's argument
		startAfter := paths.New(example.marker)

		mockBS.EXPECT().GetObjectStore(gomock.Any(), example.bucket).Return(bs.O, nil).Times(1)
		mockOS.EXPECT().List(gomock.Any(), paths.New(example.prefix),
			startAfter, nil, example.recursive, example.maxKeys, example.metaFlags).Return(items, example.more, example.err).Times(1)

		objInfo, err := storjObj.ListObjects(context.Background(), example.bucket, example.prefix,
			example.marker, example.delimiter, example.maxKeys)

		if err != nil {
			assert.EqualError(t, err, example.errString)
			if example.err != nil {
				assert.Empty(t, objInfo, errTag)
			}
		} else {
			assert.NoError(t, err, errTag)
			assert.NotNil(t, objInfo, errTag)
		}
	}
}
