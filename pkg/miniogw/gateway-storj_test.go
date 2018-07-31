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

	for _, example := range []struct {
		data                 string
		size, offset, length int64
		substr               string
		err                  string
	}{
		{"", 0, 0, 0, "", ""},
		{"abcdef", 6, 0, 0, "", ""},
		{"abcdef", 6, 3, 0, "", ""},
		{"abcdef", 6, 0, 6, "abcdef", ""},
		{"abcdef", 6, 0, 5, "abcde", ""},
		{"abcdef", 6, 0, 4, "abcd", ""},
		{"abcdef", 6, 1, 4, "bcde", ""},
		{"abcdef", 6, 2, 4, "cdef", ""},
		{"abcdefg", 7, 1, 4, "bcde", ""},
		{"abcdef", 6, 0, 7, "", ""},
		{"abcdef", 6, -1, 7, "abcde", "negative offset"},
		{"abcdef", 6, 0, -1, "abcde", "negative length"},
		{"abcdef", 6, 1, 7, "bcde", "buffer runoff"},
		{"abcdef", 6, 1, 7, "", "buffer runoff"},
	} {
		r := ranger.ByteRanger([]byte(example.data))
		if r.Size() != example.size {
			t.Fatalf("invalid size: %v != %v", r.Size(), example.size)
		}

		rr := ranger.NopCloser(r)

		mockGetObject.EXPECT().Get(gomock.Any(), paths.New("mybucket", "myobject1")).Return(rr, meta, errors.New(example.err)).Times(1)

		var buf1 bytes.Buffer
		w := io.Writer(&buf1)

		err := testUser.GetObject(context.Background(), "mybucket", "myobject1", example.offset, example.length, w, "etag")
		assert.EqualError(t, err, (example.err))
	}
}

const charset = "abcdefghijklmnopqrstuvwxyz" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" + "01234567890~!@#$%^&*()_+"

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
		{"abcdefghti", []string{"content-type", "userdef_key1", "userdef_key2"}, []string{"foo1", "userdef_val1", "user_val2"}, time.Now(), time.Time{}, int64(rand.Intn(1000)), checksumGen(25), ""},
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
			objects.SerializableMeta{
				ContentType: metadata[example.MetaKey[0]],
				UserDefined: metadata,
			},
			example.Modified,
			example.Expiration,
			example.Size,
			example.Checksum,
		}
		fmt.Println(example.Size, example.Checksum, meta1.Size, len(example.DataStream))

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

	meta1 := objects.Meta{
		Modified:   time.Now(),
		Expiration: time.Now(),
		Size:       int64(100),
		Checksum:   "034834890453",
	}

	mockGetObject.EXPECT().Meta(gomock.Any(), paths.New("mybucket", "myobject1")).Return(meta1, nil).Times(1)

	oi, err := testUser.GetObjectInfo(context.Background(), "mybucket", "myobject1")
	assert.NoError(t, err)
	assert.NotNil(t, oi)

}

func TestListObjects(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	// Mock framework initialization
	mockGetObject := NewMockStore(mockCtrl)
	s := Storj{os: mockGetObject}
	testUser := storjObjects{storj: &s}

	type iterationType struct {
		items []objects.ListItem
	}

	var iterable = []iterationType{
		iterationType{
			items: []objects.ListItem{
				objects.ListItem{
					Path: paths.Path{"a0", "b0", "c0"},
					Meta: objects.Meta{
						Modified:   time.Now(),
						Expiration: time.Now(),
					},
				},
				// add more iternationType here...
				objects.ListItem{
					Path: paths.Path{"a1", "b1", "c1"},
					Meta: objects.Meta{
						Modified:   time.Now(),
						Expiration: time.Now(),
					},
				},
			},
		},

		// add more iternationType here...
		//iterationType{},
	}

	// function arugment initialization
	for _, example := range []struct {
		bucket, prefix    string
		marker, delimiter string
		maxKeys           int
		recursive         bool
		metaFlags         uint32
		more              bool
	}{
		{"mybucket", "Development", "file1.go", "/", 1000, true, meta.All, false},
		// add more combinations here ...
	} {

		// initialize the necessary mock's argument
		startAfter := paths.New(example.marker)

		// initialize the necessary mock's return
		for _, mockRet := range iterable {
			mockGetObject.EXPECT().List(gomock.Any(), paths.New(example.bucket, example.prefix),
				startAfter, nil, example.recursive, example.maxKeys, example.metaFlags).Return(mockRet.items, example.more, nil).Times(1)

			oi, err := testUser.ListObjects(context.Background(), example.bucket, example.prefix,
				example.marker, example.delimiter, example.maxKeys)
			assert.NoError(t, err)
			assert.NotNil(t, oi)
		}
	}
}
