// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"flag"
	"fmt"
	"net"
	"path/filepath"
	"os"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/kademlia"
	proto "storj.io/storj/protos/overlay"
	"storj.io/storj/storage/redis"
	"google.golang.org/grpc/credentials"
	"github.com/zeebo/errs"
)

type TlsCredFiles struct {
	certRelPath string
	certAbsPath string
	keyRelPath string
	keyAbsPath string
}

func (t *TlsCredFiles) ensureAbsPaths() (_ error){
	if t.certAbsPath == "" {
		if t.certRelPath == "" {
			return errs.New("No relative certificate path provided")
		}

		certAbsPath, err := filepath.Abs(t.certRelPath); if err != nil {
			return errs.New(err.Error())
		}

		t.certAbsPath = certAbsPath
	}

	if t.keyAbsPath == "" {
		if t.keyRelPath == "" {
			return errs.New("No relative key path provided")
		}

		keyAbsPath, err := filepath.Abs(t.keyRelPath); if err != nil {
			return errs.New(err.Error())
		}

		t.keyAbsPath = keyAbsPath
	}

	return nil
}

func (t *TlsCredFiles) ensureExists() (_ error) {
	err := t.ensureAbsPaths(); if err != nil {
		return err
	}

	_, err = os.Stat(t.certAbsPath); if err != nil {
		if os.IsNotExist(err) {
			// TODO: generate cert
			// - generate key as well?
			// - overwrite old key?
			// return generateSomething(certPath)
			return errs.New("cert/key generation not implemented")
		}

		return errs.New(err.Error())
	}

	return nil
}

func (t *TlsCredFiles) NewServerTLSFromFile() (_ credentials.TransportCredentials,_ error) {
	t.ensureExists()
	creds, err := credentials.NewServerTLSFromFile(t.certAbsPath, t.keyAbsPath)

	return creds, errs.New(err.Error())
}

func (t *TlsCredFiles) NewClientTLSFromFile() (_ credentials.TransportCredentials, _ error) {
	t.ensureExists()
	creds, err := credentials.NewClientTLSFromFile(t.certAbsPath, "")

	return creds, errs.New(err.Error())
}


var (
	redisAddress  string
	redisPassword string
	db            int
)

func init() {
	flag.StringVar(&redisAddress, "cache", "", "The <IP:PORT> string to use for connection to a redis cache")
	flag.StringVar(&redisPassword, "password", "", "The password used for authentication to a secured redis instance")
	flag.IntVar(&db, "db", 0, "The network cache database")
}

// NewServer creates a new Overlay Service Server
func NewServer(tlsCredFiles *TlsCredFiles) (*grpc.Server, error) {
	if tlsCredFiles == nil {
		tlsCredFiles = &TlsCredFiles{
			// TODO: better defaults, env vars, etc.
			certRelPath: "./tls.cert",
			keyRelPath: "./tls.key",
		}
	}

	creds, err := tlsCredFiles.NewServerTLSFromFile()
	if err != nil {
		return nil, err
	}

	credsOption := grpc.Creds(creds)
	grpcServer := grpc.NewServer(credsOption)
	proto.RegisterOverlayServer(grpcServer, &Overlay{})

	return grpcServer, nil
}

// NewClient connects to grpc server at the provided address with the provided options
// returns a new instance of an overlay Client
func NewClient(serverAddr *string, opts ...grpc.DialOption) (proto.OverlayClient, error) {
	certPath, err := filepath.Abs("./tls.cert"); if err != nil {
		return nil, err
	}

	creds, err := credentials.NewClientTLSFromFile(certPath, "")
	if err != nil {
		return nil, err
	}

	credsOption := grpc.WithTransportCredentials(creds)
	opts = append(opts, credsOption)
	conn, err := grpc.Dial(*serverAddr, opts...)
	if err != nil {
		return nil, err
	}

	return proto.NewOverlayClient(conn), nil
}

// Service contains all methods needed to implement the process.Service interface
type Service struct {
	logger  *zap.Logger
	metrics *monkit.Registry
}

// Process is the main function that executes the service
func (s *Service) Process(ctx context.Context) error {
	// bootstrap network
	kad := kademlia.Kademlia{}

	kad.Bootstrap(ctx)
	// bootstrap cache
	cache, err := redis.NewOverlayClient(redisAddress, redisPassword, db, kad)
	if err != nil {
		s.logger.Error("Failed to create a new overlay client", zap.Error(err))
		return err
	}
	if err := cache.Bootstrap(ctx); err != nil {
		s.logger.Error("Failed to boostrap cache", zap.Error(err))
		return err
	}

	// send off cache refreshes concurrently
	go cache.Refresh(ctx)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
	if err != nil {
		s.logger.Error("Failed to initialize TCP connection", zap.Error(err))
		return err
	}

	grpcServer := grpc.NewServer()
	proto.RegisterOverlayServer(grpcServer, &Overlay{})

	defer grpcServer.GracefulStop()
	return grpcServer.Serve(lis)

}

// SetLogger adds the initialized logger to the Service
func (s *Service) SetLogger(l *zap.Logger) error {
	s.logger = l
	return nil
}

// SetMetricHandler adds the initialized metric handler to the Service
func (s *Service) SetMetricHandler(m *monkit.Registry) error {
	s.metrics = m
	return nil
}

// InstanceID implements Service.InstanceID
func (s *Service) InstanceID() string { return "" }
