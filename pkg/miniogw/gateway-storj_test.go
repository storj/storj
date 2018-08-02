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
		// most obnoxious case - happy scenario
		{"mybucket", "myobject1", "", 0, 0, 0, "", nil, "", w},
		{"x", "y", "abcdef", 6, 0, 6, "abcdef", nil, "", w},
		{"mybucket", "myobject1", "abcdef", 6, 0, 5, "abcde", nil, "", w},
		{"mybucket", "myobject1", "abcdef", 6, 0, 4, "abcd", nil, "", w},
		{"mybucket", "myobject1", "abcdef", 6, 1, 4, "bcde", nil, "", w},
		{"mybucket", "myobject1", "abcdef", 6, 2, 4, "cdef", nil, "", w},
		{"mybucket", "myobject1", "abcdefg", 7, 1, 4, "bcde", nil, "", w},

		// error returned by the objects.Put()
		{"mybucket", "myobject1", "abcdef", 6, 0, 6, "abcdef", errors.New("some err"), "some err", w},

		// errors returned by the ranger in the code
		{"mybucket", "myobject1", "abcdef", 6, 0, 7, "", nil, "ranger error: buffer runoff", w},
		{"mybucket", "myobject1", "abcdef", 6, -1, 7, "abcde", nil, "ranger error: negative offset", w},
		{"mybucket", "myobject1", "abcdef", 6, 0, -1, "abcde", nil, "ranger error: negative length", w},
		{"mybucket", "myobject1", "abcdef", 6, 1, 7, "bcde", nil, "ranger error: buffer runoff", w},
		{"mybucket", "myobject1", "abcdef", 6, 1, 7, "", nil, "ranger error: buffer runoff", w},

		// invalid io.writer
		{"", "", "abcdef", 6, 0, 6, "abcdef", nil, "Storj Gateway error: Invalid argument(s)", nil},
		{"mybucket", "myobject1", "abcdef", 6, 0, 6, "abcdef", nil, "Storj Gateway error: Invalid argument(s)", nil},

		// empty bucket and/or object
		{"mybucket", "", "abcdef", 6, 0, 0, "", nil, "Storj Gateway error: Invalid argument(s)", w},
		{"", "myobject1", "abcdef", 6, 3, 0, "", nil, "Storj Gateway error: Invalid argument(s)", w},
		{"", "", "abcdef", 6, 0, 6, "abcdef", nil, "Storj Gateway error: Invalid argument(s)", w},
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

	for i, example := range []struct {
		bucket, object string
		DataStream     string
		MetaKey        []string
		MetaVal        []string
		Modified       time.Time
		Expiration     time.Time
		Size           int64
		Checksum       string
		err            error // used by mock function
		errString      string
	}{
		// most obnoxious case - happy scenario
		{"mybucket", "myobject1", "abcdefgiiuweriiwyrwyiywrywhti", []string{"content-type", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), nil, ""},

		// emulating objects.Put() returning err
		{"mybucket", "myobject1", "abcdefghti", []string{"content-type1", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), Error.New("some non nil error"), "Storj Gateway error: some non nil error"},
		{"mybucket", "myobject1", "abcdefghti", []string{}, []string{}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), Error.New("some non nil error"), "Storj Gateway error: some non nil error"},
		{"mybucket", "myobject1", "", []string{}, []string{}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), Error.New("some non nil error"), "Storj Gateway error: some non nil error"},

		// emulate invalid bucket and/or object
		{"mybucket", "", "", []string{}, []string{}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), nil, "Storj Gateway error: Invalid argument(s)"},
		{"", "myobject1", "", []string{}, []string{}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), nil, "Storj Gateway error: Invalid argument(s)"},

		// emulate invalid io.reader
		{"", "", "abcdefgiiuweriiwyrwyiywrywhti", []string{"content-type", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(0x00), checksumGen(25), nil, "Storj Gateway error: Invalid argument(s)"},

		// emulate invalid bucket object and io.Reader
		{"", "", "", []string{}, []string{}, time.Now(), time.Time{}, int64(0x00), checksumGen(25), Error.New("DON'T CARE"), "Storj Gateway error: Invalid argument(s)"},
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
			serMetaInfo,
			example.Modified,
			example.Expiration,
			example.Size,
			example.Checksum,
		}

		if example.err != nil {
			meta1 = objects.Meta{}
		}

		data, err := hash.NewReader(bytes.NewReader([]byte(example.DataStream)), int64(len(example.DataStream)), "e2fc714c4727ee9395f324cd2e7f331f", "88d4266fd4e6338d13b845fcf289579d209c897823b9217da3e161936f031589")
		if err != nil {
			t.Fatal(err)
		}

		if (example.Size == 0x00) || (example.bucket == "") || (example.object == "") {
			/* Intentionlly set the size to zero to emulate a nil hash.Reader */
			data, err = nil, nil
		} else {
			/* for valid io.reader only */
			mockGetObject.EXPECT().Put(gomock.Any(), paths.New(example.bucket, example.object), data, serMetaInfo, example.Expiration).Return(meta1, example.err).Times(1)
		}

		objInfo, err := testUser.PutObject(context.Background(), example.bucket, example.object, data, metadata)
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
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockGetObject := NewMockStore(mockCtrl)
	s := Storj{os: mockGetObject}

	testUser := storjObjects{storj: &s}

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
		// most obnoxious case - happy scenario
		{"mybucket", "myobject1", "abcdefgiiuweriiwyrwyiywrywhti", []string{"content-type", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), nil, ""},

		// various combination of arguments (in obnoxious mode)
		{"mybucket", "myobject1", "abcdefgiiuweriiwyrwyiywrywhti", []string{"content-type1", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), nil, ""},
		{"mybucket", "myobject1", "abcdefgiiuweriiwyrwyiywrywhti", []string{}, []string{}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), nil, ""},

		// invalid bucket and/or object
		{"", "myobject1", "abcdefgiiuweriiwyrwyiywrywhti", []string{"content-type", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), nil, "Storj Gateway error: Invalid argument(s)"},
		{"mybucket", "", "abcdefgiiuweriiwyrwyiywrywhti", []string{"content-type", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), nil, "Storj Gateway error: Invalid argument(s)"},
		{"", "", "abcdefgiiuweriiwyrwyiywrywhti", []string{"content-type", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), nil, "Storj Gateway error: Invalid argument(s)"},

		// mock function to return error
		{"mybucket", "myobject1", "abcdefghti", []string{"content-type1", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), Error.New("error"), "Storj Gateway error: error"},
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
			serMetaInfo,
			example.Modified,
			example.Expiration,
			example.Size,
			example.Checksum,
		}
		if (example.bucket == "") || (example.object == "") {
			/* dont execute the mock's EXPECT() if any of the above 3 conditions are true */
		} else {
			mockGetObject.EXPECT().Meta(gomock.Any(), paths.New(example.bucket, example.object)).Return(meta1, example.err).Times(1)
		}

		objInfo, err := testUser.GetObjectInfo(context.Background(), example.bucket, example.object)
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
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	// Mock framework initialization
	mockGetObject := NewMockStore(mockCtrl)
	s := Storj{os: mockGetObject}
	testUser := storjObjects{storj: &s}

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
		/* initialize the structure */

		// happy scenario with  request items 1 and returned items 1
		{("bucket_" + checksumGen(10)), ("prefix_" + checksumGen(10)), ("marker_" + checksumGen(10)), "/", 1, true, meta.All, true, []string{"content-type", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), 1, nil, ""},

		// happy scenario with  request items 2 and returned items 2
		{("bucket_" + checksumGen(10)), ("prefix_" + checksumGen(10)), ("marker_" + checksumGen(10)), "/", 2, true, meta.All, true, []string{"content-type", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), 2, nil, ""},

		// happy scenario with requested 10 and returned items 0
		{("bucket_" + checksumGen(10)), ("prefix_" + checksumGen(10)), ("marker_" + checksumGen(10)), "/", 10, true, meta.All, true, []string{"content-type", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), 0, nil, ""},

		// happy scenario with requested 10 and returned items 100
		{("bucket_" + checksumGen(10)), ("prefix_" + checksumGen(10)), ("marker_" + checksumGen(10)), "/", 10, true, meta.All, true, []string{"content-type", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), 100, nil, ""},

		// happy scenario with requested 100 and returned items 10
		{("bucket_" + checksumGen(10)), ("prefix_" + checksumGen(10)), ("marker_" + checksumGen(10)), "/", 100, true, meta.All, true, []string{"content-type", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), 10, nil, ""},

		// happy scenario with requested 1000 and returned items 2000
		{("bucket_" + checksumGen(10)), ("prefix_" + checksumGen(10)), ("marker_" + checksumGen(10)), "/", 1000, true, meta.All, true, []string{"content-type", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), 2000, nil, ""},

		// happy scenario with requested 2000 and returned items 2000
		{("bucket_" + checksumGen(10)), ("prefix_" + checksumGen(10)), ("marker_" + checksumGen(10)), "/", 2000, true, meta.All, true, []string{"content-type", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), 2000, nil, ""},

		// happy scenario with requested -10 and returned items 2000
		{("bucket_" + checksumGen(10)), ("prefix_" + checksumGen(10)), ("marker_" + checksumGen(10)), "/", -10, true, meta.All, true, []string{"content-type", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), 2000, nil, "Storj Gateway error: Invalid argument(s)"},

		// invalid bucket
		{"", ("prefix_" + checksumGen(10)), ("marker_" + checksumGen(10)), "/", 100, true, meta.All, true, []string{"content-type", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), 10, nil, "Storj Gateway error: Invalid argument(s)"},

		// // empty prefix
		{("bucket_" + checksumGen(10)), "", ("marker_" + checksumGen(10)), "/", 100, true, meta.All, true, []string{"content-type", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), 10, nil, ""},

		// invalid bucket and/or object
		{("bucket_" + checksumGen(10)), ("prefix_" + checksumGen(10)), ("marker_" + checksumGen(10)), "/", 1000, true, meta.All, true, []string{"content-type", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), 2000, nil, ""},

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
			serMetaInfo,
			example.Modified,
			example.Expiration,
			example.Size,
			example.Checksum,
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

		if example.bucket == "" {
			/* dont execute the mock's EXPECT() if any of the above 3 conditions are true */
		} else {
			maxKeys := example.maxKeys
			if maxKeys > 0 {
				if maxKeys > 1000 {
					maxKeys = 1000
				}
				mockGetObject.EXPECT().List(gomock.Any(), paths.New(example.bucket, example.prefix),
					startAfter, nil, example.recursive, maxKeys, example.metaFlags).Return(items, example.more, example.err).Times(1)
			}
		}

		objInfo, err := testUser.ListObjects(context.Background(), example.bucket, example.prefix,
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
