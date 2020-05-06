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

	"github.com/spf13/pflag"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/identity"
	"storj.io/common/macaroon"
	"storj.io/common/pb"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/private/cfgstruct"
	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/satellite/console"
	"storj.io/uplink"
	"storj.io/uplink/private/metainfo"
	"storj.io/uplink/private/piecestore"
)

// Uplink is a general purpose
type Uplink struct {
	Log              *zap.Logger
	Info             pb.Node
	Identity         *identity.FullIdentity
	Dialer           rpc.Dialer
	StorageNodeCount int

	APIKey map[storj.NodeID]*macaroon.APIKey

	// Projects is indexed by the satellite number.
	Projects []*Project
}

// Project contains all necessary information about a user.
type Project struct {
	client *Uplink

	ID    uuid.UUID
	Owner ProjectOwner

	Satellite Peer
	APIKey    string

	RawAPIKey *macaroon.APIKey
}

// ProjectOwner contains information about the project owner.
type ProjectOwner struct {
	ID    uuid.UUID
	Email string
}

// DialMetainfo dials the satellite with the appropriate api key.
func (project *Project) DialMetainfo(ctx context.Context) (*metainfo.Client, error) {
	return project.client.DialMetainfo(ctx, project.Satellite, project.RawAPIKey)
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
	ctx := context.TODO()

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

		ownerID, err := uuid.New()
		if err != nil {
			return nil, err
		}

		owner, err := consoleDB.Users().Insert(ctx,
			&console.User{
				ID:       ownerID,
				FullName: fmt.Sprintf("User %s", projectName),
				Email:    fmt.Sprintf("user@%s.test", projectName),
			},
		)
		if err != nil {
			return nil, err
		}

		owner.Status = console.Active
		err = consoleDB.Users().Update(ctx, owner)
		if err != nil {
			return nil, err
		}

		project, err := consoleDB.Projects().Insert(ctx,
			&console.Project{
				Name:    projectName,
				OwnerID: owner.ID,
			},
		)
		if err != nil {
			return nil, err
		}

		_, err = consoleDB.ProjectMembers().Insert(ctx, owner.ID, project.ID)
		if err != nil {
			return nil, err
		}

		_, err = consoleDB.APIKeys().Create(ctx,
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

		uplink.Projects = append(uplink.Projects, &Project{
			client: uplink,

			ID: project.ID,
			Owner: ProjectOwner{
				ID:    owner.ID,
				Email: owner.Email,
			},

			Satellite: satellite,
			APIKey:    key.Serialize(),

			RawAPIKey: key,
		})
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
func (client *Uplink) Upload(ctx context.Context, satellite *Satellite, bucket string, path storj.Path, data []byte) error {
	return client.UploadWithExpiration(ctx, satellite, bucket, path, data, time.Time{})
}

// UploadWithExpiration data to specific satellite and expiration time
func (client *Uplink) UploadWithExpiration(ctx context.Context, satellite *Satellite, bucketName string, path storj.Path, data []byte, expiration time.Time) error {
	project, err := client.GetNewProject(ctx, satellite)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, project.Close()) }()

	_, err = project.EnsureBucket(ctx, bucketName)
	if err != nil {
		return err
	}

	upload, err := project.UploadObject(ctx, bucketName, path, &uplink.UploadOptions{
		Expires: expiration,
	})
	if err != nil {
		return err
	}

	_, err = io.Copy(upload, bytes.NewReader(data))
	if err != nil {
		abortErr := upload.Abort()
		err = errs.Combine(err, abortErr)
		return err
	}

	return upload.Commit()
}

// UploadWithClientConfig uploads data to specific satellite with custom client configuration
func (client *Uplink) UploadWithClientConfig(ctx context.Context, satellite *Satellite, clientConfig UplinkConfig, bucketName string, path storj.Path, data []byte) (err error) {
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
func (client *Uplink) Download(ctx context.Context, satellite *Satellite, bucketName string, path storj.Path) ([]byte, error) {
	project, err := client.GetNewProject(ctx, satellite)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, project.Close()) }()

	download, err := project.DownloadObject(ctx, bucketName, path, nil)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, download.Close()) }()

	data, err := ioutil.ReadAll(download)
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

// DownloadStream returns stream for downloading data
func (client *Uplink) DownloadStream(ctx context.Context, satellite *Satellite, bucketName string, path storj.Path) (_ io.ReadCloser, cleanup func() error, err error) {
	project, err := client.GetNewProject(ctx, satellite)
	if err != nil {
		return nil, nil, err
	}

	cleanup = func() error {
		err = errs.Combine(err,
			project.Close(),
		)
		return err
	}

	downloader, err := project.DownloadObject(ctx, bucketName, path, nil)
	return downloader, cleanup, err
}

