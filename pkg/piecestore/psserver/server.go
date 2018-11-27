// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package psserver

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/sha512"
	"errors"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/gtank/cryptopasta"
	"github.com/mr-tron/base58/base58"
	"github.com/shirou/gopsutil/disk"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/piecestore"
	as "storj.io/storj/pkg/piecestore/psserver/agreementsender"
	"storj.io/storj/pkg/piecestore/psserver/psdb"
	"storj.io/storj/pkg/provider"
)

var (
	mon = monkit.Package()

	// ServerError wraps errors returned from Server struct methods
	ServerError = errs.Class("PSServer error")
)

// Config contains everything necessary for a server
type Config struct {
	Path               string `help:"path to store data in" default:"$CONFDIR"`
	AllocatedDiskSpace int64  `help:"total allocated disk space, default(1GB)" default:"1073741824"`
	AllocatedBandwidth int64  `help:"total allocated bandwidth, default(100GB)" default:"107374182400"`
}

// Run implements provider.Responsibility
func (c Config) Run(ctx context.Context, server *provider.Provider) (err error) {
	defer mon.Task()(&ctx)(&err)

	ctx, cancel := context.WithCancel(ctx)

	s, err := Initialize(ctx, c, server.Identity().Key)
	if err != nil {
		return err
	}

	pb.RegisterPieceStoreRoutesServer(server.GRPC(), s)

	// Run the agreement sender process
	asProcess, err := as.Initialize(s.DB, server.Identity())
	if err != nil {
		return err
	}
	go func() {
		if err := asProcess.Run(ctx); err != nil {
			cancel()
		}
	}()

	defer func() {
		log.Fatal(s.Stop(ctx))
	}()

	return server.Run(ctx)
}

//DirSize returns the total size of the files in that directory
func DirSize(path string) (int64, error) {
	var size int64
	_, err := os.Stat(path)
	if err != nil {
		return 0, errors.New("path doesn't exists")
	}
	adjSize := func(_ string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	}
	err = filepath.Walk(path, adjSize)

	return size, err
}

// Server -- GRPC server meta data used in route calls
type Server struct {
	DataDir          string
	DB               *psdb.DB
	pkey             crypto.PrivateKey
	totalAllocated   int64
	totalBwAllocated int64
	verifier         auth.SignedMessageVerifier
}

// Initialize -- initializes a server struct
func Initialize(ctx context.Context, config Config, pkey crypto.PrivateKey) (*Server, error) {
	dbPath := filepath.Join(config.Path, "piecestore.db")
	dataDir := filepath.Join(config.Path, "piece-store-data")

	// read the allocated disk space from the config file
	allocatedDiskSpace := config.AllocatedDiskSpace
	allocatedBandwidth := config.AllocatedBandwidth

	// get the disk space details
	// The returned path ends in a slash only if it represents a root directory, such as "/" on Unix or `C:\` on Windows.
	rootPath := filepath.Dir(filepath.Clean(config.Path))
	diskSpace, err := disk.Usage(rootPath)
	if err != nil {
		return nil, ServerError.Wrap(err)
	}
	freeDiskSpace := int64(diskSpace.Free)

	db, err := psdb.Open(ctx, dataDir, dbPath)
	if err != nil {
		return nil, ServerError.Wrap(err)
	}

	// get how much is currently used, if for the first time totalUsed = 0
	totalUsed, err := db.SumTTLSizes()
	if err != nil {
		//first time setup
		totalUsed = 0x00
	}

	// get used bandwidth from the beginning of the month to till date
	usedBandwidth, err := db.GetTotalBandwidthBetween(getBeginningOfMonth(), time.Now())
	if err != nil {
		return nil, ServerError.Wrap(err)
	}

	if usedBandwidth > allocatedBandwidth {
		zap.S().Warnf("Exceed the allowed Bandwidth setting")
	} else {
		zap.S().Info("Remaining Bandwidth ", allocatedBandwidth-usedBandwidth)
	}

	// check your hard drive is big enough
	// first time setup as a piece node server
	if (totalUsed == 0x00) && (freeDiskSpace < allocatedDiskSpace) {
		allocatedDiskSpace = freeDiskSpace
		zap.S().Warnf("Disk space is less than requested allocated space, allocating = %d Bytes", allocatedDiskSpace)
	}

	// on restarting the Piece node server, assuming already been working as a node
	// used above the alloacated space, user changed the allocation space setting
	// before restarting
	if totalUsed >= allocatedDiskSpace {
		zap.S().Warnf("Used more space then allocated, allocating = %d Bytes", allocatedDiskSpace)
	}

	// the available diskspace is less than remaining allocated space,
	// due to change of setting before restarting
	if freeDiskSpace < (allocatedDiskSpace - totalUsed) {
		allocatedDiskSpace = freeDiskSpace
		zap.S().Warnf("Disk space is less than requested allocated space, allocating = %d Bytes", allocatedDiskSpace)
	}

	return &Server{
		DataDir:          dataDir,
		DB:               db,
		pkey:             pkey,
		totalAllocated:   allocatedDiskSpace,
		totalBwAllocated: allocatedBandwidth,
		verifier:         auth.NewSignedMessageVerifier(),
	}, nil
}

