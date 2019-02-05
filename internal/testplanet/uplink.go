// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/vivint/infectious"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/metainfo/kvmetainfo"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/storage/buckets"
	ecclient "storj.io/storj/pkg/storage/ec"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/stream"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
)

// Uplink is a general purpose
type Uplink struct {
	Log              *zap.Logger
	Info             pb.Node
	Identity         *identity.FullIdentity
	Transport        transport.Client
	StorageNodeCount int
	APIKey           map[storj.NodeID]string
}

// newUplink creates a new uplink
func (planet *Planet) newUplink(name string, storageNodeCount int) (*Uplink, error) {
	identity, err := planet.NewIdentity()
	if err != nil {
		return nil, err
	}

	uplink := &Uplink{
		Log:              planet.log.Named(name),
		Identity:         identity,
		StorageNodeCount: storageNodeCount,
	}

	uplink.Log.Debug("id=" + identity.ID.String())

	uplink.Transport = transport.NewClient(identity)

	uplink.Info = pb.Node{
		Id:   uplink.Identity.ID,
		Type: pb.NodeType_UPLINK,
		Address: &pb.NodeAddress{
			Transport: pb.NodeTransport_TCP_TLS_GRPC,
			Address:   "",
		},
	}

	apiKeys := make(map[storj.NodeID]string)
	for j, satellite := range planet.Satellites {
		// TODO: find a nicer way to do this
		// populate satellites console with example
		// project and API key and pass that to uplinks
		consoleDB := satellite.DB.Console()

		projectName := fmt.Sprintf("%s_%d", name, j)
		key := console.APIKeyFromBytes([]byte(projectName))

		project, err := consoleDB.Projects().Insert(
			context.Background(),
			&console.Project{
				Name: projectName,
			},
		)
		if err != nil {
			return nil, err
		}

		_, err = consoleDB.APIKeys().Create(
			context.Background(),
			*key,
			console.APIKeyInfo{
				Name:      "root",
				ProjectID: project.ID,
			},
		)
		if err != nil {
			return nil, err
		}

		apiKeys[satellite.ID()] = key.String()
	}

	uplink.APIKey = apiKeys
	planet.uplinks = append(planet.uplinks, uplink)

	return uplink, nil
}

// ID returns uplink id
func (uplink *Uplink) ID() storj.NodeID { return uplink.Info.Id }

// Addr returns uplink address
func (uplink *Uplink) Addr() string { return uplink.Info.Address.Address }

// Local returns uplink info
func (uplink *Uplink) Local() pb.Node { return uplink.Info }

// Shutdown shuts down all uplink dependencies
func (uplink *Uplink) Shutdown() error { return nil }

// DialPointerDB dials destination with apikey and returns pointerdb Client
func (uplink *Uplink) DialPointerDB(destination Peer, apikey string) (pdbclient.Client, error) {
	// TODO: use node.Transport instead of pdbclient.NewClient
	/*
		conn, err := node.Transport.DialNode(context.Background(), &destination.Info)
		if err != nil {
			return nil, err
		}
		return piececlient.NewPSClient
	*/

	// TODO: handle disconnect
	return pdbclient.NewClient(uplink.Identity, destination.Addr(), apikey)
}

// DialOverlay dials destination and returns an overlay.Client
func (uplink *Uplink) DialOverlay(destination Peer) (overlay.Client, error) {
	info := destination.Local()
	conn, err := uplink.Transport.DialNode(context.Background(), &info, grpc.WithBlock())
	if err != nil {
		return nil, err
	}

	// TODO: handle disconnect
	return overlay.NewClientFrom(pb.NewOverlayClient(conn)), nil
}

