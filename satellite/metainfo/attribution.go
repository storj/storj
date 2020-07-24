// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"strings"
	"sync"

	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/common/useragent"
	"storj.io/common/uuid"
	"storj.io/drpc/drpccache"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/console"
)

// ensureAttribution ensures that the bucketName has the partner information specified by keyInfo partner ID or the header user agent.
// PartnerID from keyInfo is a value associated with registered user and prevails over header user agent.
//
// Assumes that the user has permissions sufficient for authenticating.
func (endpoint *Endpoint) ensureAttribution(ctx context.Context, header *pb.RequestHeader, keyInfo *console.APIKeyInfo, bucketName []byte) error {
	if header == nil {
		return rpcstatus.Error(rpcstatus.InvalidArgument, "header is nil")
	}
	if len(header.UserAgent) == 0 && keyInfo.PartnerID.IsZero() {
		return nil
	}

	if conncache := drpccache.FromContext(ctx); conncache != nil {
		cache := conncache.LoadOrCreate(attributionCheckCacheKey{},
			func() interface{} {
				return &attributionCheckCache{}
			}).(*attributionCheckCache)
		if !cache.needsCheck(string(bucketName)) {
			return nil
		}
	}

	var err error
	partnerID := keyInfo.PartnerID
	if partnerID.IsZero() {
		partnerID, err = endpoint.ResolvePartnerID(ctx, header)
		if err != nil {
			return err
		}
		if partnerID.IsZero() {
			return nil
		}
	}

	err = endpoint.tryUpdateBucketAttribution(ctx, header, keyInfo.ProjectID, bucketName, partnerID)
	if errs2.IsRPC(err, rpcstatus.NotFound) || errs2.IsRPC(err, rpcstatus.AlreadyExists) {
		return nil
	}
	return err
}

// ResolvePartnerID returns partnerIDBytes as parsed or UUID corresponding to header.UserAgent.
// returns empty uuid when neither is defined.
func (endpoint *Endpoint) ResolvePartnerID(ctx context.Context, header *pb.RequestHeader) (uuid.UUID, error) {
	if header == nil {
		return uuid.UUID{}, rpcstatus.Error(rpcstatus.InvalidArgument, "header is nil")
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

	return uuid.UUID{}, nil
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

func (endpoint *Endpoint) tryUpdateBucketAttribution(ctx context.Context, header *pb.RequestHeader, projectID uuid.UUID, bucketName []byte, partnerID uuid.UUID) error {
	if header == nil {
		return rpcstatus.Error(rpcstatus.InvalidArgument, "header is nil")
	}

	// check if attribution is set for given bucket
	_, err := endpoint.attributions.Get(ctx, projectID, bucketName)
	if err == nil {
		// bucket has already an attribution, no need to update
		return nil
	}
	if !attribution.ErrBucketNotAttributed.Has(err) {
		// try only to set the attribution, when it's missing
		endpoint.log.Error("error while getting attribution from DB", zap.Error(err))
		return rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	empty, err := endpoint.metainfo.IsBucketEmpty(ctx, projectID, bucketName)
	if err != nil {
		return rpcstatus.Error(rpcstatus.Internal, err.Error())
	}
	if !empty {
		return rpcstatus.Errorf(rpcstatus.AlreadyExists, "bucket %q is not empty, PartnerID %q cannot be attributed", bucketName, partnerID)
	}

	// checks if bucket exists before updates it or makes a new entry
	bucket, err := endpoint.metainfo.GetBucket(ctx, bucketName, projectID)
	if err != nil {
		if storj.ErrBucketNotFound.Has(err) {
			return rpcstatus.Errorf(rpcstatus.NotFound, "bucket %q does not exist", bucketName)
		}
		endpoint.log.Error("error while getting bucket", zap.ByteString("bucketName", bucketName), zap.Error(err))
		return rpcstatus.Error(rpcstatus.Internal, "unable to set bucket attribution")
	}
	if !bucket.PartnerID.IsZero() {
		return rpcstatus.Errorf(rpcstatus.AlreadyExists, "bucket %q already has attribution, PartnerID %q cannot be attributed", bucketName, partnerID)
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
		ProjectID:  projectID,
		BucketName: bucketName,
		PartnerID:  partnerID,
	})
	if err != nil {
		endpoint.log.Error("error while inserting attribution to DB", zap.Error(err))
		return rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	return nil
}

// maxAttributionCacheSize determines how many buckets attributionCheckCache remembers.
const maxAttributionCacheSize = 10

// attributionCheckCacheKey is used as a key for the connection cache.
type attributionCheckCacheKey struct{}

// attributionCheckCache implements a basic lru cache, with a constant size.
type attributionCheckCache struct {
	mu      sync.Mutex
	pos     int
	buckets []string
}

// needsCheck returns true when the bucket should be tested for setting the useragent.
func (cache *attributionCheckCache) needsCheck(bucket string) bool {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	for _, b := range cache.buckets {
		if b == bucket {
			return false
		}
	}

	if len(cache.buckets) >= maxAttributionCacheSize {
		cache.pos = (cache.pos + 1) % len(cache.buckets)
		cache.buckets[cache.pos] = bucket
	} else {
		cache.pos = len(cache.buckets)
		cache.buckets = append(cache.buckets, bucket)
	}

	return true
}
