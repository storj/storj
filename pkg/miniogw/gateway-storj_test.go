// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package miniogw

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	io "io"
	"math/rand"
	"testing"
	time "time"

	"github.com/golang/mock/gomock"
	"github.com/minio/minio/pkg/hash"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/storage/objects"
)

func TestGetObject(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockGetObject := NewMockStore(mockCtrl)
	s := Storj{os: mockGetObject}

	testUser := storjObjects{storj: &s}

	meta := objects.Meta{}

	var buf1 bytes.Buffer
	w := io.Writer(&buf1)
	iowriter := w

	for i, example := range []struct {
		bucket, object       string
		data                 string
		size, offset, length int64
		substr               string
		err                  error
		errString            string
		iowriter             io.Writer
	}{
		{"mybucket", "myobject1", "", 0, 0, 0, "", nil, "", w},
		{"mybucket", "", "abcdef", 6, 0, 0, "", nil, "Storj Gateway error: Invalid argument(s)", w},
		{"", "myobject1", "abcdef", 6, 3, 0, "", nil, "Storj Gateway error: Invalid argument(s)", w},
		{"x", "y", "abcdef", 6, 0, 6, "abcdef", nil, "", w},
		{"mybucket", "myobject1", "abcdef", 6, 0, 6, "abcdef", errors.New("some err"), "some err", w},
		{"mybucket", "myobject1", "abcdef", 6, 0, 5, "abcde", nil, "", w},
		{"mybucket", "myobject1", "abcdef", 6, 0, 4, "abcd", nil, "", w},
		{"mybucket", "myobject1", "abcdef", 6, 1, 4, "bcde", nil, "", w},
		{"mybucket", "myobject1", "abcdef", 6, 2, 4, "cdef", nil, "", w},
		{"mybucket", "myobject1", "abcdefg", 7, 1, 4, "bcde", nil, "", w},
		{"mybucket", "myobject1", "abcdef", 6, 0, 7, "", nil, "ranger error: buffer runoff", w},
		{"mybucket", "myobject1", "abcdef", 6, -1, 7, "abcde", nil, "ranger error: negative offset", w},
		{"mybucket", "myobject1", "abcdef", 6, 0, -1, "abcde", nil, "ranger error: negative length", w},
		{"mybucket", "myobject1", "abcdef", 6, 1, 7, "bcde", nil, "ranger error: buffer runoff", w},
		{"mybucket", "myobject1", "abcdef", 6, 1, 7, "", nil, "ranger error: buffer runoff", w},
		{"mybucket", "myobject1", "abcdef", 6, 0, 6, "abcdef", nil, "Storj Gateway error: Invalid argument(s)", nil},
		{"", "", "abcdef", 6, 0, 6, "abcdef", nil, "Storj Gateway error: Invalid argument(s)", nil},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		r := ranger.ByteRanger([]byte(example.data))
		if r.Size() != example.size {
			t.Fatalf("invalid size: %v != %v", r.Size(), example.size)
		}

		iowriter = example.iowriter

		rr := ranger.NopCloser(r)

		if iowriter == nil || example.bucket == "" || example.object == "" {
			/* dont execute the mock's EXPECT() if any of the above 3 conditions are true */
		} else {
			mockGetObject.EXPECT().Get(gomock.Any(), paths.New(example.bucket, example.object)).Return(rr, meta, example.err).Times(1)
		}
		err := testUser.GetObject(context.Background(), example.bucket, example.object, example.offset, example.length, iowriter, "etag")

		if err != nil {
			assert.EqualError(t, err, example.errString)
		} else {
			assert.NoError(t, err, errTag)
		}
	}
}

const charset = "abcdefghijklmnopqrstuvwxyz" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" + "0123456789"