// Upload data to specific satellite
func (uplink *Uplink) Upload(ctx context.Context, satellite *satellite.Peer, bucket string, path storj.Path, data []byte) error {
	metainfo, streams, err := uplink.getMetainfo(satellite)
	if err != nil {
		return err
	}

	encScheme := uplink.getEncryptionScheme()
	redScheme := uplink.getRedundancyScheme()

	// create bucket if not exists
	_, err = metainfo.GetBucket(ctx, bucket)
	if err != nil {
		if storj.ErrBucketNotFound.Has(err) {
			_, err := metainfo.CreateBucket(ctx, bucket, &storj.Bucket{PathCipher: encScheme.Cipher})
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	createInfo := storj.CreateObject{
		RedundancyScheme: redScheme,
		EncryptionScheme: encScheme,
	}
	obj, err := metainfo.CreateObject(ctx, bucket, path, &createInfo)
	if err != nil {
		return err
	}

	reader := bytes.NewReader(data)
	err = uploadStream(ctx, streams, obj, reader)
	if err != nil {
		return err
	}

	return nil
}

func uploadStream(ctx context.Context, streams streams.Store, mutableObject storj.MutableObject, reader io.Reader) error {
	mutableStream, err := mutableObject.CreateStream(ctx)
	if err != nil {
		return err
	}

	upload := stream.NewUpload(ctx, mutableStream, streams)

	_, err = io.Copy(upload, reader)

	return errs.Combine(err, upload.Close())
}

// Download data from specific satellite
func (uplink *Uplink) Download(ctx context.Context, satellite *satellite.Peer, bucket string, path storj.Path) ([]byte, error) {
	metainfo, streams, err := uplink.getMetainfo(satellite)
	if err != nil {
		return []byte{}, err
	}

	readOnlyStream, err := metainfo.GetObjectStream(ctx, bucket, path)
	if err != nil {
		return []byte{}, err
	}

	download := stream.NewDownload(ctx, readOnlyStream, streams)
	defer func() { err = errs.Combine(err, download.Close()) }()

	data, err := ioutil.ReadAll(download)
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

func (uplink *Uplink) getMetainfo(satellite *satellite.Peer) (db storj.Metainfo, ss streams.Store, err error) {
	encScheme := uplink.getEncryptionScheme()
	redScheme := uplink.getRedundancyScheme()

	// redundancy settings
	minThreshold := int(redScheme.RequiredShares)
	repairThreshold := int(redScheme.RepairShares)
	successThreshold := int(redScheme.OptimalShares)
	maxThreshold := int(redScheme.TotalShares)
	erasureShareSize := 1 * memory.KiB
	maxBufferMem := 4 * memory.MiB

	// client settings
	maxInlineSize := 4 * memory.KiB
	segmentSize := 64 * memory.MiB

	oc, err := uplink.DialOverlay(satellite)
	if err != nil {
		return nil, nil, err
	}

	pdb, err := uplink.DialPointerDB(satellite, uplink.APIKey[satellite.ID()]) // TODO pass api key?
	if err != nil {
		return nil, nil, err
	}

	ec := ecclient.NewClient(uplink.Identity, maxBufferMem.Int())
	fc, err := infectious.NewFEC(minThreshold, maxThreshold)
	if err != nil {
		return nil, nil, err
	}

	rs, err := eestream.NewRedundancyStrategy(eestream.NewRSScheme(fc, erasureShareSize.Int()), repairThreshold, successThreshold)
	if err != nil {
		return nil, nil, err
	}

	segments := segments.NewSegmentStore(oc, ec, pdb, rs, maxInlineSize.Int())

	if erasureShareSize.Int()*minThreshold%int(encScheme.BlockSize) != 0 {
		return nil, nil, fmt.Errorf("EncryptionBlockSize must be a multiple of ErasureShareSize * RS MinThreshold")
	}

	key := new(storj.Key)
	copy(key[:], "enc.key")

	streams, err := streams.NewStreamStore(segments, segmentSize.Int64(), key, int(encScheme.BlockSize), encScheme.Cipher)
	if err != nil {
		return nil, nil, err
	}

	buckets := buckets.NewStore(streams)

	return kvmetainfo.New(buckets, streams, segments, pdb, key), streams, nil
}

func (uplink *Uplink) getRedundancyScheme() storj.RedundancyScheme {
	return storj.RedundancyScheme{
		Algorithm:      storj.ReedSolomon,
		RequiredShares: int16(1 * uplink.StorageNodeCount / 5),
		RepairShares:   int16(2 * uplink.StorageNodeCount / 5),
		OptimalShares:  int16(3 * uplink.StorageNodeCount / 5),
		TotalShares:    int16(4 * uplink.StorageNodeCount / 5),
	}
}

func (uplink *Uplink) getEncryptionScheme() storj.EncryptionScheme {
	return storj.EncryptionScheme{
		Cipher:    storj.AESGCM,
		BlockSize: 1 * memory.KiB.Int32(),
	}
}
