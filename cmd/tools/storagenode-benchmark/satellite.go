// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/zeebo/errs"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/process"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/drpc/drpcmigrate"
	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"
)

var (
	runSatelliteCmd = &cobra.Command{
		Use:   "satellite",
		Short: "starts fake satellite",
		RunE:  runSatellite,
	}

	satelliteConfig SatelliteConfig
)

// SatelliteConfig is the configuration for the satellite command.
type SatelliteConfig struct {
	SatelliteAddress string
	IdentityDir      string
}

// BindFlags adds flags to the flagset.
func (config *SatelliteConfig) BindFlags(flag *flag.FlagSet) {
	flag.StringVar(&config.SatelliteAddress, "address", "0.0.0.0:7777", "satellite address")
	flag.StringVar(&config.IdentityDir, "identity-dir", "", "location of the satellite identity")
}

func init() {
	rootCmd.AddCommand(runSatelliteCmd)

	satelliteConfig.BindFlags(runSatelliteCmd.Flags())
}

type nodeEndpoint struct {
	pb.DRPCNodeUnimplementedServer
}

func (s *nodeEndpoint) GetTime(context.Context, *pb.GetTimeRequest) (*pb.GetTimeResponse, error) {
	return &pb.GetTimeResponse{
		Timestamp: time.Now(),
	}, nil
}

func (s *nodeEndpoint) CheckIn(ctx context.Context, req *pb.CheckInRequest) (*pb.CheckInResponse, error) {

	peerID, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	fmt.Println("NodeID:", peerID.ID, "NoisePublicKey(hex):", hex.EncodeToString(req.NoiseKeyAttestation.NoisePublicKey))
	return &pb.CheckInResponse{
		PingNodeSuccess: true,
	}, nil
}

type nodeStatEndpoint struct {
}

func (n *nodeStatEndpoint) DailyStorageUsage(ctx context.Context, request *pb.DailyStorageUsageRequest) (*pb.DailyStorageUsageResponse, error) {
	return &pb.DailyStorageUsageResponse{}, nil
}

func (n *nodeStatEndpoint) PricingModel(ctx context.Context, request *pb.PricingModelRequest) (*pb.PricingModelResponse, error) {
	return &pb.PricingModelResponse{}, nil
}

func (n *nodeStatEndpoint) GetStats(context.Context, *pb.GetStatsRequest) (*pb.GetStatsResponse, error) {
	return &pb.GetStatsResponse{}, nil
}

type heldAmountEndpoint struct {
}

func (h heldAmountEndpoint) GetPayStub(ctx context.Context, request *pb.GetHeldAmountRequest) (*pb.GetHeldAmountResponse, error) {
	return &pb.GetHeldAmountResponse{}, nil
}

func (h heldAmountEndpoint) GetAllPaystubs(ctx context.Context, request *pb.GetAllPaystubsRequest) (*pb.GetAllPaystubsResponse, error) {
	return &pb.GetAllPaystubsResponse{
		Paystub: []*pb.GetHeldAmountResponse{},
	}, nil
}

func (h heldAmountEndpoint) GetPayment(ctx context.Context, request *pb.GetPaymentRequest) (*pb.GetPaymentResponse, error) {
	return &pb.GetPaymentResponse{}, nil
}

func (h heldAmountEndpoint) GetAllPayments(ctx context.Context, request *pb.GetAllPaymentsRequest) (*pb.GetAllPaymentsResponse, error) {
	return &pb.GetAllPaymentsResponse{
		Payment: []*pb.GetPaymentResponse{},
	}, nil
}

type ordersEndpoint struct {
}

func (o *ordersEndpoint) SettlementWithWindow(stream pb.DRPCOrders_SettlementWithWindowStream) error {
	storagenodeSettled := map[int32]int64{}
	for {
		s, err := stream.Recv()
		if err != nil {
			return stream.SendAndClose(&pb.SettlementWithWindowResponse{
				Status:        pb.SettlementWithWindowResponse_ACCEPTED,
				ActionSettled: storagenodeSettled,
			})
		}
		storagenodeSettled[int32(s.Limit.Action)] += s.Order.Amount
	}

}

func runSatellite(cmd *cobra.Command, args []string) error {
	ctx, _ := process.Ctx(cmd)

	config := satelliteConfig
	identityConfig := identity.Config{
		CertPath: filepath.Join(config.IdentityDir, "identity.cert"),
		KeyPath:  filepath.Join(config.IdentityDir, "identity.key"),
	}

	ident, err := identityConfig.Load()
	if err != nil {
		return err
	}

	fmt.Println("Starting ", ident.ID.String()+"@"+config.SatelliteAddress)

	tlsConfig := tlsopts.Config{
		UsePeerCAWhitelist: false,
		PeerIDVersions:     "0",
	}
	tlsOptions, err := tlsopts.NewOptions(ident, tlsConfig, nil)
	if err != nil {
		return errs.Wrap(err)
	}

	tcpListener, err := net.Listen("tcp", config.SatelliteAddress)
	if err != nil {
		return errs.Wrap(err)
	}

	listenMux := drpcmigrate.NewListenMux(tcpListener, len(drpcmigrate.DRPCHeader))
	tlsListener := tls.NewListener(listenMux.Route(drpcmigrate.DRPCHeader), tlsOptions.ServerTLSConfig())
	go func() {
		_ = listenMux.Run(ctx)
	}()
	m := drpcmux.New()

	err = pb.DRPCRegisterNode(m, &nodeEndpoint{})
	if err != nil {
		return errs.Wrap(err)
	}
	err = pb.DRPCRegisterNodeStats(m, &nodeStatEndpoint{})
	if err != nil {
		return errs.Wrap(err)
	}
	err = pb.DRPCRegisterHeldAmount(m, &heldAmountEndpoint{})
	if err != nil {
		return errs.Wrap(err)
	}

	err = pb.DRPCRegisterOrders(m, &ordersEndpoint{})
	if err != nil {
		return errs.Wrap(err)
	}

	serv := drpcserver.NewWithOptions(m, drpcserver.Options{})
	return serv.Serve(ctx, tlsListener)
}
