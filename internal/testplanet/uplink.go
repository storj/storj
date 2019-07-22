// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"strconv"
	"time"

	"github.com/spf13/pflag"
	"github.com/vivint/infectious"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/macaroon"
	"storj.io/storj/pkg/metainfo/kvmetainfo"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/tlsopts"
	ecclient "storj.io/storj/pkg/storage/ec"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/stream"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/uplink"
	"storj.io/storj/uplink/metainfo"
	"storj.io/storj/uplink/piecestore"
	"storj.io/storj/uplink/setup"
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

// newUplinks creates initializes uplinks, requires peer to have at least one satellite
func (planet *Planet) newUplinks(prefix string, count, storageNodeCount int) ([]*Uplink, error) {
	var xs []*Uplink
	for i := 0; i < count; i++ {
		uplink, err := planet.newUplink(prefix+strconv.Itoa(i), storageNodeCount)
		if err != nil {
			return nil, err
		}
		xs = append(xs, uplink)
	}

	return xs, nil
}

// newUplink creates a new uplink
func (planet *Planet) newUplink(name string, storageNodeCount int) (*Uplink, error) {
	identity, err := planet.NewIdentity()
	if err != nil {
		return nil, err
	}

	tlsOpts, err := tlsopts.NewOptions(identity, tlsopts.Config{
		PeerIDVersions: strconv.Itoa(int(planet.config.IdentityVersion.Number)),
	})
	if err != nil {
		return nil, err
	}

	uplink := &Uplink{
		Log:              planet.log.Named(name),
		Identity:         identity,
		StorageNodeCount: storageNodeCount,
	}

	uplink.Log.Debug("id=" + identity.ID.String())

	uplink.Transport = transport.NewClient(tlsOpts)

	uplink.Info = pb.Node{
		Id: uplink.Identity.ID,
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
		key, err := macaroon.NewAPIKey([]byte("testSecret"))
		if err != nil {
			return nil, err
		}

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
			key.Head(),
			console.APIKeyInfo{
				Name:      "root",
				ProjectID: project.ID,
				Secret:    []byte("testSecret"),
			},
		)
		if err != nil {
			return nil, err
		}

		apiKeys[satellite.ID()] = key.Serialize()
	}

	uplink.APIKey = apiKeys
	planet.uplinks = append(planet.uplinks, uplink)

	return uplink, nil
}

// ID returns uplink id
func (planetUplink *Uplink) ID() storj.NodeID { return planetUplink.Info.Id }

// Addr returns uplink address
func (planetUplink *Uplink) Addr() string { return planetUplink.Info.Address.Address }

// Local returns uplink info
func (planetUplink *Uplink) Local() pb.Node { return planetUplink.Info }

// Shutdown shuts down all uplink dependencies
func (planetUplink *Uplink) Shutdown() error { return nil }

// DialMetainfo dials destination with apikey and returns metainfo Client
func (planetUplink *Uplink) DialMetainfo(ctx context.Context, destination Peer, apikey string) (*metainfo.Client, error) {
	return metainfo.Dial(ctx, planetUplink.Transport, destination.Addr(), apikey)
}

// DialPiecestore dials destination storagenode and returns a piecestore client.
func (planetUplink *Uplink) DialPiecestore(ctx context.Context, destination Peer) (*piecestore.Client, error) {
	node := destination.Local()
	return piecestore.Dial(ctx, planetUplink.Transport, &node.Node, planetUplink.Log.Named("uplink>piecestore"), piecestore.DefaultConfig)
}

// Upload data to specific satellite
func (planetUplink *Uplink) Upload(ctx context.Context, satellite *satellite.Peer, bucket string, path storj.Path, data []byte) error {
	return planetUplink.UploadWithExpiration(ctx, satellite, bucket, path, data, time.Time{})
}

// UploadWithExpiration data to specific satellite and expiration time
func (planetUplink *Uplink) UploadWithExpiration(ctx context.Context, satellite *satellite.Peer, bucket string, path storj.Path, data []byte, expiration time.Time) error {
	return planetUplink.UploadWithExpirationAndConfig(ctx, satellite, nil, bucket, path, data, expiration)
}

