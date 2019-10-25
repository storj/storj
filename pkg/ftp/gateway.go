// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package ftp

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/wthorp/ftpserver-zap/server"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/memory"
	"storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
)

//ListBucketLimit was chosen based on the max files in a FAT32 dir
const ListBucketLimit = 65534

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

// ToBucketName builds a more-valid bucket name from random strings
func ToBucketName(in string) string {
	if len(in) > 0 && in[0] == '/' {
		in = in[1:]
	}
	out := []rune(in)
	for i, rune := range in {
		if unicode.IsLetter(rune) {
			out[i] = unicode.ToLower(rune)
		} else if rune == '.' {
			out[i] = '.'
		} else {
			out[i] = '-'
		}
	}
	return string(out)
}

// MakeDirectory creates a directory
func (driver *Driver) MakeDirectory(cc server.ClientContext, directory string) (err error) {
	ctx, bucketName, path := ParsePath(directory)
	defer mon.Task()(&ctx)(&err)

	if bucketName == "" {
		directory = ToBucketName(directory)
		zap.S().Debug("creating bucket: ", directory)
		cfg := uplink.BucketConfig{
			PathCipher:           driver.pathCipher,
			EncryptionParameters: driver.encryption,
		}
		cfg.Volatile.RedundancyScheme = driver.redundancy
		cfg.Volatile.SegmentsSize = driver.segmentSize

		_, err = driver.project.CreateBucket(ctx, directory, &cfg)
		return err
	}
	zap.S().Debug("Mkdir: ", directory)

	bucket, err := driver.openBucket(ctx, bucketName)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, bucket.Close()) }()
	return bucket.UploadObject(ctx, path+"/", bytes.NewReader([]byte{}), &uplink.UploadOptions{
		ContentType: "application/directory",
	})

}

// ListBuckets lists all the top level prefixes
func (driver *Driver) ListBuckets(ctx context.Context) (bucketItems []os.FileInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	startAfter := ""
	listOpts := storj.BucketListOptions{
		Direction: storj.Forward,
		Cursor:    startAfter,
		Limit:     ListBucketLimit,
	}
	for {
		list, err := driver.project.ListBuckets(ctx, &listOpts)
		if err != nil {
			return nil, err
		}
		for _, item := range list.Items {
			// Windows creates a folder called "New Folder" whenever you try to make
			// a new folder; this is a hack to accomodate that
			if item.Name == "new-folder" {
				item.Name = "New Folder"
			}
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
	parts := strings.SplitN(path, "/", 2)
	parts[0] = ToBucketName(parts[0])
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

	bucket, err := driver.openBucket(ctx, bucketName)
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
			Limit:     ListBucketLimit,
		})
		if err != nil {
			return files, err
		}

		for _, item := range list.Items {
			if item.IsPrefix && item.Path[len(item.Path)-1] == '/' {
				item.Path = item.Path[:len(item.Path)-1]
			}
			if item.Path == "" {
				continue
			}
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
	ctx, bucketName, path := ParsePath(path)
	defer mon.Task()(&ctx)(&err)

	// If we are writing and we are not in append mode, we should remove the file
	if (flag & os.O_WRONLY) != 0 {
		flag |= os.O_CREATE
		if (flag & os.O_APPEND) == 0 {
			//todo: del file
		}
	}

	bucket, err := driver.openBucket(ctx, bucketName)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, bucket.Close()) }()

	object, err := bucket.OpenObject(ctx, path)
	// file doesn't already exist
	if err != nil {
		if storj.ErrObjectNotFound.Has(err) {
			if (flag&os.O_APPEND) != 0 || (flag&os.O_RDONLY) != 0 {
				return nil, os.ErrNotExist
			}
		}
		return &virtualFile{ctx: ctx, path: path, bucket: bucket, size: 0, flag: flag}, nil
	}
	// file already exists
	if (flag & os.O_EXCL) != 0 {
		return nil, os.ErrExist
	}
	return &virtualFile{ctx: ctx, path: path, bucket: bucket, size: object.Meta.Size, flag: flag}, nil
}

// openBucket wraps project.OpenBucket returning friendlier errors
func (driver *Driver) openBucket(ctx context.Context, bucketName string) (b *uplink.Bucket, err error) {
	bucket, err := driver.project.OpenBucket(ctx, bucketName, driver.access)
	if storj.ErrBucketNotFound.Has(err) {
		return bucket, os.ErrNotExist
	}
	return bucket, err
}

// GetFileInfo gets some info around a file or a directory
func (driver *Driver) GetFileInfo(cc server.ClientContext, path string) (fi os.FileInfo, err error) {
	ctx, bucketName, path := ParsePath(path)
	defer mon.Task()(&ctx)(&err)
	//if path is root
	if bucketName == "" {
		return &virtualFileInfo{name: path, size: 0, isDir: true}, nil
	}
	//if path is a bucket
	bucket, err := driver.openBucket(ctx, bucketName)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, bucket.Close()) }()
	if path == "" {
		return virtualFileInfo{name: bucketName, modTime: bucket.Created, isDir: true}, nil
	}
	//if path is a file
	object, err := bucket.OpenObject(ctx, path)
	if err == nil {
		return virtualFileInfo{name: object.Meta.Path, modTime: object.Meta.Modified, size: object.Meta.Size}, nil
	}
	//if path is a directory
	if storj.ErrObjectNotFound.Has(err) {
		object, err = bucket.OpenObject(ctx, path+"/")
		if storj.ErrObjectNotFound.Has(err) {
			return nil, os.ErrNotExist
		}
	}
	return nil, err
}

// CanAllocate gives the approval to allocate some data
func (driver *Driver) CanAllocate(cc server.ClientContext, size int) (ok bool, err error) {
	ctx, _, _ := ParsePath(cc.Path())
	defer mon.Task()(&ctx)(&err)
	return true, nil
}

// ChmodFile changes the attributes of the file
func (driver *Driver) ChmodFile(cc server.ClientContext, path string, mode os.FileMode) (err error) {
	ctx, _, _ := ParsePath(path)
	defer mon.Task()(&ctx)(&err)
	return nil
}

// DeleteFile deletes a file or a directory
func (driver *Driver) DeleteFile(cc server.ClientContext, path string) (err error) {
	ctx, bucketName, path := ParsePath(path)
	defer mon.Task()(&ctx)(&err)

	if path == "" {
		return driver.project.DeleteBucket(ctx, bucketName)
	}

	bucket, err := driver.openBucket(ctx, bucketName)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, bucket.Close()) }()
	err = bucket.DeleteObject(ctx, path)
	if err != nil {
		err = bucket.DeleteObject(ctx, path+"/")
	}
	return err
}

// RenameFile renames a file or a directory
func (driver *Driver) RenameFile(cc server.ClientContext, from, to string) (err error) {
	ctx, _, _ := ParsePath(cc.Path())
	defer mon.Task()(&ctx)(&err)
	// Windows creates a folder called "New Folder" whenever you try to make
	// a new folder; this is a hack to accomodate that
	if strings.HasSuffix(from, "New Folder") {
		return errs.Combine(
			driver.MakeDirectory(cc, to+"/"),
			driver.DeleteFile(cc, from+"/"),
		)
	}
	return Error.New("can't rename")
}
