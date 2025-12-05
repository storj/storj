// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"bytes"
	"context"
	"sync"

	"storj.io/common/errs2"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/common/useragent"
	"storj.io/common/uuid"
	"storj.io/drpc/drpccache"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
)

// MaxUserAgentLength is the maximum allowable length of the User Agent.
const MaxUserAgentLength = 500

// ensureAttribution ensures that the bucketName has the partner information specified by project-level user agent, or header user agent.
// If `forceBucketUpdate` is true, then the buckets table will be updated if necessary (needed for bucket creation). Otherwise, it is sufficient
// to only ensure the attribution exists in the value attributions db.
//
// Assumes that the user has permissions sufficient for authenticating.
func (endpoint *Endpoint) ensureAttribution(ctx context.Context, header *pb.RequestHeader, keyInfo *console.APIKeyInfo, bucketName, projectUserAgent []byte, placement storj.PlacementConstraint, validatePlacement, forceBucketUpdate bool) (err error) {
	defer mon.Task()(&ctx)(&err)

	if header == nil {
		return rpcstatus.Error(rpcstatus.InvalidArgument, "header is nil")
	}

	if !forceBucketUpdate {
		if conncache := drpccache.FromContext(ctx); conncache != nil {
			cache := conncache.LoadOrCreate(attributionCheckCacheKey{},
				func() interface{} {
					return &attributionCheckCache{}
				}).(*attributionCheckCache)
			if !cache.needsCheck(string(bucketName)) {
				return nil
			}
		}
	}

	userAgent := keyInfo.UserAgent
	if len(projectUserAgent) > 0 {
		userAgent = projectUserAgent
	}

	// first check keyInfo (user) attribution
	if userAgent == nil {
		// otherwise, use header (partner tool) as attribution
		userAgent = header.UserAgent
	}

	userAgent, err = TrimUserAgent(userAgent)
	if err != nil {
		return err
	}

	err = endpoint.tryUpdateBucketAttribution(ctx, header, keyInfo.ProjectID, bucketName, userAgent, placement, validatePlacement, forceBucketUpdate)
	if errs2.IsRPC(err, rpcstatus.NotFound) || errs2.IsRPC(err, rpcstatus.AlreadyExists) {
		return nil
	}
	return err
}

// ensureAttributionOnBucketDelete makes sure there’s an attribution record after deleting the bucket.
func (endpoint *Endpoint) ensureAttributionOnBucketDelete(ctx context.Context, bucket buckets.Bucket) (err error) {
	defer mon.Task()(&ctx)(&err)

	nameBytes := []byte(bucket.Name)

	if _, err = endpoint.attributions.Get(ctx, bucket.ProjectID, nameBytes); err == nil {
		// Already attributed, nothing to do
		return nil
	}

	if !attribution.ErrBucketNotAttributed.Has(err) {
		return endpoint.ConvertKnownErrWithMessage(err, "unable to get bucket attribution")
	}

	info := &attribution.Info{
		ProjectID:  bucket.ProjectID,
		BucketName: nameBytes,
		UserAgent:  bucket.UserAgent,
		Placement:  &bucket.Placement,
	}
	if _, err = endpoint.attributions.Insert(ctx, info); err != nil {
		return endpoint.ConvertKnownErrWithMessage(err, "unable to set bucket attribution")
	}

	return nil
}

