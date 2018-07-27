// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package miniogw

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"testing"
	time "time"

	"storj.io/storj/protos/meta"

	"github.com/golang/mock/gomock"
	"github.com/minio/minio/pkg/hash"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage"
)

func TestGetObject(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockGetObject := NewMockStore(mockCtrl)
	s := Storj{os: mockGetObject}

	testUser := storjObjects{storj: &s}

	meta := storage.Meta{}

	for _, example := range []struct {
		data                 string
		size, offset, length int64
		substr               string
		fail                 bool
	}{
		{"abcdef", 6, 0, 0, "", false},
	} {
		fh, err := ioutil.TempFile("", "test")
		if err != nil {
			t.Fatalf("failed making tempfile")
		}
		_, err = fh.Write([]byte(example.data))
		if err != nil {
			t.Fatalf("failed writing data")
		}
		name := fh.Name()
		err = fh.Close()
		if err != nil {
			t.Fatalf("failed closing data")
		}
		rr, err := ranger.FileRanger(name)
		if err != nil {
			t.Fatalf("failed opening tempfile")
		}
		defer rr.Close()
		if rr.Size() != example.size {
			t.Fatalf("invalid size: %v != %v", rr.Size(), example.size)
		}

		mockGetObject.EXPECT().Get(gomock.Any(), paths.New("mybucket", "myobject1")).Return(rr, meta, err).Times(1)

		var buf1 bytes.Buffer
		w := io.Writer(&buf1)

		err = testUser.GetObject(context.Background(), "mybucket", "myobject1", 0, 0, w, "etag")
		assert.NoError(t, err)
	}
}

func TestPutObject(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockGetObject := NewMockStore(mockCtrl)
	s := Storj{os: mockGetObject}

	testUser := storjObjects{storj: &s}

	meta1 := storage.Meta{}

	data, err := hash.NewReader(bytes.NewReader([]byte("abcd")), 4, "e2fc714c4727ee9395f324cd2e7f331f", "88d4266fd4e6338d13b845fcf289579d209c897823b9217da3e161936f031589")
	if err != nil {
		t.Fatal(err)
	}
	var metadata = make(map[string]string)
	metadata["content-type"] = "foo"

	//metadata serialized
	serMetaInfo := meta.Serializable{
		ContentType: "foo",
		UserDefined: metadata,
	}

	expTime := time.Time{}

	mockGetObject.EXPECT().Put(gomock.Any(), paths.New("mybucket", "myobject1"), data, serMetaInfo, expTime).Return(meta1, nil).Times(1)

	objInfo, err := testUser.PutObject(context.Background(), "mybucket", "myobject1", data, metadata)
	assert.NoError(t, err)
	assert.NotNil(t, objInfo)
}

func TestGetObjectInfo(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockGetObject := NewMockStore(mockCtrl)
	s := Storj{os: mockGetObject}
	testUser := storjObjects{storj: &s}

	meta1 := storage.Meta{
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
		items []storage.ListItem
	}

	var iterable = []iterationType{
		iterationType{
			items: []storage.ListItem{
				storage.ListItem{
					Path: paths.Path{"a0", "b0", "c0"},
					Meta: storage.Meta{
						Modified:   time.Now(),
						Expiration: time.Now(),
					},
				},
				// add more iternationType here...
				storage.ListItem{
					Path: paths.Path{"a1", "b1", "c1"},
					Meta: storage.Meta{
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
		metaFlags         uint64
		more              bool
	}{
		{"mybucket", "Development", "file1.go", "/", 1000, true, storage.MetaAll, false},
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
