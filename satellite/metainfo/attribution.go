// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"strings"

	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/useragent"
	"storj.io/common/uuid"
	"storj.io/storj/private/dbutil"
)

// ResolvePartnerID returns partnerIDBytes as parsed or UUID corresponding to header.UserAgent.
// returns empty uuid when neither is defined.
func (endpoint *Endpoint) ResolvePartnerID(ctx context.Context, header *pb.RequestHeader, partnerIDBytes []byte) (uuid.UUID, error) {
	if header == nil {
		return uuid.UUID{}, rpcstatus.Error(rpcstatus.InvalidArgument, "header is nil")
	}

	if len(partnerIDBytes) > 0 {
		partnerID, err := dbutil.BytesToUUID(partnerIDBytes)
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
			if err != nil || partner.UUID == nil {
				continue
			}

			return *partner.UUID, nil
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
