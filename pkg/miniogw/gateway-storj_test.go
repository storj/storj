// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"

	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/storage"
	"storj.io/storj/pkg/storage/mocks"
	"storj.io/storj/pkg/storage/objects"
)

func TestGetObject(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockGetObject := mocks.NewMockStore(mockCtrl)
	testUser := objects.NewStore(nil)

	meta := storage.Meta{}

	mockGetObject.EXPECT().Get(context.Background(), paths.New("mybucket", "myobject1")).Return(nil, meta, nil).Times(1)

	testUser.Get(context.Background(), paths.New("mybucket", "myobject1"))
}
