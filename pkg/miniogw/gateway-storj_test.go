// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage"
	"storj.io/storj/pkg/storage/mocks"
)

func TestGetObject(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockGetObject := mocks.NewMockStore(mockCtrl)
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
