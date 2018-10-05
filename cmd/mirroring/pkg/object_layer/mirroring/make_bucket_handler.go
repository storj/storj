package mirroring

import (
	"context"
)

func NewMakeBucketHandler(m *MirroringObjectLayer, ctx context.Context, bucket, location string) *makeBucketHandler {
	h:= &makeBucketHandler{}

	h.m = m
	h.ctx = ctx
	h.bucket = bucket
	h.location = location

	return h
}

type makeBucketHandler struct {
	baseHandler
	bucket, location string
}

func (h *makeBucketHandler) execPrime() *makeBucketHandler {
	h.primeErr = h.m.Prime.MakeBucketWithLocation(h.ctx, h.bucket, h.location)

	return h
}

func (h *makeBucketHandler) execAlter() *makeBucketHandler {
	h.alterErr = h.m.Alter.MakeBucketWithLocation(h.ctx, h.bucket, h.location)

	return h
}

func (h *makeBucketHandler) Process () error {
	h.execPrime()

	if h.primeErr != nil {
		return h.primeErr
	}

	h.execAlter()

	if h.alterErr != nil {
		//h.m.Logger.Err = h.alterErr
	}

	return nil
}