// UploadWithConfig uploads data to specific satellite with configured values
func (planetUplink *Uplink) UploadWithConfig(ctx context.Context, satellite *satellite.Peer, redundancy *uplink.RSConfig, bucket string, path storj.Path, data []byte) error {
	return planetUplink.UploadWithExpirationAndConfig(ctx, satellite, redundancy, bucket, path, data, time.Time{})
}

// UploadWithExpirationAndConfig uploads data to specific satellite with configured values and expiration time
func (planetUplink *Uplink) UploadWithExpirationAndConfig(ctx context.Context, satellite *satellite.Peer, redundancy *uplink.RSConfig, bucketName string, path storj.Path, data []byte, expiration time.Time) (err error) {
	config := planetUplink.GetConfig(satellite)
	if redundancy != nil {
		if redundancy.MinThreshold > 0 {
			config.RS.MinThreshold = redundancy.MinThreshold
		}
		if redundancy.RepairThreshold > 0 {
			config.RS.RepairThreshold = redundancy.RepairThreshold
		}
		if redundancy.SuccessThreshold > 0 {
			config.RS.SuccessThreshold = redundancy.SuccessThreshold
		}
		if redundancy.MaxThreshold > 0 {
			config.RS.MaxThreshold = redundancy.MaxThreshold
		}
		if redundancy.ErasureShareSize > 0 {
			config.RS.ErasureShareSize = redundancy.ErasureShareSize
		}
	}

	project, err := planetUplink.GetProject(ctx, satellite)
	defer func() {
		err = errs.Combine(err,
			project.Close(),
		)
	}()

	access, err := setup.LoadEncryptionAccess(ctx, config.Enc)
	if err != nil {
		return err
	}

	// Check if the bucket exists, if not then create it
	_, _, err = project.GetBucketInfo(ctx, bucketName)
	if err != nil {
		if !storj.ErrBucketNotFound.Has(err) {
			err := createBucket(ctx, config, *project, bucketName)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	bucket, err := project.OpenBucket(ctx, bucketName, access)
	if err != nil {
		return err
	}
	defer func() {
		err = errs.Combine(err,
			bucket.Close(),
		)
	}()

	opts := &libuplink.UploadOptions{}
	opts.Volatile.RedundancyScheme = config.GetRedundancyScheme()
	opts.Volatile.EncryptionParameters = config.GetEncryptionParameters()

	reader := bytes.NewReader(data)
	if err := bucket.UploadObject(ctx, path, reader, opts); err != nil {
		return err
	}

	return nil
}

// DownloadStream returns stream for downloading data.
func (planetUplink *Uplink) DownloadStream(ctx context.Context, satellite *satellite.Peer, bucket string, path storj.Path) (*stream.Download, func() error, error) {
	config := planetUplink.GetConfig(satellite)
	metainfo, streams, cleanup, err := DialMetainfo(ctx, planetUplink.Log.Named("metainfo"), config, planetUplink.Identity)
	if err != nil {
		return nil, func() error { return nil }, errs.Combine(err, cleanup())
	}

	readOnlyStream, err := metainfo.GetObjectStream(ctx, bucket, path)
	if err != nil {
		return nil, func() error { return nil }, errs.Combine(err, cleanup())
	}

	return stream.NewDownload(ctx, readOnlyStream, streams), cleanup, nil
}

// Download data from specific satellite
func (planetUplink *Uplink) Download(ctx context.Context, satellite *satellite.Peer, bucket string, path storj.Path) ([]byte, error) {
	download, cleanup, err := planetUplink.DownloadStream(ctx, satellite, bucket, path)
	if err != nil {
		return []byte{}, err
	}
	defer func() {
		err = errs.Combine(err,
			download.Close(),
			cleanup(),
		)
	}()

	data, err := ioutil.ReadAll(download)
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

// Delete data to specific satellite
func (planetUplink *Uplink) Delete(ctx context.Context, satellite *satellite.Peer, bucket string, path storj.Path) error {
	config := planetUplink.GetConfig(satellite)
	metainfo, _, cleanup, err := DialMetainfo(ctx, planetUplink.Log.Named("metainfo"), config, planetUplink.Identity)
	if err != nil {
		return err
	}
	return errs.Combine(
		metainfo.DeleteObject(ctx, bucket, path),
		cleanup(),
	)
}

// GetConfig returns a default config for a given satellite.
func (planetUplink *Uplink) GetConfig(satellite *satellite.Peer) uplink.Config {
	config := getDefaultConfig()
	config.Client.SatelliteAddr = satellite.Addr()
	config.Client.APIKey = planetUplink.APIKey[satellite.ID()]
	config.Client.RequestTimeout = 10 * time.Second
	config.Client.DialTimeout = 10 * time.Second

	config.RS.MinThreshold = atLeastOne(planetUplink.StorageNodeCount * 1 / 5)     // 20% of storage nodes
	config.RS.RepairThreshold = atLeastOne(planetUplink.StorageNodeCount * 2 / 5)  // 40% of storage nodes
	config.RS.SuccessThreshold = atLeastOne(planetUplink.StorageNodeCount * 3 / 5) // 60% of storage nodes
	config.RS.MaxThreshold = atLeastOne(planetUplink.StorageNodeCount * 4 / 5)     // 80% of storage nodes

	config.TLS.UsePeerCAWhitelist = false
	config.TLS.Extensions.Revocation = false
	config.TLS.Extensions.WhitelistSignedLeaf = false

	return config
}

// NewLibuplink creates a libuplink.Uplink object with the testplanet Uplink config
func (planetUplink *Uplink) NewLibuplink(ctx context.Context) (*libuplink.Uplink, error) {
	config := getDefaultConfig()
	libuplinkCfg := &libuplink.Config{}
	libuplinkCfg.Volatile.MaxInlineSize = config.Client.MaxInlineSize
	libuplinkCfg.Volatile.MaxMemory = config.RS.MaxBufferMem
	libuplinkCfg.Volatile.PeerIDVersion = config.TLS.PeerIDVersions
	libuplinkCfg.Volatile.TLS = struct {
		SkipPeerCAWhitelist bool
		PeerCAWhitelistPath string
	}{
		SkipPeerCAWhitelist: !config.TLS.UsePeerCAWhitelist,
		PeerCAWhitelistPath: config.TLS.PeerCAWhitelistPath,
	}

	libuplinkCfg.Volatile.DialTimeout = config.Client.DialTimeout
	libuplinkCfg.Volatile.RequestTimeout = config.Client.RequestTimeout

	return libuplink.NewUplink(ctx, libuplinkCfg)
}

func getDefaultConfig() uplink.Config {
	config := uplink.Config{}
	cfgstruct.Bind(&pflag.FlagSet{}, &config, cfgstruct.UseDevDefaults())
	return config
}

// atLeastOne returns 1 if value < 1, or value otherwise.
func atLeastOne(value int) int {
	if value < 1 {
		return 1
	}
	return value
}

// DialMetainfo returns a metainfo and streams store for the given configuration and identity.
func DialMetainfo(ctx context.Context, log *zap.Logger, config uplink.Config, identity *identity.FullIdentity) (db storj.Metainfo, ss streams.Store, cleanup func() error, err error) {
	tlsOpts, err := tlsopts.NewOptions(identity, config.TLS)
	if err != nil {
		return nil, nil, cleanup, err
	}

	// ToDo: Handle Versioning for Uplinks here

	tc := transport.NewClientWithTimeouts(tlsOpts, transport.Timeouts{
		Request: config.Client.RequestTimeout,
		Dial:    config.Client.DialTimeout,
	})

	if config.Client.SatelliteAddr == "" {
		return nil, nil, cleanup, errs.New("satellite address not specified")
	}

	m, err := metainfo.Dial(ctx, tc, config.Client.SatelliteAddr, config.Client.APIKey)
	if err != nil {
		return nil, nil, cleanup, errs.New("failed to connect to metainfo service: %v", err)
	}
	defer func() {
		if err != nil {
			// close metainfo if any of the setup fails
			err = errs.Combine(err, m.Close())
		}
	}()

	project, err := kvmetainfo.SetupProject(m)
	if err != nil {
		return nil, nil, cleanup, errs.New("failed to create project: %v", err)
	}

	ec := ecclient.NewClient(log.Named("ecclient"), tc, config.RS.MaxBufferMem.Int())
	fc, err := infectious.NewFEC(config.RS.MinThreshold, config.RS.MaxThreshold)
	if err != nil {
		return nil, nil, cleanup, errs.New("failed to create erasure coding client: %v", err)
	}
	rs, err := eestream.NewRedundancyStrategy(eestream.NewRSScheme(fc, config.RS.ErasureShareSize.Int()), config.RS.RepairThreshold, config.RS.SuccessThreshold)
	if err != nil {
		return nil, nil, cleanup, errs.New("failed to create redundancy strategy: %v", err)
	}

	maxEncryptedSegmentSize, err := encryption.CalcEncryptedSize(config.Client.SegmentSize.Int64(), config.GetEncryptionParameters())
	if err != nil {
		return nil, nil, cleanup, errs.New("failed to calculate max encrypted segment size: %v", err)
	}
	segment := segments.NewSegmentStore(m, ec, rs, config.Client.MaxInlineSize.Int(), maxEncryptedSegmentSize)

	blockSize := config.GetEncryptionParameters().BlockSize
	if int(blockSize)%config.RS.ErasureShareSize.Int()*config.RS.MinThreshold != 0 {
		err = errs.New("EncryptionBlockSize must be a multiple of ErasureShareSize * RS MinThreshold")
		return nil, nil, cleanup, err
	}

	// TODO(jeff): there's some cycles with libuplink and this package in the libuplink tests
	// and so this package can't import libuplink. that's why this function is duplicated
	// in some spots.

	encStore := encryption.NewStore()
	encStore.SetDefaultKey(new(storj.Key))

	strms, err := streams.NewStreamStore(segment, config.Client.SegmentSize.Int64(), encStore,
		int(blockSize), storj.CipherSuite(config.Enc.DataType), config.Client.MaxInlineSize.Int(),
	)
	if err != nil {
		return nil, nil, cleanup, errs.New("failed to create stream store: %v", err)
	}

	return kvmetainfo.New(project, m, strms, segment, encStore), strms, m.Close, nil
}

// GetProject returns a libuplink.Project which allows interactions with a specific project
func (planetUplink *Uplink) GetProject(ctx context.Context, satellite *satellite.Peer) (*libuplink.Project, error) {
	testLibuplink, err := planetUplink.NewLibuplink(ctx)
	if err != nil {
		return nil, err
	}
	planetUplinkAPIKey := planetUplink.APIKey[satellite.ID()]
	key, err := libuplink.ParseAPIKey(planetUplinkAPIKey)

	project, err := testLibuplink.OpenProject(ctx, satellite.Addr(), key)
	if err != nil {
		if err := testLibuplink.Close(); err != nil {
			fmt.Printf("error closing uplink: %+v\n", err)
		}
	}
	return project, nil
}

func createBucket(ctx context.Context, config uplink.Config, project libuplink.Project, bucketName string) error {
	bucketCfg := &libuplink.BucketConfig{}
	bucketCfg.PathCipher = config.GetPathCipherSuite()
	bucketCfg.EncryptionParameters = config.GetEncryptionParameters()
	bucketCfg.Volatile = struct {
		RedundancyScheme storj.RedundancyScheme
		SegmentsSize     memory.Size
	}{
		RedundancyScheme: config.GetRedundancyScheme(),
		SegmentsSize:     config.GetSegmentSize(),
	}

	_, err := project.CreateBucket(ctx, bucketName, bucketCfg)
	if err != nil {
		return err
	}
	return nil
}
