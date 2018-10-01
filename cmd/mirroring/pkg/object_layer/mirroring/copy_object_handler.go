// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package mirroring

import (
	"context"
	minio "github.com/minio/minio/cmd"
)

func NewCopyObjectHandler(m 	     *MirroringObjectLayer,
						  ctx        context.Context,
						  srcBucket  string,
						  srcObject  string,
						  destBucket string,
						  destObject string,
						  srcInfo 	 minio.ObjectInfo,
						  srcOpts 	 minio.ObjectOptions,
						  dstOpts 	 minio.ObjectOptions) *copyObjectHandler {

	h := &copyObjectHandler{}

	h.m = m
	h.ctx =  ctx
	h.srcBucket = srcBucket
	h.srcObject = srcObject
	h.destBucket = destBucket
	h.destObject = destObject
	h.srcInfo = srcInfo
	h.srcOpts = srcOpts
	h.dstOpts = dstOpts

	return h
}

type copyObjectHandler struct {
	baseHandler
	primeInfo, alterInfo minio.ObjectInfo
	srcBucket, srcObject, destBucket, destObject string
	srcInfo minio.ObjectInfo
	srcOpts, dstOpts minio.ObjectOptions
}

func (h *copyObjectHandler) execPrime() *copyObjectHandler {
	h.primeInfo, h.primeErr =
		h.m.Prime.CopyObject(h.ctx, h.srcBucket, h.srcObject, h.destBucket, h.destObject, h.srcInfo, h.srcOpts, h.dstOpts)

	return h
}

func (h *copyObjectHandler) execAlter() *copyObjectHandler {
	h.alterInfo, h.alterErr =
		h.m.Alter.CopyObject(h.ctx, h.srcBucket, h.srcObject, h.destBucket, h.destObject, h.srcInfo, h.srcOpts, h.dstOpts)

	return h
}

func (h *copyObjectHandler) Process () (objInfo minio.ObjectInfo, err error) {
	h.execPrime()

	if h.primeErr != nil {
		return objInfo, h.primeErr
	}

	h.execAlter()

	if h.alterErr != nil {
		//h.m.Logger.Err = h.alterErr
	}

	return h.primeInfo, nil
}

