package certificates

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
)

type CertSignerConfig struct {
	AuthorizationDBURL string
}

type CertificateSigner struct {
	Log *zap.Logger
}

var (
	mon = monkit.Package()
	// Error represents an overlay error
	Error = errs.Class("certificate signing request error")
)

func NewServer(log *zap.Logger) pb.CertificatesServer {
	srv := CertificateSigner{
		Log: log,
	}

	return &srv
}

func (c CertSignerConfig) Run(ctx context.Context, server *provider.Provider) (err error) {
	defer mon.Task()(&ctx)(&err)

	srv := NewServer(zap.L())
	pb.RegisterCertificatesServer(server.GRPC(), srv)

	return server.Run(ctx)
}

func (c CertificateSigner) Sign(ctx context.Context, req *pb.SigningRequest) (*pb.SigningResponse, error) {
	// lookup authtoken
	// sign cert
	// send response
}
