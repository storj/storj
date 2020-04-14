// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/common/useragent"
	"storj.io/common/uuid"
	"storj.io/storj/pkg/macaroon"
	"storj.io/storj/satellite/attribution"
)

// ResolvePartnerID returns partnerIDBytes as parsed or UUID corresponding to header.UserAgent.
// returns empty uuid when neither is defined.
func (endpoint *Endpoint) ResolvePartnerID(ctx context.Context, header *pb.RequestHeader, partnerIDBytes []byte) (uuid.UUID, error) {
	if header == nil {
		return uuid.UUID{}, rpcstatus.Error(rpcstatus.InvalidArgument, "header is nil")
	}

	if len(partnerIDBytes) > 0 {
		partnerID, err := uuid.FromBytes(partnerIDBytes)
		if err != nil {
			return uuid.UUID{}, rpcstatus.Errorf(rpcstatus.InvalidArgument, "unable to parse partner ID: %v", err)
		}
		return partnerID, nil
	}

	if len(header.UserAgent) == 0 {
		return uuid.UUID{}, nil
	}

	entries, err := useragent.ParseEntries(header.UserAgent)
	if err != nil {
		return uuid.UUID{}, rpcstatus.Errorf(rpcstatus.InvalidArgument, "invalid user agent %q: %v", string(header.UserAgent), err)
	}
	entries = removeUplinkUserAgent(entries)

	// no user agent defined
	if len(entries) == 0 {
		return uuid.UUID{}, nil
	}

	// Use the first partner product entry as the PartnerID.
	for _, entry := range entries {
		if entry.Product != "" {
			partner, err := endpoint.partners.ByUserAgent(ctx, entry.Product)
			if err != nil || partner.UUID.IsZero() {
				continue
			}

			return partner.UUID, nil
		}
	}

	return uuid.UUID{}, rpcstatus.Errorf(rpcstatus.InvalidArgument, "unable to resolve user agent %q", string(header.UserAgent))
}

func removeUplinkUserAgent(entries []useragent.Entry) []useragent.Entry {
	var xs []useragent.Entry
	for i := 0; i < len(entries); i++ {
		// If it's "uplink" then skip it.
		if strings.EqualFold(entries[i].Product, "uplink") {
			// also skip any associated comments
			for i+1 < len(entries) && entries[i+1].Comment != "" {
				i++
			}
			continue
		}

		xs = append(xs, entries[i])
	}
	return xs
}

// SetAttributionOld tries to add attribution to the bucket.
func (endpoint *Endpoint) SetAttributionOld(ctx context.Context, req *pb.SetAttributionRequestOld) (_ *pb.SetAttributionResponseOld, err error) {
	defer mon.Task()(&ctx)(&err)

	err = endpoint.setBucketAttribution(ctx, req.Header, req.BucketName, req.PartnerId)

	return &pb.SetAttributionResponseOld{}, err
}

// SetBucketAttribution sets the bucket attribution.
func (endpoint *Endpoint) SetBucketAttribution(ctx context.Context, req *pb.BucketSetAttributionRequest) (resp *pb.BucketSetAttributionResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	err = endpoint.setBucketAttribution(ctx, req.Header, req.Name, req.PartnerId)

	return &pb.BucketSetAttributionResponse{}, err
}

func (endpoint *Endpoint) setBucketAttribution(ctx context.Context, header *pb.RequestHeader, bucketName []byte, partnerIDBytes []byte) error {
	if header == nil {
		return rpcstatus.Error(rpcstatus.InvalidArgument, "header is nil")
	}

	keyInfo, err := endpoint.validateAuth(ctx, header, macaroon.Action{
		Op:            macaroon.ActionList,
		Bucket:        bucketName,
		EncryptedPath: []byte(""),
		Time:          time.Now(),
	})
	if err != nil {
		return err
	}

	partnerID, err := endpoint.ResolvePartnerID(ctx, header, partnerIDBytes)
	if err != nil {
		return rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}
	if partnerID.IsZero() {
		return rpcstatus.Error(rpcstatus.InvalidArgument, "unknown user agent or partner id")
	}

	// check if attribution is set for given bucket
	_, err = endpoint.attributions.Get(ctx, keyInfo.ProjectID, bucketName)
	if err == nil {
		endpoint.log.Info("bucket already attributed", zap.ByteString("bucketName", bucketName), zap.Stringer("Partner ID", partnerID))
		return nil
	}

	if !attribution.ErrBucketNotAttributed.Has(err) {
		// try only to set the attribution, when it's missing
		endpoint.log.Error("error while getting attribution from DB", zap.Error(err))
		return rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	empty, err := endpoint.metainfo.IsBucketEmpty(ctx, keyInfo.ProjectID, bucketName)
	if err != nil {
		return rpcstatus.Error(rpcstatus.Internal, err.Error())
	}
	if !empty {
		return rpcstatus.Errorf(rpcstatus.AlreadyExists, "bucket %q is not empty, PartnerID %q cannot be attributed", bucketName, partnerID)
	}

	// checks if bucket exists before updates it or makes a new entry
	bucket, err := endpoint.metainfo.GetBucket(ctx, bucketName, keyInfo.ProjectID)
	if err != nil {
		if storj.ErrBucketNotFound.Has(err) {
			return rpcstatus.Errorf(rpcstatus.NotFound, "bucket %q does not exist", bucketName)
		}
		endpoint.log.Error("error while getting bucket", zap.ByteString("bucketName", bucketName), zap.Error(err))
		return rpcstatus.Error(rpcstatus.Internal, "unable to set bucket attribution")
	}
	if !bucket.PartnerID.IsZero() {
		endpoint.log.Info("bucket already attributed", zap.ByteString("bucketName", bucketName), zap.Stringer("Partner ID", partnerID))
		return nil
	}

	// update bucket information
	bucket.PartnerID = partnerID
	_, err = endpoint.metainfo.UpdateBucket(ctx, bucket)
	if err != nil {
		endpoint.log.Error("error while updating bucket", zap.ByteString("bucketName", bucketName), zap.Error(err))
		return rpcstatus.Error(rpcstatus.Internal, "unable to set bucket attribution")
	}

	// update attribution table
	_, err = endpoint.attributions.Insert(ctx, &attribution.Info{
		ProjectID:  keyInfo.ProjectID,
		BucketName: bucketName,
		PartnerID:  partnerID,
	})
	if err != nil {
		endpoint.log.Error("error while inserting attribution to DB", zap.Error(err))
		return rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	return nil
}
