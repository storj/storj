// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/spf13/pflag"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/identity"
	"storj.io/common/macaroon"
	"storj.io/common/pb"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/satellite/console"
	"storj.io/storj/uplink"
	"storj.io/storj/uplink/metainfo"
	"storj.io/storj/uplink/piecestore"
)

// Uplink is a general purpose
type Uplink struct {
	Log              *zap.Logger
	Info             pb.Node
	Identity         *identity.FullIdentity
	Dialer           rpc.Dialer
	StorageNodeCount int

	APIKey    map[storj.NodeID]*macaroon.APIKey
	ProjectID map[storj.NodeID]uuid.UUID
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

	tlsOptions, err := tlsopts.NewOptions(identity, tlsopts.Config{
		PeerIDVersions: strconv.Itoa(int(planet.config.IdentityVersion.Number)),
	}, nil)
	if err != nil {
		return nil, err
	}

	uplink := &Uplink{
		Log:              planet.log.Named(name),
		Identity:         identity,
		StorageNodeCount: storageNodeCount,
		APIKey:           map[storj.NodeID]*macaroon.APIKey{},
		ProjectID:        map[storj.NodeID]uuid.UUID{},
	}

	uplink.Log.Debug("id=" + identity.ID.String())

	uplink.Dialer = rpc.NewDefaultDialer(tlsOptions)

	uplink.Info = pb.Node{
		Id: uplink.Identity.ID,
		Address: &pb.NodeAddress{
			Transport: pb.NodeTransport_TCP_TLS_GRPC,
			Address:   "",
		},
	}

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

		uplink.APIKey[satellite.ID()] = key
		uplink.ProjectID[satellite.ID()] = project.ID
	}

	planet.uplinks = append(planet.uplinks, uplink)

	return uplink, nil
}

// ID returns uplink id
func (client *Uplink) ID() storj.NodeID { return client.Info.Id }

// Addr returns uplink address
func (client *Uplink) Addr() string { return client.Info.Address.Address }

// Local returns uplink info
func (client *Uplink) Local() pb.Node { return client.Info }

// Shutdown shuts down all uplink dependencies
func (client *Uplink) Shutdown() error { return nil }

// DialMetainfo dials destination with apikey and returns metainfo Client
func (client *Uplink) DialMetainfo(ctx context.Context, destination Peer, apikey *macaroon.APIKey) (*metainfo.Client, error) {
	return metainfo.Dial(ctx, client.Dialer, destination.Addr(), apikey, "Test/1.0")
}

// DialPiecestore dials destination storagenode and returns a piecestore client.
func (client *Uplink) DialPiecestore(ctx context.Context, destination Peer) (*piecestore.Client, error) {
	node := destination.Local()
	return piecestore.Dial(ctx, client.Dialer, &node.Node, client.Log.Named("uplink>piecestore"), piecestore.DefaultConfig)
}

// Upload data to specific satellite
func (client *Uplink) Upload(ctx context.Context, satellite *SatelliteSystem, bucket string, path storj.Path, data []byte) error {
	return client.UploadWithExpiration(ctx, satellite, bucket, path, data, time.Time{})
}

// UploadWithExpiration data to specific satellite and expiration time
func (client *Uplink) UploadWithExpiration(ctx context.Context, satellite *SatelliteSystem, bucket string, path storj.Path, data []byte, expiration time.Time) error {
	return client.UploadWithExpirationAndConfig(ctx, satellite, nil, bucket, path, data, expiration)
}

// UploadWithConfig uploads data to specific satellite with configured values
func (client *Uplink) UploadWithConfig(ctx context.Context, satellite *SatelliteSystem, redundancy *uplink.RSConfig, bucket string, path storj.Path, data []byte) error {
	return client.UploadWithExpirationAndConfig(ctx, satellite, redundancy, bucket, path, data, time.Time{})
}

// UploadWithExpirationAndConfig uploads data to specific satellite with configured values and expiration time
func (client *Uplink) UploadWithExpirationAndConfig(ctx context.Context, satellite *SatelliteSystem, redundancy *uplink.RSConfig, bucketName string, path storj.Path, data []byte, expiration time.Time) (err error) {
	config := client.GetConfig(satellite)
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

	project, bucket, err := client.GetProjectAndBucket(ctx, satellite, bucketName, config)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, bucket.Close(), project.Close()) }()

	opts := &libuplink.UploadOptions{}
	opts.Expires = expiration
	opts.Volatile.RedundancyScheme = config.GetRedundancyScheme()
	opts.Volatile.EncryptionParameters = config.GetEncryptionParameters()

	reader := bytes.NewReader(data)
	if err := bucket.UploadObject(ctx, path, reader, opts); err != nil {
		return err
	}

	return nil
}

