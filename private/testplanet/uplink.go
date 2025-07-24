// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"runtime/pprof"
	"strconv"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/grant"
	"storj.io/common/identity"
	"storj.io/common/macaroon"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/uplink"
	"storj.io/uplink/private/metaclient"
	"storj.io/uplink/private/object"
	"storj.io/uplink/private/piecestore"
	"storj.io/uplink/private/testuplink"
)

// UplinkConfig testplanet configuration for uplink.
type UplinkConfig struct {
	DefaultPathCipher storj.CipherSuite
	APIKeyVersion     macaroon.APIKeyVersion
}

// Uplink is a registered user on all satellites,
// which contains the necessary accesses and project info.
type Uplink struct {
	Log      *zap.Logger
	Identity *identity.FullIdentity
	Dialer   rpc.Dialer
	Config   uplink.Config

	APIKey map[storj.NodeID]*macaroon.APIKey
	Access map[storj.NodeID]*uplink.Access
	User   map[storj.NodeID]UserLogin

	// Projects is indexed by the satellite number.
	Projects []*Project
}

// Project contains all necessary information about a project.
type Project struct {
	client *Uplink

	ID       uuid.UUID
	PublicID uuid.UUID
	Owner    ProjectOwner

	Satellite Peer
	APIKey    string

	RawAPIKey *macaroon.APIKey
}

// ProjectOwner contains information about the project owner.
type ProjectOwner struct {
	ID    uuid.UUID
	Email string
}

// UserLogin contains information about the user login.
type UserLogin struct {
	Email    string
	Password string
}

// DialMetainfo dials the satellite with the appropriate api key.
func (project *Project) DialMetainfo(ctx context.Context) (_ *metaclient.Client, err error) {
	defer mon.Task()(&ctx)(&err)
	return project.client.DialMetainfo(ctx, project.Satellite, project.RawAPIKey)
}

// newUplinks creates initializes uplinks, requires peer to have at least one satellite.
func (planet *Planet) newUplinks(ctx context.Context, prefix string, count int) (_ []*Uplink, err error) {
	defer mon.Task()(&ctx)(&err)

	var xs []*Uplink
	for i := 0; i < count; i++ {
		name := prefix + strconv.Itoa(i)

		log := planet.log.Named(name)

		var uplink *Uplink
		var err error
		pprof.Do(ctx, pprof.Labels("peer", name), func(ctx context.Context) {
			uplink, err = planet.newUplink(ctx, i, log, name)
		})
		if err != nil {
			return nil, errs.Wrap(err)
		}
		xs = append(xs, uplink)
	}

	return xs, nil
}

// newUplink creates a new uplink.
func (planet *Planet) newUplink(ctx context.Context, index int, log *zap.Logger, name string) (_ *Uplink, err error) {
	defer mon.Task()(&ctx)(&err)

	identity, err := planet.NewIdentity()
	if err != nil {
		return nil, errs.Wrap(err)
	}

	tlsOptions, err := tlsopts.NewOptions(identity, tlsopts.Config{
		PeerIDVersions: strconv.Itoa(int(planet.config.IdentityVersion.Number)),
	}, nil)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	planetUplink := &Uplink{
		Log:      planet.log.Named(name),
		Identity: identity,
		APIKey:   map[storj.NodeID]*macaroon.APIKey{},
		Access:   map[storj.NodeID]*uplink.Access{},
		User:     map[storj.NodeID]UserLogin{},
	}

	planetUplink.Log.Debug("id=" + identity.ID.String())

	planetUplink.Dialer = rpc.NewDefaultDialer(tlsOptions)

	for j, satellite := range planet.Satellites {
		var config UplinkConfig
		if planet.config.Reconfigure.Uplink != nil {
			planet.config.Reconfigure.Uplink(log, index, &config)
		}

		projectName := fmt.Sprintf("%s_%d", name, j)
		user, err := satellite.AddUser(ctx, console.CreateUser{
			FullName: "User " + projectName,
			Email:    fmt.Sprintf("user@%s.test", projectName),
		}, 10)
		if err != nil {
			return nil, errs.Wrap(err)
		}

		planetUplink.User[satellite.ID()] = UserLogin{
			Email:    user.Email,
			Password: user.FullName,
		}

		project, err := satellite.AddProject(ctx, user.ID, projectName)
		if err != nil {
			return nil, errs.Wrap(err)
		}

		apiKey, err := satellite.CreateAPIKey(ctx, project.ID, project.OwnerID, config.APIKeyVersion)
		if err != nil {
			return nil, errs.Wrap(err)
		}

		planetUplink.APIKey[satellite.ID()] = apiKey

		planetUplink.Projects = append(planetUplink.Projects, &Project{
			client: planetUplink,

			ID:       project.ID,
			PublicID: project.PublicID,
			Owner: ProjectOwner{
				ID:    user.ID,
				Email: user.Email,
			},

			Satellite: satellite,
			APIKey:    apiKey.Serialize(),

			RawAPIKey: apiKey,
		})

		// create access grant manually to avoid dialing satellite for
		// project id and deriving key with argon2.IDKey method
		encAccess := grant.NewEncryptionAccessWithDefaultKey(&storj.Key{})
		if config.DefaultPathCipher == storj.EncUnspecified {
			encAccess.SetDefaultPathCipher(storj.EncAESGCM)
		} else {
			encAccess.SetDefaultPathCipher(config.DefaultPathCipher)
		}

		grantAccess := grant.Access{
			SatelliteAddress: satellite.URL(),
			APIKey:           apiKey,
			EncAccess:        encAccess,
		}

		serializedAccess, err := grantAccess.Serialize()
		if err != nil {
			return nil, errs.Wrap(err)
		}

		access, err := uplink.ParseAccess(serializedAccess)
		if err != nil {
			return nil, errs.Wrap(err)
		}

		planetUplink.Access[satellite.ID()] = access
	}

	planet.Uplinks = append(planet.Uplinks, planetUplink)

	return planetUplink, nil
}

