// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"runtime/pprof"
	"strconv"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/identity"
	"storj.io/common/macaroon"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
	"storj.io/uplink"
	"storj.io/uplink/private/metainfo"
	"storj.io/uplink/private/piecestore"
	"storj.io/uplink/private/testuplink"
)

// Uplink is a registered user on all satellites,
// which contains the necessary accesses and project info.
type Uplink struct {
	Log      *zap.Logger
	Identity *identity.FullIdentity
	Dialer   rpc.Dialer
	Config   uplink.Config

	APIKey map[storj.NodeID]*macaroon.APIKey
	Access map[storj.NodeID]*uplink.Access

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

// newUplinks creates initializes uplinks, requires peer to have at least one satellite.
func (planet *Planet) newUplinks(ctx context.Context, prefix string, count int) ([]*Uplink, error) {
	var xs []*Uplink
	for i := 0; i < count; i++ {
		name := prefix + strconv.Itoa(i)
		var uplink *Uplink
		var err error
		pprof.Do(ctx, pprof.Labels("peer", name), func(ctx context.Context) {
			uplink, err = planet.newUplink(ctx, name)
		})
		if err != nil {
			return nil, err
		}
		xs = append(xs, uplink)
	}

	return xs, nil
}

// newUplink creates a new uplink.
func (planet *Planet) newUplink(ctx context.Context, name string) (*Uplink, error) {
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

	planetUplink := &Uplink{
		Log:      planet.log.Named(name),
		Identity: identity,
		APIKey:   map[storj.NodeID]*macaroon.APIKey{},
		Access:   map[storj.NodeID]*uplink.Access{},
	}

	planetUplink.Log.Debug("id=" + identity.ID.String())

	planetUplink.Dialer = rpc.NewDefaultDialer(tlsOptions)

	for j, satellite := range planet.Satellites {
		consoleAPI := satellite.API.Console

		projectName := fmt.Sprintf("%s_%d", name, j)
		user, err := satellite.AddUser(ctx, console.CreateUser{
			FullName: fmt.Sprintf("User %s", projectName),
			Email:    fmt.Sprintf("user@%s.test", projectName),
		}, 10)
		if err != nil {
			return nil, err
		}

		project, err := satellite.AddProject(ctx, user.ID, projectName)
		if err != nil {
			return nil, err
		}

		authCtx, err := satellite.AuthenticatedContext(ctx, user.ID)
		if err != nil {
			return nil, err
		}
		_, apiKey, err := consoleAPI.Service.CreateAPIKey(authCtx, project.ID, "root")
		if err != nil {
			return nil, err
		}

		planetUplink.APIKey[satellite.ID()] = apiKey

		planetUplink.Projects = append(planetUplink.Projects, &Project{
			client: planetUplink,

			ID: project.ID,
			Owner: ProjectOwner{
				ID:    user.ID,
				Email: user.Email,
			},

			Satellite: satellite,
			APIKey:    apiKey.Serialize(),

			RawAPIKey: apiKey,
		})
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
func (client *Uplink) DialMetainfo(ctx context.Context, destination Peer, apikey *macaroon.APIKey) (*metainfo.Client, error) {
	return metainfo.DialNodeURL(ctx, client.Dialer, destination.NodeURL().String(), apikey, "Test/1.0")
}

// DialPiecestore dials destination storagenode and returns a piecestore client.
func (client *Uplink) DialPiecestore(ctx context.Context, destination Peer) (*piecestore.Client, error) {
	return piecestore.DialNodeURL(ctx, client.Dialer, destination.NodeURL(), client.Log.Named("uplink>piecestore"), piecestore.DefaultConfig)
}

// Upload data to specific satellite.
func (client *Uplink) Upload(ctx context.Context, satellite *Satellite, bucket string, path storj.Path, data []byte) error {
	return client.UploadWithExpiration(ctx, satellite, bucket, path, data, time.Time{})
}

// UploadWithExpiration data to specific satellite and expiration time.
func (client *Uplink) UploadWithExpiration(ctx context.Context, satellite *Satellite, bucketName string, path storj.Path, data []byte, expiration time.Time) error {
	_, found := testuplink.GetMaxSegmentSize(ctx)
	if !found {
		ctx = testuplink.WithMaxSegmentSize(ctx, satellite.Config.Metainfo.MaxSegmentSize)
	}

	project, err := client.GetProject(ctx, satellite)
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

// Download data from specific satellite.
func (client *Uplink) Download(ctx context.Context, satellite *Satellite, bucketName string, path storj.Path) ([]byte, error) {
	project, err := client.GetProject(ctx, satellite)
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

// DownloadStream returns stream for downloading data.
func (client *Uplink) DownloadStream(ctx context.Context, satellite *Satellite, bucketName string, path storj.Path) (_ io.ReadCloser, cleanup func() error, err error) {
	project, err := client.GetProject(ctx, satellite)
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

// DownloadStreamRange returns stream for downloading data.
func (client *Uplink) DownloadStreamRange(ctx context.Context, satellite *Satellite, bucketName string, path storj.Path, start, limit int64) (_ io.ReadCloser, cleanup func() error, err error) {
	project, err := client.GetProject(ctx, satellite)
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

// DeleteObject deletes an object at the path in a bucket.
func (client *Uplink) DeleteObject(ctx context.Context, satellite *Satellite, bucketName string, path storj.Path) error {
	project, err := client.GetProject(ctx, satellite)
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

// CreateBucket creates a new bucket.
func (client *Uplink) CreateBucket(ctx context.Context, satellite *Satellite, bucketName string) error {
	project, err := client.GetProject(ctx, satellite)
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
	project, err := client.GetProject(ctx, satellite)
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

// GetProject returns a uplink.Project which allows interactions with a specific project.
func (client *Uplink) GetProject(ctx context.Context, satellite *Satellite) (*uplink.Project, error) {
	access := client.Access[satellite.ID()]

	project, err := client.Config.OpenProject(ctx, access)
	if err != nil {
		return nil, err
	}
	return project, nil
}
