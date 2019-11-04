// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"context"
	"io"
	"io/ioutil"
	"time"

	"github.com/zeebo/errs"
	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink/metainfo"
	"storj.io/storj/uplink/storage/segments"
)

// commitUpload is a function signature used to commit a metainfo segment upload
// operation.
type commitUpload func(results []*pb.SegmentPieceUploadResult, sizeEncryptedData int64)

// metainfoSegment is the result of a metainfo segment upload operation.
type metainfoSegment struct {
	IsRemote        bool
	Data            io.Reader
	Limits          []*pb.AddressedOrderLimit
	PiecePrivateKey storj.PiecePrivateKey
	Commit          commitUpload
}

// metainfoSegmentUpload manages the upload of segments metainfo. It must be
// created through a constructor function.
//
// Instances aren't concurrent safe.
type metainfoSegmentsUpload struct {
	client                  *metainfo.Client
	cipher                  storj.CipherSuite
	maxEncryptedSegmentSize int64
	inlineThreshold         int

	beginObjectReq       *metainfo.BeginObjectParams
	currentSegment       int32
	streamID             storj.StreamID
	lastCommitSegmentReq *metainfo.CommitSegmentParams

	// canUpload controls if upload can be called because previous call was
	// required to commit the operation.
	canUpload bool
}

// newMetainfoSegmentUpload creates a metainfoSegmentUpload instance.
func newMetainfoSegmentsUpload(
	c *metainfo.Client, bucket string, encryptedPath string, expiration time.Time,
	cipher storj.CipherSuite, maxEncryptedSegmentSize int64, inlineThreshold int,
) *metainfoSegmentsUpload {

	return &metainfoSegmentsUpload{
		client:                  c,
		cipher:                  cipher,
		maxEncryptedSegmentSize: maxEncryptedSegmentSize,
		inlineThreshold:         inlineThreshold,

		beginObjectReq: &metainfo.BeginObjectParams{
			Bucket:        []byte(bucket),
			EncryptedPath: []byte(encryptedPath),
			ExpiresAt:     expiration,
		},
		currentSegment: 0,

		canUpload: true,
	}
}

// Upload uploads the metadata information of the segment to the satellite and
// the data of the segment if the segment is determined to be inline.
//
// metainfoSegment only has values for the fields Limits, PiecePrivateKey and
// Commit if IsRemote field is true. When IsRemote is true, the function set to
// Commit field must be called after the segment has been upload to the Storage
// Nodes and before to call Upload again. Commit function cannot be called
// concurrently with metainfoSegmentsUpload methods.
//
// metainfoSegment Data field returns a io.Reader which can be consumed by the
// the caller because the segment io.Reader is consumed.
//
func (metainfoUpload *metainfoSegmentsUpload) Upload(
	ctx context.Context, segment io.Reader, segmentEncryption storj.SegmentEncryption,
	contentKey storj.Key, contentNonce storj.Nonce,
) (_ metainfoSegment, err error) {
	if !metainfoUpload.canUpload {
		return metainfoSegment{}, errs.New("Previous Upload call must be commited before calling Upload again")
	}

	peekReader := segments.NewPeekThresholdReader(segment)
	// If the data is larger than the inline threshold size, then it will be a remote segment
	isRemote, err := peekReader.IsLargerThan(metainfoUpload.inlineThreshold)
	if err != nil {
		return metainfoSegment{}, err
	}

	defer func() {
		if err == nil {
			metainfoUpload.currentSegment++
		}
	}()

	if isRemote {
		beginSegment := &metainfo.BeginSegmentParams{
			MaxOrderLimit: metainfoUpload.maxEncryptedSegmentSize,
			Position: storj.SegmentPosition{
				Index: metainfoUpload.currentSegment,
			},
		}

		var responses []metainfo.BatchResponse
		if metainfoUpload.currentSegment == 0 {
			responses, err = metainfoUpload.client.Batch(ctx, metainfoUpload.beginObjectReq, beginSegment)
			if err != nil {
				return metainfoSegment{}, err
			}
			objResponse, err := responses[0].BeginObject()
			if err != nil {
				return metainfoSegment{}, err
			}

			metainfoUpload.streamID = objResponse.StreamID
		} else {
			beginSegment.StreamID = metainfoUpload.streamID
			responses, err = metainfoUpload.client.Batch(ctx, metainfoUpload.lastCommitSegmentReq, beginSegment)
			if err != nil {
				return metainfoSegment{}, err
			}
		}
		segResponse, err := responses[1].BeginSegment()
		if err != nil {
			return metainfoSegment{}, err
		}

		metainfoUpload.canUpload = false
		commit := commitUpload(func(results []*pb.SegmentPieceUploadResult, size int64) {

			metainfoUpload.canUpload = true
			metainfoUpload.lastCommitSegmentReq = &metainfo.CommitSegmentParams{
				SegmentID:         segResponse.SegmentID,
				SizeEncryptedData: size,
				Encryption:        segmentEncryption,
				UploadResult:      results,
			}
		})

		return metainfoSegment{
			IsRemote:        true,
			Data:            peekReader,
			Limits:          segResponse.Limits,
			PiecePrivateKey: segResponse.PiecePrivateKey,
			Commit:          commit,
		}, nil
	}

	data, err := ioutil.ReadAll(peekReader)
	if err != nil {
		return metainfoSegment{}, err
	}
	cipherData, err := encryption.Encrypt(data, metainfoUpload.cipher, &contentKey, &contentNonce)
	if err != nil {
		return metainfoSegment{}, err
	}

	makeInlineSegment := &metainfo.MakeInlineSegmentParams{
		Position: storj.SegmentPosition{
			Index: metainfoUpload.currentSegment,
		},
		Encryption:          segmentEncryption,
		EncryptedInlineData: cipherData,
	}

	if metainfoUpload.currentSegment == 0 {
		responses, err := metainfoUpload.client.Batch(ctx, metainfoUpload.beginObjectReq, makeInlineSegment)
		if err != nil {
			return metainfoSegment{}, err
		}
		objResponse, err := responses[0].BeginObject()
		if err != nil {
			return metainfoSegment{}, err
		}

		metainfoUpload.streamID = objResponse.StreamID
	} else {
		makeInlineSegment.StreamID = metainfoUpload.streamID
		err = metainfoUpload.client.MakeInlineSegment(ctx, *makeInlineSegment)
		if err != nil {
			return metainfoSegment{}, err
		}
	}

	return metainfoSegment{
		IsRemote: false,
		Data:     peekReader,
	}, nil
}

// Close finalizes a metainfo segments upload. It has to be always called once
// all the segments upload is finished.
func (metainfoUpload *metainfoSegmentsUpload) Close(
	ctx context.Context, objectMetadata []byte,
) (storj.StreamID, error) {
	var (
		err          error
		commitObject = metainfo.CommitObjectParams{
			StreamID:          metainfoUpload.streamID,
			EncryptedMetadata: objectMetadata,
		}
	)

	if metainfoUpload.lastCommitSegmentReq != nil {
		_, err = metainfoUpload.client.Batch(ctx, metainfoUpload.lastCommitSegmentReq, &commitObject)
	} else {
		err = metainfoUpload.client.CommitObject(ctx, commitObject)
	}

	return metainfoUpload.streamID, err
}
