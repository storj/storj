// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package segments

import (
	"io"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"storj.io/storj/pkg/paths"
)

func TestSegmentStore(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for _, tt := range []struct {
		path       paths.Path
		data       io.Reader
		expiration time.Time
	}{} {

	}
}