// DownloadStreamRange returns stream for downloading data
func (client *Uplink) DownloadStreamRange(ctx context.Context, satellite *Satellite, bucketName string, path storj.Path, start, limit int64) (_ io.ReadCloser, cleanup func() error, err error) {
	project, err := client.GetNewProject(ctx, satellite)
	if err != nil {
		return nil, nil, err
	}

	cleanup = func() error {
		err = errs.Combine(err,
			project.Close(),
		)
		return err
	}

	downloader, err := project.DownloadObject(ctx, bucketName, path, &uplink.DownloadOptions{
		Offset: start,
		Length: limit,
	})
	return downloader, cleanup, err
}

// DeleteObject deletes an object at the path in a bucket
func (client *Uplink) DeleteObject(ctx context.Context, satellite *Satellite, bucketName string, path storj.Path) error {
	project, err := client.GetNewProject(ctx, satellite)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, project.Close()) }()

	_, err = project.DeleteObject(ctx, bucketName, path)
	if err != nil {
		return err
	}
	return err
}

// CreateBucket creates a new bucket
func (client *Uplink) CreateBucket(ctx context.Context, satellite *Satellite, bucketName string) error {
	project, err := client.GetNewProject(ctx, satellite)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, project.Close()) }()

	_, err = project.CreateBucket(ctx, bucketName)
	if err != nil {
		return err
	}
	return nil
}

// DeleteBucket deletes a bucket.
func (client *Uplink) DeleteBucket(ctx context.Context, satellite *Satellite, bucketName string) error {
	project, err := client.GetNewProject(ctx, satellite)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, project.Close()) }()

	_, err = project.DeleteBucket(ctx, bucketName)
	if err != nil {
		return err
	}
	return nil
}

// GetConfig returns a default config for a given satellite.
func (client *Uplink) GetConfig(satellite *Satellite) UplinkConfig {
	config := getDefaultConfig()

	// client.APIKey[satellite.ID()] is a *macaroon.APIKey, but we want a
	// *libuplink.APIKey, so, serialize and parse for now
	apiKey, err := libuplink.ParseAPIKey(client.APIKey[satellite.ID()].Serialize())
	if err != nil {
		panic(err)
	}

	encAccess := libuplink.NewEncryptionAccess()
	encAccess.SetDefaultKey(storj.Key{})
	encAccess.SetDefaultPathCipher(storj.EncAESGCM)

	accessData, err := (&libuplink.Scope{
		SatelliteAddr:    satellite.URL(),
		APIKey:           apiKey,
		EncryptionAccess: encAccess,
	}).Serialize()
	if err != nil {
		panic(err)
	}

	config.Access = accessData

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

func getDefaultConfig() UplinkConfig {
	config := UplinkConfig{}
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
func (client *Uplink) GetProject(ctx context.Context, satellite *Satellite) (*libuplink.Project, error) {
	testLibuplink, err := client.NewLibuplink(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, testLibuplink.Close()) }()

	access, err := client.GetConfig(satellite).GetAccess()
	if err != nil {
		return nil, err
	}

	project, err := testLibuplink.OpenProject(ctx, access.SatelliteAddr, access.APIKey)
	if err != nil {
		return nil, err
	}
	return project, nil
}

// GetNewProject returns a uplink.Project which allows interactions with a specific project
func (client *Uplink) GetNewProject(ctx context.Context, satellite *Satellite) (*uplink.Project, error) {
	oldAccess, err := client.GetConfig(satellite).GetAccess()
	if err != nil {
		return nil, err
	}

	serializedOldAccess, err := oldAccess.Serialize()
	if err != nil {
		return nil, err
	}

	access, err := uplink.ParseAccess(serializedOldAccess)
	if err != nil {
		return nil, err
	}

	project, err := uplink.OpenProject(ctx, access)
	if err != nil {
		return nil, err
	}
	return project, nil
}

// GetProjectAndBucket returns a libuplink.Project and Bucket which allows interactions with a specific project and its buckets
func (client *Uplink) GetProjectAndBucket(ctx context.Context, satellite *Satellite, bucketName string, clientCfg UplinkConfig) (_ *libuplink.Project, _ *libuplink.Bucket, err error) {
	project, err := client.GetProject(ctx, satellite)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err != nil {
			err = errs.Combine(err, project.Close())
		}
	}()

	access, err := client.GetConfig(satellite).GetAccess()
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

	bucket, err := project.OpenBucket(ctx, bucketName, access.EncryptionAccess)
	if err != nil {
		return nil, nil, err
	}
	return project, bucket, nil
}

func createBucket(ctx context.Context, config UplinkConfig, project libuplink.Project, bucketName string) error {
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
