// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/vivint/infectious"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/macaroon"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/rpc"
	"storj.io/storj/uplink/ecclient"
	"storj.io/storj/uplink/eestream"
	"storj.io/storj/uplink/metainfo"
	"storj.io/storj/uplink/storage/segments"
)

var (
	flagAPIKey        = flag.String("api-key", "", "api key")
	flagSatelliteAddr = flag.String("satellite-addr", "", "satellite address")
)

func main() {
	flag.Parse()
	err := run(context.Background())
	if err != nil {
		panic(err)
	}
}

func run(ctx context.Context) (err error) {
	apiKey, err := macaroon.ParseAPIKey(*flagAPIKey)
	if err != nil {
		return err
	}

	ident, err := identity.NewFullIdentity(ctx, identity.NewCAOptions{
		Difficulty:  9,
		Concurrency: 1,
	})
	if err != nil {
		return err
	}
	tlsOpts, err := tlsopts.NewOptions(ident, tlsopts.Config{
		PeerIDVersions: "0",
	}, nil)
	if err != nil {
		return err
	}

	dialer := rpc.NewDefaultDialer(tlsOpts)

	conn, err := dialer.DialAddressInsecureBestEffort(ctx, *flagSatelliteAddr)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, conn.Close()) }()
	client := conn.MetainfoClient()

	ec := ecclient.NewClient(zap.L(), dialer, int(32*memory.MB))

	return newDownloader(client, ec, apiKey).Audit(ctx)
}

type downloader struct {
	client pb.MetainfoClient
	ec     ecclient.Client
	apiKey *macaroon.APIKey
}

func newDownloader(client pb.MetainfoClient, ec ecclient.Client, apiKey *macaroon.APIKey) *downloader {
	return &downloader{client: client, ec: ec, apiKey: apiKey}
}

func (d *downloader) Audit(ctx context.Context) error {
	segments, err := d.list(ctx)
	if err != nil {
		return err
	}

	return d.sanityCheck(ctx, segments)
}

func (d *downloader) list(ctx context.Context) (map[string]map[string]bool, error) {
	// assume we can hold all of a project's segment paths in memory.
	// safe assumption for now but unlikely to be true after launch
	segments := map[string]map[string]bool{}

	cursor := ""
	for {
		resp, err := d.client.AdminSegmentAudit(ctx, &pb.ListSegmentsRequestOld{
			Header: &pb.RequestHeader{
				ApiKey: d.apiKey.SerializeRaw(),
			},
			StartAfter: []byte(cursor),
			Recursive:  true,
			Limit:      100,
		})
		if err != nil {
			return nil, err
		}

		for _, item := range resp.GetItems() {
			cursor := string(item.GetPath())
			parts := strings.SplitN(cursor, "/", 2)
			if len(parts) != 2 {
				return nil, errs.New("malformed segment path")
			}
			segment := parts[0]
			bucketpath := parts[1]

			if _, exists := segments[bucketpath]; !exists {
				segments[bucketpath] = map[string]bool{}
			}
			if _, exists := segments[bucketpath][segment]; exists {
				return nil, errs.New("segment for path listed twice")
			}
			segments[bucketpath][segment] = true

			parts = strings.SplitN(bucketpath, "/", 2)
			if len(parts) != 2 {
				return nil, errs.New("malformed segment path")
			}
			bucket := parts[0]
			path := parts[1]

			err = d.download(ctx, segment, bucket, path)
			if err != nil {
				return nil, err
			}
		}

		if !resp.GetMore() {
			break
		}
	}

	return segments, nil
}

func (d *downloader) sanityCheck(ctx context.Context, segments map[string]map[string]bool) error {
	// partial sanity check, make sure we're not obviously missing any segments.
	// TODO: it is impossible to make this a perfect sanity check in general because
	// initially last segment metadata encrypted the segment count, so audit tools
	// like this one couldn't actually determine how many segments an object has.
	// this still could be better though. more recently, the last segment metadata
	// did start including the segment count, so we could actually confirm that
	// there are no segments missing in those cases, if we loaded the pointers in
	// the original listing.
	for path, knownSegments := range segments {
		foundSegments := 0
		for {
			segment := fmt.Sprintf("s%d", foundSegments)
			if _, exists := knownSegments[segment]; !exists {
				break
			}
			foundSegments++
			delete(knownSegments, segment)
		}
		if _, exists := knownSegments["l"]; !exists {
			fmt.Printf("warning: path %q is missing the last segment!\n", path)
		} else {
			delete(knownSegments, "l")
		}
		for segment := range knownSegments {
			fmt.Printf("warning: path %q has noncontiguous segment %s\n", path, segment)
		}
	}
	return nil
}

func (d *downloader) download(ctx context.Context, segment, bucket, path string) (err error) {
	segmentIndex, err := parseSegment(segment)
	if err != nil {
		return err
	}

	fc, err := infectious.NewFEC(1, 1)
	if err != nil {
		return err
	}
	rstrat, err := eestream.NewRedundancyStrategy(eestream.NewRSScheme(fc, 1), 1, 1)
	if err != nil {
		return err
	}

	client := metainfo.New(d.client, d.apiKey)

	// TODO: aaaiieeeeeee this needs to be batched in with the SegmentStore Get.
	// We have two round trips to the Satellite here back to back.
	object, err := client.GetObject(ctx, metainfo.GetObjectParams{
		Bucket:        []byte(bucket),
		EncryptedPath: []byte(path),
	})
	if err != nil {
		return err
	}

	ranger, _, err := segments.NewSegmentStore(client, d.ec, rstrat, 0, 0).Get(ctx, object.StreamID, int32(segmentIndex), object.RedundancyScheme)
	if err != nil {
		return err
	}

	reader, err := ranger.Range(ctx, 0, ranger.Size())
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, reader.Close()) }()

	n, err := io.Copy(ioutil.Discard, reader)
	err = errs.Wrap(err)
	if err != nil {
		fmt.Printf("downloaded %d bytes (error: %v)\n", n, err)
	} else {
		fmt.Printf("downloaded %d bytes (success)\n", n)
	}
	return err
}

func parseSegment(segment string) (segmentIndex int, err error) {
	if segment == "l" {
		return -1, nil
	}
	if !strings.HasPrefix(segment, "s") {
		return 0, errs.New("invalid segment id %q", segment)
	}
	return strconv.Atoi(segment[1:])
}
