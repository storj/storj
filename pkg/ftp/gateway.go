// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package ftp

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"strings"

	"github.com/wthorp/ftpserver-zap/server"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/memory"
	"storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
)

var (
	mon = monkit.Package()
	// Error is the errs class of standard End User Client errors
	Error = errs.Class("Storj FTP error")
)

// ServerConfig determines how FTP listens for requests
type ServerConfig struct {
	Address string `help:"address to serve FTP over" default:"127.0.0.1:7777"`
}

// Driver defines a very basic ftpserver driver
type Driver struct {
	project     *uplink.Project
	access      *uplink.EncryptionAccess
	pathCipher  storj.CipherSuite
	encryption  storj.EncryptionParameters
	redundancy  storj.RedundancyScheme
	segmentSize memory.Size

	Logger       *zap.Logger // Logger
	SettingsFile string      // Settings file
	tlsConfig    *tls.Config // TLS config (if applies)
	nbClients    int32       // Number of clients
	server       server.Settings
}

// NewFtpServer creates a FTP session handler
func NewFtpServer(project *uplink.Project, access *uplink.EncryptionAccess, pathCipher storj.CipherSuite, encryption storj.EncryptionParameters,
	redundancy storj.RedundancyScheme, segmentSize memory.Size, logger *zap.Logger) *server.FtpServer {
	driver := &Driver{
		project:     project,
		access:      access,
		pathCipher:  pathCipher,
		encryption:  encryption,
		redundancy:  redundancy,
		segmentSize: segmentSize,
		Logger:      logger,
		tlsConfig:   &tls.Config{},
		nbClients:   4,
	}
	return server.NewFtpServer(driver)
}

// GetSettings returns some general settings around the server setup
func (driver *Driver) GetSettings() (*server.Settings, error) {
	if driver.server.PublicHost == "" {
		driver.server.PublicHost = "127.0.0.1"
		// driver.server.PublicIPResolver = func(cc server.ClientContext) (string, error) {
		// 	return "127.0.0.1", nil
		// }
	}
	return &driver.server, nil
}

// GetTLSConfig returns a TLS Certificate to use
func (driver *Driver) GetTLSConfig() (*tls.Config, error) {
	return nil, nil
}

// WelcomeUser is called to send the very first welcome message
func (driver *Driver) WelcomeUser(cc server.ClientContext) (string, error) {
	cc.SetDebug(true)
	return fmt.Sprintf("Welcome on the Storj FTP gateway, your ID is %d, your IP:port is %s", cc.ID(), cc.RemoteAddr()), nil
}

// AuthUser authenticates the user and selects an handling driver
func (driver *Driver) AuthUser(cc server.ClientContext, user, pass string) (server.ClientHandlingDriver, error) {
	return driver, nil
}

// UserLeft is called when the user disconnects, even if he never authenticated
func (driver *Driver) UserLeft(cc server.ClientContext) {
}

////////////////////////////////////////////////////////////////////////////////
//                                 FTP stuff                                  //
////////////////////////////////////////////////////////////////////////////////

// ChangeDirectory changes the current working directory
func (driver *Driver) ChangeDirectory(cc server.ClientContext, directory string) (err error) {
	ctx, _, _ := ParsePath(cc.Path())
	defer mon.Task()(&ctx)(&err)
	return err
}

// MakeDirectory creates a directory
func (driver *Driver) MakeDirectory(cc server.ClientContext, directory string) (err error) {
	ctx, _, _ := ParsePath(cc.Path())
	defer mon.Task()(&ctx)(&err)

	cfg := uplink.BucketConfig{
		PathCipher:           driver.pathCipher,
		EncryptionParameters: driver.encryption,
	}
	cfg.Volatile.RedundancyScheme = driver.redundancy
	cfg.Volatile.SegmentsSize = driver.segmentSize

	_, err = driver.project.CreateBucket(ctx, cc.Path(), &cfg)

	return err
}