// New creates a Server with custom db
func New(dataDir string, db *psdb.DB, config Config, pkey crypto.PrivateKey) *Server {
	return &Server{
		DataDir:          dataDir,
		DB:               db,
		pkey:             pkey,
		totalAllocated:   config.AllocatedDiskSpace,
		totalBwAllocated: config.AllocatedBandwidth,
		verifier:         auth.NewSignedMessageVerifier(),
	}
}

// Stop the piececstore node
func (s *Server) Stop(ctx context.Context) (err error) {
	return s.DB.Close()
}

// Piece -- Send meta data about a stored by by Id
func (s *Server) Piece(ctx context.Context, in *pb.PieceId) (*pb.PieceSummary, error) {
	zap.S().Infof("Getting Meta for %s...", in.GetId())

	authorization := in.GetAuthorization()
	if err := s.verifier(authorization); err != nil {
		return nil, ServerError.Wrap(err)
	}

	id, err := getNamespacedPieceID([]byte(in.GetId()), getNamespace(authorization))
	if err != nil {
		return nil, err
	}

	path, err := pstore.PathByID(id, s.DataDir)
	if err != nil {
		return nil, err
	}

	match, err := regexp.MatchString("^[A-Za-z0-9]{20,64}$", id)
	if err != nil {
		return nil, err
	}

	if !match {
		return nil, ServerError.New("invalid ID")
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// Read database to calculate expiration
	ttl, err := s.DB.GetTTLByID(id)
	if err != nil {
		return nil, err
	}

	zap.S().Infof("Successfully retrieved meta for %s.", in.GetId())
	return &pb.PieceSummary{Id: in.GetId(), Size: fileInfo.Size(), ExpirationUnixSec: ttl}, nil
}

// Stats will return statistics about the Server
func (s *Server) Stats(ctx context.Context, in *pb.StatsReq) (*pb.StatSummary, error) {
	zap.S().Infof("Getting Stats...\n")

	totalUsed, err := s.DB.SumTTLSizes()
	if err != nil {
		return nil, err
	}

	totalUsedBandwidth, err := s.DB.GetTotalBandwidthBetween(getBeginningOfMonth(), time.Now())
	if err != nil {
		return nil, err
	}

	return &pb.StatSummary{UsedSpace: totalUsed, AvailableSpace: (s.totalAllocated - totalUsed), UsedBandwidth: totalUsedBandwidth, AvailableBandwidth: (s.totalBwAllocated - totalUsedBandwidth)}, nil
}

// Delete -- Delete data by Id from piecestore
func (s *Server) Delete(ctx context.Context, in *pb.PieceDelete) (*pb.PieceDeleteSummary, error) {
	zap.S().Infof("Deleting %s...", in.GetId())

	authorization := in.GetAuthorization()
	if err := s.verifier(authorization); err != nil {
		return nil, ServerError.Wrap(err)
	}

	id, err := getNamespacedPieceID([]byte(in.GetId()), getNamespace(authorization))
	if err != nil {
		return nil, err
	}
	if err := s.deleteByID(id); err != nil {
		return nil, err
	}

	zap.S().Infof("Successfully deleted %s.", in.GetId())
	return &pb.PieceDeleteSummary{Message: OK}, nil
}

func (s *Server) deleteByID(id string) error {
	if err := pstore.Delete(id, s.DataDir); err != nil {
		return err
	}

	if err := s.DB.DeleteTTLByID(id); err != nil {
		return err
	}

	zap.S().Infof("Deleted data of id (%s)\n", id)

	return nil
}

func (s *Server) verifySignature(ctx context.Context, ba *pb.RenterBandwidthAllocation) error {
	// TODO(security): detect replay attacks
	pi, err := provider.PeerIdentityFromContext(ctx)
	if err != nil {
		return err
	}

	k, ok := pi.Leaf.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return peertls.ErrUnsupportedKey.New("%T", pi.Leaf.PublicKey)
	}

	if ok := cryptopasta.Verify(ba.GetData(), ba.GetSignature(), k); !ok {
		return ServerError.New("failed to verify Signature")
	}
	return nil
}

func getBeginningOfMonth() time.Time {
	t := time.Now()
	y, m, _ := t.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, time.Now().Location())
}

func getNamespacedPieceID(pieceID, namespace []byte) (string, error) {
	if namespace == nil {
		return string(pieceID), nil
	}

	mac := hmac.New(sha512.New, namespace)
	_, err := mac.Write(pieceID)
	if err != nil {
		return "", err
	}
	h := mac.Sum(nil)
	return base58.Encode(h), nil
}

func getNamespace(signedMessage *pb.SignedMessage) []byte {
	return signedMessage.GetData()
}