// UploadWithClientConfig uploads data to specific satellite with custom client configuration
func (client *Uplink) UploadWithClientConfig(ctx context.Context, satellite *SatelliteSystem, clientConfig uplink.Config, bucketName string, path storj.Path, data []byte) (err error) {
	project, bucket, err := client.GetProjectAndBucket(ctx, satellite, bucketName, clientConfig)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, bucket.Close(), project.Close()) }()

	opts := &libuplink.UploadOptions{}
	opts.Volatile.RedundancyScheme = clientConfig.GetRedundancyScheme()
	opts.Volatile.EncryptionParameters = clientConfig.GetEncryptionParameters()

	reader := bytes.NewReader(data)
	if err := bucket.UploadObject(ctx, path, reader, opts); err != nil {
		return err
	}

	return nil
}

// Download data from specific satellite
func (client *Uplink) Download(ctx context.Context, satellite *SatelliteSystem, bucketName string, path storj.Path) ([]byte, error) {
	project, bucket, err := client.GetProjectAndBucket(ctx, satellite, bucketName, client.GetConfig(satellite))
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, project.Close(), bucket.Close()) }()

	object, err := bucket.OpenObject(ctx, path)
	if err != nil {
		return nil, err
	}

	rc, err := object.DownloadRange(ctx, 0, object.Meta.Size)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rc.Close()) }()

	data, err := ioutil.ReadAll(rc)
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

// DownloadStream returns stream for downloading data
func (client *Uplink) DownloadStream(ctx context.Context, satellite *SatelliteSystem, bucketName string, path storj.Path) (_ io.ReadCloser, cleanup func() error, err error) {
	project, bucket, err := client.GetProjectAndBucket(ctx, satellite, bucketName, client.GetConfig(satellite))
	if err != nil {
		return nil, nil, err
	}

	cleanup = func() error {
		err = errs.Combine(err,
			project.Close(),
			bucket.Close(),
		)
		return err
	}

	downloader, err := bucket.Download(ctx, path)
	return downloader, cleanup, err
}

// DownloadStreamRange returns stream for downloading data
func (client *Uplink) DownloadStreamRange(ctx context.Context, satellite *SatelliteSystem, bucketName string, path storj.Path, start, limit int64) (_ io.ReadCloser, cleanup func() error, err error) {
	project, bucket, err := client.GetProjectAndBucket(ctx, satellite, bucketName, client.GetConfig(satellite))
	if err != nil {
		return nil, nil, err
	}

	cleanup = func() error {
		err = errs.Combine(err,
			project.Close(),
			bucket.Close(),
		)
		return err
	}

	downloader, err := bucket.DownloadRange(ctx, path, start, limit)
	return downloader, cleanup, err
}

// Delete deletes an object at the path in a bucket
func (client *Uplink) Delete(ctx context.Context, satellite *SatelliteSystem, bucketName string, path storj.Path) error {
	project, bucket, err := client.GetProjectAndBucket(ctx, satellite, bucketName, client.GetConfig(satellite))
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, project.Close(), bucket.Close()) }()

	err = bucket.DeleteObject(ctx, path)
	if err != nil {
		return err
	}
	return nil
}

// CreateBucket creates a new bucket
func (client *Uplink) CreateBucket(ctx context.Context, satellite *SatelliteSystem, bucketName string) error {
	project, err := client.GetProject(ctx, satellite)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, project.Close()) }()

	clientCfg := client.GetConfig(satellite)
	bucketCfg := &libuplink.BucketConfig{}
	bucketCfg.PathCipher = clientCfg.GetPathCipherSuite()
	bucketCfg.EncryptionParameters = clientCfg.GetEncryptionParameters()
	bucketCfg.Volatile.RedundancyScheme = clientCfg.GetRedundancyScheme()
	bucketCfg.Volatile.SegmentsSize = clientCfg.GetSegmentSize()

	_, err = project.CreateBucket(ctx, bucketName, bucketCfg)
	if err != nil {
		return err
	}
	return nil
}

