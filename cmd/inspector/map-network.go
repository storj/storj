package main

import (
	"bytes"
	"context"
	"crypto/x509"
	"github.com/spf13/cobra"
	"log"
	"os"
	"time"

	"storj.io/storj/pkg/graphs"
	"storj.io/storj/pkg/graphs/dot"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storj"
)

var (
	cmdMap = &cobra.Command{
		Use:   "map-network <bootstrap address>",
		Short: "builds a map of the network by recursively walking routing tables",
		Args:  cobra.ExactArgs(1),
		RunE:  mapNetworkCmd,
	}

	colorSigned   = "#2683ff"
	colorUnsigned = "#3fd69a"
	colorErr      = "#d63f3f"
)

func init() {
	rootCmd.AddCommand(cmdMap)
}

type network struct {
	ctx    context.Context
	verify peertls.PeerCertVerificationFunc
	addrs  []string
	seen   map[string]struct{}
	err    *log.Logger
	graph  graphs.Graph
}

func mapNetworkCmd(cmd *cobra.Command, args []string) error {
	whitelist, err := pkcrypto.CertFromPEM([]byte(tlsopts.DefaultPeerCAWhitelist))
	if err != nil {
		// TODO error handling
		log.Printf("error: %s\n", err.Error())
		return nil
	}
	verify := peertls.VerifyCAWhitelist([]*x509.Certificate{whitelist})

	errLog := log.New(os.Stderr, "", 0)
	if *IdentityPath == "" {
		return ErrArgs.New("--identity-path required")
	}

	ctx, _ := context.WithTimeout(process.Ctx(cmd), 30*time.Second)
	n := &network{
		ctx:    ctx,
		verify: verify,
		seen:   map[string]struct{}{args[0]: {}},
		addrs:  []string{args[0]},
		err:    errLog,
		graph:  dot.New("StorjNetwork"),
	}

	n.walk(args[0])
	out := bytes.Buffer{}
	// TODO: is this ok?
	if _, err := n.graph.Write(out.Bytes()); err != nil {
		errLog.Println(err)
		return err
	}
	return nil
}

//func walk(ctx context.Context, next string, queue *Queue) error {
func (n *network) walk(next string) {
	select {
	case <-n.ctx.Done():
		return
	default:
		inspector, err := NewInspector(next, *IdentityPath)
		if err != nil {
			// TODO error handling
			n.err.Print(err)
			return
		}

		kadDialer := kademlia.NewDialer(nil, inspector.transportClient)

		res, err := inspector.kadclient.FindNear(n.ctx, &pb.FindNearRequest{
			Start: storj.NodeID{},
			Limit: 100000,
		})
		if err != nil {
			// TODO error handling
			n.err.Print(err)
		}


		for _, node := range res.Nodes {
			var verifyErr error
			pID, err := kadDialer.FetchPeerIdentity(n.ctx, *node)
			if err != nil {
				// TODO error handling
				n.err.Print(err)
			} else {
				verifyErr = n.verify(nil, [][]*x509.Certificate{append(append([]*x509.Certificate{pID.Leaf}, pID.CA), pID.RestChain...)})
			}

			newAddr := node.Address.Address
			addEdge := func(a, b string) {
				var edge graphs.Edge
				switch {
				case verifyErr != nil:
					edge = dot.NewEdge(next, newAddr, colorUnsigned, "unsigned")
				case err != nil:
					edge = dot.NewEdge(next, newAddr, colorErr, err.Error())
				default:
					edge = dot.NewEdge(next, newAddr, colorSigned, "signed")
				}

				if err := n.graph.AddEdge(edge); err != nil {
					n.err.Print(err)
				}
			}

			if _, ok := n.seen[newAddr]; ok {
				// NB: only applicable for directional graphs
				addEdge(newAddr, next)
				continue
			}

			addEdge(next, newAddr)

			n.seen[newAddr] = struct{}{}
			n.addrs = append(n.addrs, newAddr)

			n.walk(newAddr)
		}
	}
}
