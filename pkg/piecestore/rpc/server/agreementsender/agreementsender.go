// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package agreementsender

import (
	"flag"
	"log"
	"time"

	"github.com/zeebo/errs"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/piecestore/rpc/server/psdb"
	"storj.io/storj/pkg/utils"
)

var (
	mon                  = monkit.Package()
	defaultCheckInterval = flag.Duration("piecestore.agreementsender.check_interval", time.Hour, "number of seconds to sleep between agreement checks")

	// ASError wraps errors returned from agreementsender package
	ASError = errs.Class("agreement sender error")
)

// AgreementSender maintains variables required for reading bandwidth agreements from a DB and sending them to a Payers
type AgreementSender struct {
	DB   *psdb.DB
	errs []error
}

// Initialize the Agreement Sender
func Initialize(DB *psdb.DB) (*AgreementSender, error) {
	return &AgreementSender{DB: DB}, nil
}

// Run the afreement sender with a context to cehck for cancel
func (as *AgreementSender) Run(ctx context.Context) error {
	log.Println("AgreementSender is starting up")

	type agreementGroup struct {
		satellite  string
		agreements []*psdb.Agreement
	}

	c := make(chan *agreementGroup, 1)

	ticker := time.NewTicker(*defaultCheckInterval)
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			agreementGroups, err := as.DB.GetBandwidthAllocations()
			if err != nil {
				as.errs = append(as.errs, err)
				continue
			}

			for satellite, agreements := range agreementGroups {
				c <- &agreementGroup{satellite, agreements}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return utils.CombineErrors(as.errs...)
		case agreementGroup := <-c:
			go func() {
				log.Printf("Sending Sending %v agreements to satellite %s\n", len(agreementGroup.agreements), agreementGroup.satellite)

				// TODO: Get satellite ip from agreementGroup.satellite
				satelliteAddr := ":7777"

				// TODO: Create client from satellite ip
				identOpt, err := identity.DialOption()
				if err != nil {
					as.errs = append(as.errs, err)
					return
				}

				var conn *grpc.ClientConn
				conn, err = grpc.Dial(satelliteAddr, identOpt)
				if err != nil {
					as.errs = append(as.errs, err)
					return
				}

				client := pb.NewBandwidthClient(conn)
				stream, err := client.BandwidthAgreements(ctx)
				if err != nil {
					as.errs = append(as.errs, err)
					return
				}

				for _, agreement := range agreementGroup.agreements {
					log.Println(agreement)

					// TODO: Deserealize agreement
					msg := &pb.RenterBandwidthAllocation{}

					// Send agreement to satellite
					if err = stream.Send(msg); err != nil {
						if _, closeErr := stream.CloseAndRecv(); closeErr != nil {
							log.Printf("error closing stream %s :: %v.Send() = %v", closeErr, stream, closeErr)
						}

						as.errs = append(as.errs, err)
						return
					}

					// TODO: Delete from PSDB by signature
				}
			}()
		}
	}
}