// GetConfig returns a default config for a given satellite.
func (client *Uplink) GetConfig(satellite *SatelliteSystem) uplink.Config {
	config := getDefaultConfig()

	// client.APIKey[satellite.ID()] is a *macaroon.APIKey, but we want a
	// *libuplink.APIKey, so, serialize and parse for now
	apiKey, err := libuplink.ParseAPIKey(client.APIKey[satellite.ID()].Serialize())
	if err != nil {
		panic(err)
	}

	encAccess := libuplink.NewEncryptionAccess()
	encAccess.SetDefaultKey(storj.Key{})

	scopeData, err := (&libuplink.Scope{
		SatelliteAddr:    satellite.Addr(),
		APIKey:           apiKey,
		EncryptionAccess: encAccess,
	}).Serialize()
	if err != nil {
		panic(err)
	}

	config.Scope = scopeData

	// Support some legacy stuff
	config.Legacy.Client.APIKey = apiKey.Serialize()
	config.Legacy.Client.SatelliteAddr = satellite.Addr()

	config.Client.DialTimeout = 10 * time.Second

	config.RS.MinThreshold = atLeastOne(client.StorageNodeCount * 1 / 5)     // 20% of storage nodes
	config.RS.RepairThreshold = atLeastOne(client.StorageNodeCount * 2 / 5)  // 40% of storage nodes
	config.RS.SuccessThreshold = atLeastOne(client.StorageNodeCount * 3 / 5) // 60% of storage nodes
	config.RS.MaxThreshold = atLeastOne(client.StorageNodeCount * 4 / 5)     // 80% of storage nodes

	config.TLS.UsePeerCAWhitelist = false
	config.TLS.Extensions.Revocation = false
	config.TLS.Extensions.WhitelistSignedLeaf = false

	return config
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

// NewLibuplink creates a libuplink.Uplink object with the testplanet Uplink config
func (client *Uplink) NewLibuplink(ctx context.Context) (*libuplink.Uplink, error) {
	config := getDefaultConfig()
	libuplinkCfg := &libuplink.Config{}
	libuplinkCfg.Volatile.Log = client.Log
	libuplinkCfg.Volatile.MaxInlineSize = config.Client.MaxInlineSize
	libuplinkCfg.Volatile.MaxMemory = config.RS.MaxBufferMem
	libuplinkCfg.Volatile.PeerIDVersion = config.TLS.PeerIDVersions
	libuplinkCfg.Volatile.TLS.SkipPeerCAWhitelist = !config.TLS.UsePeerCAWhitelist
	libuplinkCfg.Volatile.TLS.PeerCAWhitelistPath = config.TLS.PeerCAWhitelistPath
	libuplinkCfg.Volatile.DialTimeout = config.Client.DialTimeout

	return libuplink.NewUplink(ctx, libuplinkCfg)
}

// GetProject returns a libuplink.Project which allows interactions with a specific project
func (client *Uplink) GetProject(ctx context.Context, satellite *SatelliteSystem) (*libuplink.Project, error) {
	testLibuplink, err := client.NewLibuplink(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, testLibuplink.Close()) }()

	scope, err := client.GetConfig(satellite).GetScope()
	if err != nil {
		return nil, err
	}

	project, err := testLibuplink.OpenProject(ctx, scope.SatelliteAddr, scope.APIKey)
	if err != nil {
		return nil, err
	}
	return project, nil
}

// GetProjectAndBucket returns a libuplink.Project and Bucket which allows interactions with a specific project and its buckets
func (client *Uplink) GetProjectAndBucket(ctx context.Context, satellite *SatelliteSystem, bucketName string, clientCfg uplink.Config) (_ *libuplink.Project, _ *libuplink.Bucket, err error) {
	project, err := client.GetProject(ctx, satellite)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err != nil {
			err = errs.Combine(err, project.Close())
		}
	}()

	scope, err := client.GetConfig(satellite).GetScope()
	if err != nil {
		return nil, nil, err
	}

	// Check if the bucket exists, if not then create it
	_, _, err = project.GetBucketInfo(ctx, bucketName)
	if err != nil {
		if storj.ErrBucketNotFound.Has(err) {
			err := createBucket(ctx, clientCfg, *project, bucketName)
			if err != nil {
				return nil, nil, err
			}
		} else {
			return nil, nil, err
		}
	}

	bucket, err := project.OpenBucket(ctx, bucketName, scope.EncryptionAccess)
	if err != nil {
		return nil, nil, err
	}
	return project, bucket, nil
}

func createBucket(ctx context.Context, config uplink.Config, project libuplink.Project, bucketName string) error {
	bucketCfg := &libuplink.BucketConfig{}
	bucketCfg.PathCipher = config.GetPathCipherSuite()
	bucketCfg.EncryptionParameters = config.GetEncryptionParameters()
	bucketCfg.Volatile.RedundancyScheme = config.GetRedundancyScheme()
	bucketCfg.Volatile.SegmentsSize = config.GetSegmentSize()

	_, err := project.CreateBucket(ctx, bucketName, bucketCfg)
	if err != nil {
		return err
	}
	return nil
}