// ID returns uplink id.
func (client *Uplink) ID() storj.NodeID { return client.Identity.ID }

// Addr returns uplink address.
func (client *Uplink) Addr() string { return "" }

// Shutdown shuts down all uplink dependencies.
func (client *Uplink) Shutdown() error { return nil }

// DialMetainfo dials destination with apikey and returns metainfo Client.
func (client *Uplink) DialMetainfo(ctx context.Context, destination Peer, apikey *macaroon.APIKey) (_ *metaclient.Client, err error) {
	defer mon.Task()(&ctx)(&err)
	return metaclient.DialNodeURL(ctx, client.Dialer, destination.NodeURL().String(), apikey, "Test/1.0")
}

// DialPiecestore dials destination storagenode and returns a piecestore client.
func (client *Uplink) DialPiecestore(ctx context.Context, destination Peer) (_ *piecestore.Client, err error) {
	defer mon.Task()(&ctx)(&err)
	return piecestore.Dial(ctx, client.Dialer, destination.NodeURL(), piecestore.DefaultConfig)
}

// OpenProject opens project with predefined access grant and gives access to pure uplink API.
func (client *Uplink) OpenProject(ctx context.Context, satellite *Satellite) (_ *uplink.Project, err error) {
	defer mon.Task()(&ctx)(&err)
	_, found := testuplink.GetMaxSegmentSize(ctx)
	if !found {
		ctx = testuplink.WithMaxSegmentSize(ctx, satellite.Config.Metainfo.MaxSegmentSize)
	}
	return uplink.OpenProject(ctx, client.Access[satellite.ID()])
}

// Upload data to specific satellite.
func (client *Uplink) Upload(ctx context.Context, satellite *Satellite, bucket string, path storj.Path, data []byte) (err error) {
	defer mon.Task()(&ctx)(&err)
	return errs.Wrap(client.UploadWithExpiration(ctx, satellite, bucket, path, data, time.Time{}))
}

// UploadWithExpiration data to specific satellite and expiration time.
func (client *Uplink) UploadWithExpiration(ctx context.Context, satellite *Satellite, bucketName string, key string, data []byte, expiration time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = client.UploadWithOptions(ctx, satellite, bucketName, key, data, &metaclient.UploadOptions{
		Expires: expiration,
	})
	return errs.Wrap(err)
}

// UploadWithOptions uploads data to specific satellite, with defined options.
func (client *Uplink) UploadWithOptions(ctx context.Context, satellite *Satellite, bucketName, key string, data []byte, options *metaclient.UploadOptions) (obj *object.VersionedObject, err error) {
	defer mon.Task()(&ctx)(&err)

	_, found := testuplink.GetMaxSegmentSize(ctx)
	if !found {
		ctx = testuplink.WithMaxSegmentSize(ctx, satellite.Config.Metainfo.MaxSegmentSize)
	}

	project, err := client.GetProject(ctx, satellite)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, project.Close()) }()

	err = client.TestingCreateBucket(ctx, satellite, bucketName)
	if err != nil && !buckets.ErrBucketAlreadyExists.Has(err) {
		return nil, errs.Wrap(err)
	}

	upload, err := object.UploadObject(ctx, project, bucketName, key, options)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	_, err = io.Copy(upload, bytes.NewReader(data))
	if err != nil {
		abortErr := upload.Abort()
		err = errs.Combine(err, abortErr)
		return nil, errs.Wrap(err)
	}

	err = upload.Commit()
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return upload.Info(), nil
}

// Download data from specific satellite.
func (client *Uplink) Download(ctx context.Context, satellite *Satellite, bucketName string, path storj.Path) (_ []byte, err error) {
	defer mon.Task()(&ctx)(&err)

	project, err := client.GetProject(ctx, satellite)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, project.Close()) }()

	download, err := project.DownloadObject(ctx, bucketName, path, nil)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, download.Close()) }()

	data, err := io.ReadAll(download)
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

