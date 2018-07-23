// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"storj.io/storj/pkg/paths"
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

	mockGetObject.EXPECT().Get(gomock.Any(), paths.New("mybucket", "myobject1")).Return(nil, meta, nil).Times(1)

	var buf1 bytes.Buffer
	w := io.Writer(&buf1)

	err := testUser.GetObject(context.Background(), "bucke", "object", 0, 0, w, "etag")
	assert.NoError(t, err)
}
