// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package mirroring

import (
	"context"
)

func NewDeleteBucketHandler(m *MirroringObjectLayer, ctx context.Context, bucket string) *deleteBucketHandler {

	h := &deleteBucketHandler{}

	h.m = m
	h.ctx =  ctx
	h.bucket = bucket

	return h
}

type deleteBucketHandler struct {
	baseHandler
	bucket, object string
}

func (h *deleteBucketHandler) execPrime() *deleteBucketHandler {
	h.primeErr = h.m.Prime.DeleteBucket(h.ctx, h.bucket)

	return h
}

func (h *deleteBucketHandler) execAlter() *deleteBucketHandler {
	h.alterErr = h.m.Alter.DeleteBucket(h.ctx, h.bucket)

	return h
}

func (h *deleteBucketHandler) Process () error {
	h.execPrime()

	if h.primeErr != nil {
		return  h.primeErr
	}

	h.execAlter()

	if h.alterErr != nil {
		//h.m.Logger.Err = h.alterErr
	}

	return nil
}