// DownloadStream returns stream for downloading data.
func (client *Uplink) DownloadStream(ctx context.Context, satellite *Satellite, bucketName string, path storj.Path) (_ io.ReadCloser, cleanup func() error, err error) {
	defer mon.Task()(&ctx)(&err)

	project, err := client.GetProject(ctx, satellite)
	if err != nil {
		return nil, nil, errs.Wrap(err)
	}

	cleanup = func() error {
		err = errs.Combine(err,
			project.Close(),
		)
		return errs.Wrap(err)
	}

	downloader, err := project.DownloadObject(ctx, bucketName, path, nil)
	return downloader, cleanup, err
}

// DownloadStreamRange returns stream for downloading data.
func (client *Uplink) DownloadStreamRange(ctx context.Context, satellite *Satellite, bucketName string, path storj.Path, start, limit int64) (_ io.ReadCloser, cleanup func() error, err error) {
	defer mon.Task()(&ctx)(&err)

	project, err := client.GetProject(ctx, satellite)
	if err != nil {
		return nil, nil, errs.Wrap(err)
	}

	cleanup = func() error {
		return errs.Combine(err, project.Close())
	}

	downloader, err := project.DownloadObject(ctx, bucketName, path, &uplink.DownloadOptions{
		Offset: start,
		Length: limit,
	})
	return downloader, cleanup, errs.Wrap(err)
}

// DeleteObject deletes an object at the path in a bucket.
func (client *Uplink) DeleteObject(ctx context.Context, satellite *Satellite, bucketName string, path storj.Path) (err error) {
	defer mon.Task()(&ctx)(&err)

	project, err := client.GetProject(ctx, satellite)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, project.Close()) }()

	_, err = project.DeleteObject(ctx, bucketName, path)
	if err != nil {
		return errs.Wrap(err)
	}
	return errs.Wrap(err)
}

// CopyObject copies an object.
func (client *Uplink) CopyObject(ctx context.Context, satellite *Satellite, oldBucket, oldKey, newBucket, newKey string) (err error) {
	defer mon.Task()(&ctx)(&err)

	project, err := client.GetProject(ctx, satellite)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, project.Close()) }()

	_, err = project.CopyObject(ctx, oldBucket, oldKey, newBucket, newKey, nil)
	return err
}

// TestingCreateBucket creates a new bucket for testing.
// It's doing it using directly DB API so it avoids a lot of overhead from uplink and satellite.
func (client *Uplink) TestingCreateBucket(ctx context.Context, satellite *Satellite, bucketName string) (err error) {
	defer mon.Task()(&ctx)(&err)

	var projectID uuid.UUID
	for _, project := range client.Projects {
		if project.Satellite == satellite {
			projectID = project.ID
			break
		}
	}

	if projectID.IsZero() {
		return errs.New("project not found for satellite %s", satellite.ID())
	}

	_, err = satellite.DB.Buckets().CreateBucket(ctx, buckets.Bucket{
		Name:       bucketName,
		ProjectID:  projectID,
		Versioning: buckets.Unversioned,
	})
	return errs.Wrap(err)
}

// CreateBucket creates a new bucket. It's doing it using uplink API.
func (client *Uplink) CreateBucket(ctx context.Context, satellite *Satellite, bucketName string) (err error) {
	defer mon.Task()(&ctx)(&err)

	project, err := client.GetProject(ctx, satellite)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, project.Close()) }()

	_, err = project.EnsureBucket(ctx, bucketName)
	if err != nil {
		return errs.Wrap(err)
	}
	return nil
}

// DeleteBucket deletes a bucket.
func (client *Uplink) DeleteBucket(ctx context.Context, satellite *Satellite, bucketName string) (err error) {
	defer mon.Task()(&ctx)(&err)

	project, err := client.GetProject(ctx, satellite)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, project.Close()) }()

	_, err = project.DeleteBucket(ctx, bucketName)
	if err != nil {
		return errs.Wrap(err)
	}
	return nil
}

// ListBuckets returns a list of all buckets in a project.
func (client *Uplink) ListBuckets(ctx context.Context, satellite *Satellite) (_ []*uplink.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	var buckets = []*uplink.Bucket{}
	project, err := client.GetProject(ctx, satellite)
	if err != nil {
		return buckets, errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, project.Close()) }()

	iter := project.ListBuckets(ctx, &uplink.ListBucketsOptions{})
	for iter.Next() {
		buckets = append(buckets, iter.Item())
	}
	return buckets, iter.Err()
}

// ListObjects returns a list of all objects in a bucket.
func (client *Uplink) ListObjects(ctx context.Context, satellite *Satellite, bucketName string) (_ []*uplink.Object, err error) {
	defer mon.Task()(&ctx)(&err)

	var objects = []*uplink.Object{}
	project, err := client.GetProject(ctx, satellite)
	if err != nil {
		return objects, errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, project.Close()) }()

	iter := project.ListObjects(ctx, bucketName, &uplink.ListObjectsOptions{})
	for iter.Next() {
		objects = append(objects, iter.Item())
	}
	return objects, iter.Err()
}

// GetProject returns a uplink.Project which allows interactions with a specific project.
func (client *Uplink) GetProject(ctx context.Context, satellite *Satellite) (_ *uplink.Project, err error) {
	defer mon.Task()(&ctx)(&err)

	access := client.Access[satellite.ID()]

	project, err := client.Config.OpenProject(ctx, access)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return project, nil
}
