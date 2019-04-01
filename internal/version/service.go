// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/transport"
)

// Config contains the necessary Information to check the Software Version
type Config struct {
	ServerAddress  string        `help:"server address to check its version against" default:"https://satellite.stefan-benten.de/version"`
	RequestTimeout time.Duration `help:"Request timeout for version checks" default:"0h1m0s"`
	CheckInterval  time.Duration `help:"Interval to check the version" default:"0h15m0s"`
}

// Service contains the information and variables to ensure the Software is up to date
type Service struct {
	config *Config
	info   *Info

	Loop *sync2.Cycle

	mu      sync.Mutex
	allowed bool
}

type VersionedClient struct {
	transport transport.Client
	version   *Service
}

const (
	ErrOldVersion = "Outdated Software Version, please update!"
)

// NewService creates a Version Check Client with default configuration
func NewService(config *Config, info *Info) (client *Service) {
	return &Service{
		config: config,
		info:   info,
		Loop:   sync2.NewCycle(config.CheckInterval),
	}
}

// NewVersionedClient returns a transport client which ensures, that the software is up to date
func NewVersionedClient(transport transport.Client, service *Service) *VersionedClient {
	return &VersionedClient{
		transport: transport,
		version:   service,
	}
}

// DialNode returns a grpc connection with tls to a node.
//
// Use this method for communicating with nodes as it is more secure than
// DialAddress. The connection will be established successfully only if the
// target node has the private key for the requested node ID.
func (client *VersionedClient) DialNode(ctx context.Context, node *pb.Node, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	if !client.version.IsUpToDate() {
		return nil, errs.New(ErrOldVersion)
	}
	return client.transport.DialNode(ctx, node, opts...)
}

// DialAddress returns a grpc connection with tls to an IP address.
//
// Do not use this method unless having a good reason. In most cases DialNode
// should be used for communicating with nodes as it is more secure than
// DialAddress.
func (client *VersionedClient) DialAddress(ctx context.Context, address string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	if !client.version.IsUpToDate() {
		return nil, errs.New(ErrOldVersion)
	}
	return client.transport.DialAddress(ctx, address, opts...)
}

// Identity is a getter for the transport's identity
func (client *VersionedClient) Identity() *identity.FullIdentity {
	return client.transport.Identity()
}

// WithObservers returns a new transport including the listed observers.
func (client *VersionedClient) WithObservers(obs ...transport.Observer) transport.Client {
	return &VersionedClient{client.transport.WithObservers(obs...), client.version}
}

// Run logs the current version information
func (srv *Service) Run(ctx context.Context) error {
	return srv.Loop.Run(ctx, func(ctx context.Context) error {
		var err error
		allowed, err := srv.checkVersion(ctx)

		srv.mu.Lock()
		srv.allowed = allowed
		srv.mu.Unlock()

		if err != nil {
			zap.S().Errorf("Failed to do periodic version check: ", err)
		}
		return err
	})
}

// IsUpToDate returns whether if the Service is allowed to operate or not
func (srv *Service) IsUpToDate() bool {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	return srv.allowed
}

// CheckVersion checks if the client is running latest/allowed code
func (srv *Service) checkVersion(ctx context.Context) (allowed bool, err error) {
	defer mon.Task()(&ctx)(&err)
	accepted, err := srv.queryVersionFromControlServer(ctx)
	if err != nil {
		return true, err
	}

	zap.S().Debugf("allowed versions from Control Server: %v", accepted)

	// ToDo: Fetch own Service Tag to compare correctly!
	list := accepted.Storagenode
	if list == nil {
		return true, errs.New("Empty List from Versioning Server")
	}
	if containsVersion(list, srv.info.Version) {
		zap.S().Infof("running on version %s", srv.info.Version.String())
		allowed = true
	} else {
		zap.S().Errorf("running on not allowed/outdated version %s", srv.info.Version.String())
		allowed = false
	}
	return
}

// QueryVersionFromControlServer handles the HTTP request to gather the allowed and latest version information
func (srv *Service) queryVersionFromControlServer(ctx context.Context) (ver AllowedVersions, err error) {
	// Tune Client to have a custom Timeout (reduces hanging software)
	client := http.Client{
		Timeout: srv.config.RequestTimeout,
	}
	// New Request that used the passed in context
	req, err := http.NewRequest("GET", srv.config.ServerAddress, nil)
	if err != nil {
		return AllowedVersions{}, err
	}
	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		return AllowedVersions{}, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return AllowedVersions{}, err
	}
	err = json.Unmarshal(body, &ver)
	return
}

// DebugHandler returns a json representation of the current version information for the binary
func (srv *Service) DebugHandler(w http.ResponseWriter, r *http.Request) {
	j, err := Build.Marshal()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_, err = w.Write(j)
	if err != nil {
		zap.S().Errorf("error writing data to client %v", err)
	}
}
