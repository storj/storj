// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"

	"storj.io/common/pb"
	"storj.io/common/signing"
	"storj.io/storj/satellite/internalpb"
)

// SignStreamID signs the stream ID using the specified signer.
// Signer is a satellite.
func SignStreamID(ctx context.Context, signer signing.Signer, unsigned *internalpb.StreamID) (_ *internalpb.StreamID, err error) {
	defer mon.Task()(&ctx)(&err)
	bytes, err := EncodeStreamID(ctx, unsigned)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	signed := *unsigned
	signed.SatelliteSignature, err = signer.SignHMACSHA256(ctx, bytes)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &signed, nil
}

// SignSegmentID signs the segment ID using the specified signer.
// Signer is a satellite.
func SignSegmentID(ctx context.Context, signer signing.Signer, unsigned *internalpb.SegmentID) (_ *internalpb.SegmentID, err error) {
	defer mon.Task()(&ctx)(&err)
	bytes, err := EncodeSegmentID(ctx, unsigned)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	signed := *unsigned
	signed.SatelliteSignature, err = signer.HashAndSign(ctx, bytes)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &signed, nil
}

// EncodeStreamID encodes stream ID into bytes for signing.
func EncodeStreamID(ctx context.Context, streamID *internalpb.StreamID) (_ []byte, err error) {
	defer mon.Task()(&ctx)(&err)
	signature := streamID.SatelliteSignature
	streamID.SatelliteSignature = nil
	out, err := pb.Marshal(streamID)
	streamID.SatelliteSignature = signature
	return out, err
}

// EncodeSegmentID encodes segment ID into bytes for signing.
func EncodeSegmentID(ctx context.Context, segmentID *internalpb.SegmentID) (_ []byte, err error) {
	defer mon.Task()(&ctx)(&err)
	signature := segmentID.SatelliteSignature
	segmentID.SatelliteSignature = nil
	out, err := pb.Marshal(segmentID)
	segmentID.SatelliteSignature = signature
	return out, err
}

// VerifyStreamID verifies that the signature inside stream ID belongs to the satellite.
func VerifyStreamID(ctx context.Context, satellite signing.Signer, signed *internalpb.StreamID) (err error) {
	defer mon.Task()(&ctx)(&err)
	bytes, err := EncodeStreamID(ctx, signed)
	if err != nil {
		return Error.Wrap(err)
	}

	return satellite.VerifyHMACSHA256(ctx, bytes, signed.SatelliteSignature)
}

// VerifySegmentID verifies that the signature inside segment ID belongs to the satellite.
func VerifySegmentID(ctx context.Context, satellite signing.Signee, signed *internalpb.SegmentID) (err error) {
	defer mon.Task()(&ctx)(&err)
	bytes, err := EncodeSegmentID(ctx, signed)
	if err != nil {
		return Error.Wrap(err)
	}

	return satellite.HashAndVerifySignature(ctx, bytes, signed.SatelliteSignature)
}
