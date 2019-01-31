// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"bytes"
	"context"
	"fmt"
	"io"

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
	"storj.io/storj/pkg/utils"
	"storj.io/storj/satellite"
)

// Node is a general purpose
type Node struct {
	Log              *zap.Logger
	Info             pb.Node
	Identity         *identity.FullIdentity
	Transport        transport.Client
	storageNodeCount int
}

// newUplink creates a new uplink
func (planet *Planet) newUplink(name string) (*Node, error) {
	identity, err := planet.NewIdentity()
	if err != nil {
		return nil, err
	}

	node := &Node{
		Log:              planet.log.Named(name),
		Identity:         identity,
		storageNodeCount: len(planet.StorageNodes),
	}

	node.Log.Debug("id=" + identity.ID.String())

	node.Transport = transport.NewClient(identity)

	node.Info = pb.Node{
		Id:   node.Identity.ID,
		Type: pb.NodeType_UPLINK,
		Address: &pb.NodeAddress{
			Transport: pb.NodeTransport_TCP_TLS_GRPC,
			Address:   "",
		},
	}

	planet.nodes = append(planet.nodes, node)

	return node, nil
}

// ID returns node id
func (node *Node) ID() storj.NodeID { return node.Info.Id }

// Addr returns node address
func (node *Node) Addr() string { return node.Info.Address.Address }

// Local returns node info
func (node *Node) Local() pb.Node { return node.Info }

// Shutdown shuts down all node dependencies
func (node *Node) Shutdown() error { return nil }

// DialPointerDB dials destination with apikey and returns pointerdb Client
func (node *Node) DialPointerDB(destination Peer, apikey string) (pdbclient.Client, error) {
	// TODO: use node.Transport instead of pdbclient.NewClient
	/*
		conn, err := node.Transport.DialNode(context.Background(), &destination.Info)
		if err != nil {
			return nil, err
		}
		return piececlient.NewPSClient
	*/

	// TODO: handle disconnect
	return pdbclient.NewClient(node.Identity, destination.Addr(), apikey)
}

// DialOverlay dials destination and returns an overlay.Client
func (node *Node) DialOverlay(destination Peer) (overlay.Client, error) {
	info := destination.Local()
	conn, err := node.Transport.DialNode(context.Background(), &info, grpc.WithBlock())
	if err != nil {
		return nil, err
	}

	// TODO: handle disconnect
	return overlay.NewClientFrom(pb.NewOverlayClient(conn)), nil
}

// Upload test
func (node *Node) Upload(ctx context.Context, satellite *satellite.Peer, bucket string, path storj.Path, data []byte) error {
	metainfo, streams, err := node.getMetainfo(satellite)
	if err != nil {
		return err
	}

	_, err = metainfo.GetBucket(ctx, bucket)
	if err != nil {
		if storj.ErrBucketNotFound.Has(err) {
			_, err := metainfo.CreateBucket(ctx, bucket, &storj.Bucket{PathCipher: storj.Cipher(1)})
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	createInfo := storj.CreateObject{
		RedundancyScheme: node.getRedundancyScheme(),
		EncryptionScheme: node.getEncryptionScheme(),
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

	return utils.CombineErrors(err, upload.Close())
}

// Download test
func (node *Node) Download(ctx context.Context, satellite *satellite.Peer, bucket string, path storj.Path) ([]byte, error) {

	metainfo, streams, err := node.getMetainfo(satellite)
	if err != nil {
		return []byte{}, err
	}

	readOnlyStream, err := metainfo.GetObjectStream(ctx, bucket, path)
	if err != nil {
		return []byte{}, err
	}

	download := stream.NewDownload(ctx, readOnlyStream, streams)
	defer func() { err = errs.Combine(err, download.Close()) }()

	buffer := bytes.NewBuffer([]byte{})
	_, err = io.Copy(buffer, download)
	if err != nil {
		return []byte{}, err
	}
	return buffer.Bytes(), nil
}

func (node *Node) getMetainfo(satellite *satellite.Peer) (db storj.Metainfo, ss streams.Store, err error) {
	redundancyScheme := node.getRedundancyScheme()
	minThreshold := int(redundancyScheme.RequiredShares)
	repairThreshold := int(redundancyScheme.RepairShares)
	successThreshold := int(redundancyScheme.OptimalShares)
	maxThreshold := int(redundancyScheme.TotalShares)

	maxBufferMem := int(4 * memory.MB)
	erasureShareSize := int(1 * memory.KB)
	blockSize := int(1 * memory.KB)
	maxInlineSize := int(4 * memory.KB)
	segmentSize := int64(64 * memory.MB)

	oc, err := node.DialOverlay(satellite)
	if err != nil {
		return nil, nil, err
	}

	pdb, err := node.DialPointerDB(satellite, "")
	if err != nil {
		return nil, nil, err
	}

	ec := ecclient.NewClient(node.Identity, maxBufferMem)
	fc, err := infectious.NewFEC(minThreshold, maxThreshold)
	if err != nil {
		return nil, nil, err
	}

	rs, err := eestream.NewRedundancyStrategy(eestream.NewRSScheme(fc, erasureShareSize), repairThreshold, successThreshold)
	if err != nil {
		return nil, nil, err
	}

	segments := segments.NewSegmentStore(oc, ec, pdb, rs, maxInlineSize)

	if erasureShareSize*minThreshold%blockSize != 0 {
		return nil, nil, fmt.Errorf("EncryptionBlockSize must be a multiple of ErasureShareSize * RS MinThreshold")
	}

	key := new(storj.Key)
	copy(key[:], "enc.key")

	streams, err := streams.NewStreamStore(segments, segmentSize, key, blockSize, storj.Cipher(1))
	if err != nil {
		return nil, nil, err
	}

	buckets := buckets.NewStore(streams)

	return kvmetainfo.New(buckets, streams, segments, pdb, key), streams, nil
}

func (node *Node) getRedundancyScheme() storj.RedundancyScheme {
	return storj.RedundancyScheme{
		Algorithm:      storj.ReedSolomon,
		RequiredShares: int16(1 * node.storageNodeCount / 5),
		RepairShares:   int16(2 * node.storageNodeCount / 5),
		OptimalShares:  int16(3 * node.storageNodeCount / 5),
		TotalShares:    int16(4 * node.storageNodeCount / 5),
	}
}

func (node *Node) getEncryptionScheme() storj.EncryptionScheme {
	return storj.EncryptionScheme{
		Cipher:    storj.Cipher(1),
		BlockSize: int32(1 * memory.KiB),
	}
}