// ListBuckets lists all the top level prefixes
func (driver *Driver) ListBuckets(ctx context.Context) (bucketItems []os.FileInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	startAfter := ""
	listOpts := storj.BucketListOptions{
		Direction: storj.Forward,
		Cursor:    startAfter,
	}
	for {
		list, err := driver.project.ListBuckets(ctx, &listOpts)
		if err != nil {
			return nil, err
		}
		for _, item := range list.Items {
			bucketItems = append(bucketItems, virtualFileInfo{
				name:    item.Name,
				modTime: item.Created,
				size:    0,
				isDir:   true,
			})
		}
		if !list.More {
			break
		}
		listOpts = listOpts.NextPage(list)
	}

	return bucketItems, err
}

//ParsePath returns buckets and prefixes and a context
func ParsePath(path string) (context.Context, string, string) {
	ctx := context.TODO()
	defer mon.Task()(&ctx)
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}
	parts := strings.SplitN(path, "/", 1)
	if len(parts) == 1 {
		return ctx, parts[0], ""
	}
	return ctx, parts[0], parts[1]

}

// ListFiles lists the files of a directory
func (driver *Driver) ListFiles(cc server.ClientContext) (files []os.FileInfo, err error) {
	ctx, bucketName, path := ParsePath(cc.Path())
	defer mon.Task()(&ctx)(&err)

	if bucketName == "" {
		return driver.ListBuckets(ctx)
	}

	files = make([]os.FileInfo, 0)

	bucket, err := driver.project.OpenBucket(ctx, bucketName, driver.access)
	if err != nil {
		return files, err
	}
	defer func() { err = errs.Combine(err, bucket.Close()) }()

	startAfter := ""

	for {
		list, err := bucket.ListObjects(ctx, &storj.ListOptions{
			Direction: storj.After,
			Cursor:    startAfter,
			Prefix:    path,
			Recursive: false,
			Limit:     1000, //todo
		})
		if err != nil {
			return files, err
		}

		for _, item := range list.Items {
			files = append(files, virtualFileInfo{
				name:    item.Path,
				modTime: item.Modified,
				size:    item.Size,
				isDir:   item.IsPrefix,
			})
		}
		if !list.More {
			break
		}
		startAfter = list.Items[len(list.Items)-1].Path
	}

	return files, nil
}

// OpenFile opens a file in 3 possible modes: read, write, appending write (use appropriate flags)
func (driver *Driver) OpenFile(cc server.ClientContext, path string, flag int) (fs server.FileStream, err error) {
	ctx, _, _ := ParsePath(cc.Path())
	defer mon.Task()(&ctx)(&err)

	// If we are writing and we are not in append mode, we should remove the file
	if (flag & os.O_WRONLY) != 0 {
		flag |= os.O_CREATE
		if (flag & os.O_APPEND) == 0 {
			os.Remove(path)
		}
	}
	return &virtualFile{content: []byte{}}, nil
}

// GetFileInfo gets some info around a file or a directory
func (driver *Driver) GetFileInfo(cc server.ClientContext, path string) (fi os.FileInfo, err error) {
	ctx, _, path := ParsePath(cc.Path())
	defer mon.Task()(&ctx)(&err)
	//todo
	return &virtualFileInfo{name: path, size: 4096, isDir: true}, nil
}

// CanAllocate gives the approval to allocate some data
func (driver *Driver) CanAllocate(cc server.ClientContext, size int) (ok bool, err error) {
	ctx, _, _ := ParsePath(cc.Path())
	defer mon.Task()(&ctx)(&err)
	return true, nil
}

// ChmodFile changes the attributes of the file
func (driver *Driver) ChmodFile(cc server.ClientContext, path string, mode os.FileMode) (err error) {
	ctx, _, _ := ParsePath(cc.Path())
	defer mon.Task()(&ctx)(&err)
	return nil
}

// DeleteFile deletes a file or a directory
func (driver *Driver) DeleteFile(cc server.ClientContext, path string) (err error) {
	ctx, bucketName, path := ParsePath(cc.Path())
	defer mon.Task()(&ctx)(&err)

	bucket, err := driver.project.OpenBucket(ctx, bucketName, driver.access)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, bucket.Close()) }()
	err = bucket.DeleteObject(ctx, path)
	return err
}

// RenameFile renames a file or a directory
func (driver *Driver) RenameFile(cc server.ClientContext, from, to string) (err error) {
	ctx, _, _ := ParsePath(cc.Path())
	defer mon.Task()(&ctx)(&err)
	//todo
	return Error.New("can't rename")
}