var seededRand *rand.Rand = rand.New(
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
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockGetObject := NewMockStore(mockCtrl)
	s := Storj{os: mockGetObject}

	testUser := storjObjects{storj: &s}

	for _, example := range []struct {
		DataStream string
		MetaKey    []string
		MetaVal    []string
		Modified   time.Time
		Expiration time.Time
		Size       int64
		Checksum   string
		err        string
	}{
		{"abcdefgiiuweriiwyrwyiywrywhti", []string{"content-type", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), ""},
		{"abcdefghti", []string{"content-type1", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), "some non nil error"},
	} {
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
			serMetaInfo,
			example.Modified,
			example.Expiration,
			example.Size,
			example.Checksum,
		}

		data, err := hash.NewReader(bytes.NewReader([]byte(example.DataStream)), int64(len(example.DataStream)), "e2fc714c4727ee9395f324cd2e7f331f", "88d4266fd4e6338d13b845fcf289579d209c897823b9217da3e161936f031589")
		if err != nil {
			t.Fatal(err)
		}

		mockGetObject.EXPECT().Put(gomock.Any(), paths.New("mybucket", "myobject1"), data, serMetaInfo, example.Expiration).Return(meta1, errors.New(example.err)).Times(1)

		objInfo, err := testUser.PutObject(context.Background(), "mybucket", "myobject1", data, metadata)
		assert.EqualError(t, err, (example.err))
		assert.NotNil(t, objInfo)
	}
}

func TestGetObjectInfo(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockGetObject := NewMockStore(mockCtrl)
	s := Storj{os: mockGetObject}

	testUser := storjObjects{storj: &s}

	for _, example := range []struct {
		DataStream string
		MetaKey    []string
		MetaVal    []string
		Modified   time.Time
		Expiration time.Time
		Size       int64
		Checksum   string
		err        string
	}{
		{"abcdefgiiuweriiwyrwyiywrywhti", []string{"content-type", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), ""},
		{"abcdefghti", []string{"content-type1", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), "some non nil error"},
	} {
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
			serMetaInfo,
			example.Modified,
			example.Expiration,
			example.Size,
			example.Checksum,
		}
		mockGetObject.EXPECT().Meta(gomock.Any(), paths.New("mybucket", "myobject1")).Return(meta1, errors.New(example.err)).Times(1)

		oi, err := testUser.GetObjectInfo(context.Background(), "mybucket", "myobject1")
		assert.EqualError(t, err, (example.err))
		assert.NotNil(t, oi)
	}
}

func TestListObjects(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	// Mock framework initialization
	mockGetObject := NewMockStore(mockCtrl)
	s := Storj{os: mockGetObject}
	testUser := storjObjects{storj: &s}

	for _, example := range []struct {
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
	}{
		/* initialize the structure */
		{("bucket_" + checksumGen(10)), ("prefix_" + checksumGen(10)), ("marker_" + checksumGen(10)), "/", 1000, true, meta.All, true, []string{"content-type", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), 1, nil},
		{("bucket_" + checksumGen(10)), ("prefix_" + checksumGen(10)), ("marker_" + checksumGen(10)), "/", 1000, true, meta.All, true, []string{"content-type", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), 2, nil},
		{("bucket_" + checksumGen(10)), ("prefix_" + checksumGen(10)), ("marker_" + checksumGen(10)), "/", 1000, true, meta.All, true, []string{"content-type", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), 2, errors.New("some error")},
	} {

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
			serMetaInfo,
			example.Modified,
			example.Expiration,
			example.Size,
			example.Checksum,
		}
		items := make([]objects.ListItem, example.NumOfListItems)

		for i := 0x00; i < example.NumOfListItems; i++ {
			items[i].Path = paths.Path{example.bucket, example.prefix}
			items[i].Meta = meta1
		}

		// initialize the necessary mock's argument
		startAfter := paths.New(example.marker)

		mockGetObject.EXPECT().List(gomock.Any(), paths.New(example.bucket, example.prefix),
			startAfter, nil, example.recursive, example.maxKeys, example.metaFlags).Return(items, example.more, example.err).Times(1)

		oi, err := testUser.ListObjects(context.Background(), example.bucket, example.prefix,
			example.marker, example.delimiter, example.maxKeys)

		if example.err != nil {
			assert.NotNil(t, err)
		} else {
			assert.Nil(t, err)
		}
		assert.NotNil(t, oi)
	}
}