// TrimUserAgent returns userAgentBytes that consist of only the product portion of the user agent, and is bounded by
// the maxUserAgentLength.
func TrimUserAgent(userAgent []byte) ([]byte, error) {
	if len(userAgent) == 0 {
		return userAgent, nil
	}
	userAgentEntries, err := useragent.ParseEntries(userAgent)
	if err != nil {
		return userAgent, Error.New("error while parsing user agent: %w", err)
	}
	// strip comments, libraries, and empty products from the user agent
	newEntries := userAgentEntries[:0]
	for _, e := range userAgentEntries {
		switch product := e.Product; product {
		case "uplink", "common", "drpc", "Gateway-ST", "":
		default:
			e.Comment = ""
			newEntries = append(newEntries, e)
		}
	}
	userAgent, err = useragent.EncodeEntries(newEntries)
	if err != nil {
		return userAgent, Error.New("error while encoding user agent entries: %w", err)
	}

	// bound the user agent length
	if len(userAgent) > MaxUserAgentLength && len(newEntries) > 0 {
		// try to preserve the first entry
		if (len(newEntries[0].Product) + len(newEntries[0].Version)) <= MaxUserAgentLength {
			userAgent, err = useragent.EncodeEntries(newEntries[:1])
			if err != nil {
				return userAgent, Error.New("error while encoding first user agent entry: %w", err)
			}
		} else {
			// first entry is too large, truncate
			userAgent = userAgent[:MaxUserAgentLength]
		}
	}
	return userAgent, nil
}

func (endpoint *Endpoint) tryUpdateBucketAttribution(ctx context.Context, header *pb.RequestHeader, projectID uuid.UUID, bucketName []byte, userAgent []byte, placement storj.PlacementConstraint, validatePlacement, forceBucketUpdate bool) (err error) {
	defer mon.Task()(&ctx)(&err)

	if header == nil {
		return rpcstatus.Error(rpcstatus.InvalidArgument, "header is nil")
	}

	if !forceBucketUpdate && len(userAgent) == 0 {
		// no user agent, nothing to do
		return nil
	}

	// check if attribution is set for given bucket
	attrInfo, err := endpoint.attributions.Get(ctx, projectID, bucketName)
	if err == nil {
		if validatePlacement && attrInfo.Placement != nil && *attrInfo.Placement != placement {
			return rpcstatus.Errorf(rpcstatus.FailedPrecondition, "bucket %q already attributed to a different placement constraint", bucketName)
		}

		if !forceBucketUpdate {
			if attrInfo.UserAgent == nil {
				// set user agent if not set
				err = endpoint.attributions.UpdateUserAgent(ctx, projectID, string(bucketName), userAgent)
				if err != nil {
					return endpoint.ConvertKnownErrWithMessage(err, "unable to set bucket attribution")
				}
			}
			return nil
		}
	} else if !attribution.ErrBucketNotAttributed.Has(err) {
		return endpoint.ConvertKnownErrWithMessage(err, "unable to get bucket attribution")
	}

	// checks if bucket exists before updates it or makes a new entry
	bucket, err := endpoint.buckets.GetBucket(ctx, bucketName, projectID)
	if err != nil {
		if buckets.ErrBucketNotFound.Has(err) {
			return rpcstatus.Errorf(rpcstatus.NotFound, "bucket %q does not exist", bucketName)
		}
		return endpoint.ConvertKnownErrWithMessage(err, "error while getting bucket")
	}

	if attrInfo != nil {
		// bucket user agent and value attributions user agent already set
		if bytes.Equal(bucket.UserAgent, attrInfo.UserAgent) {
			return nil
		}
		// make sure bucket user_agent matches value_attribution
		userAgent = attrInfo.UserAgent
	}

	empty, err := endpoint.isBucketEmpty(ctx, projectID, bucketName)
	if err != nil {
		return endpoint.ConvertKnownErr(err)
	}
	if !empty {
		return rpcstatus.Errorf(rpcstatus.AlreadyExists, "bucket %q is not empty, Partner %q cannot be attributed", bucketName, userAgent)
	}

	if attrInfo == nil {
		// update attribution table
		_, err = endpoint.attributions.Insert(ctx, &attribution.Info{
			ProjectID:  projectID,
			BucketName: bucketName,
			UserAgent:  userAgent,
			Placement:  &placement,
		})
		if err != nil {
			return endpoint.ConvertKnownErrWithMessage(err, "unable to set bucket attribution")
		}
	}

	// update bucket information
	bucket.UserAgent = userAgent
	_, err = endpoint.buckets.UpdateBucket(ctx, bucket)
	if err != nil {
		return endpoint.ConvertKnownErrWithMessage(err, "unable to set bucket attribution")
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
