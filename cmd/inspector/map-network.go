package main

import (
	"bytes"
	"context"
	"crypto/x509"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"log"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/storage"
	"storj.io/storj/storage/testqueue"
	"sync"
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

const (
	InspectorKey     = "inspector"
	AddrKey          = "addr"
	WorkQueueKey     = "work_queue"
	IdentityQueueKey = "identity_queue"
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

func mapNetworkCmd(cmd *cobra.Command, args []string) error {
	whitelist, err := pkcrypto.CertFromPEM([]byte(tlsopts.DefaultPeerCAWhitelist))
	if err != nil {
		// TODO error handling
		log.Printf("error: %s\n", err.Error())
		return nil
	}
	verify := peertls.VerifyCAWhitelist([]*x509.Certificate{whitelist})
	graph := dot.New("Storj.Network")

	if *IdentityPath == "" {
		return ErrArgs.New("--identity-path required")
	}

	ctx, _ := context.WithTimeout(process.Ctx(cmd), 30*time.Second)
	wq := testqueue.NewWorkGroup(
		ctx,
		zap.L(),
		10,
		mapKad,
	)

	//wq.walk(args[0])
	wq.Go()
	out := bytes.Buffer{}
	// TODO: is this ok?
	if _, err := wq.graph.Write(out.Bytes()); err != nil {
		errLog.Println(err)
		return err
	}
	return nil
}

type KadWork struct {
	testqueue.Work

	identityQueue storage.Queue

	mu        sync.Mutex
	seenNodes map[string]kadWorkItem
	waitCount *int

	Inspector  *Inspector
	VerifyFunc peertls.PeerCertVerificationFunc
	Graph      graphs.Graph

	Address string
}

type kadWorkItem struct {
	node      *pb.Node
	ident     *identity.PeerIdentity
	verifyErr error
}

func (kadWork *KadWork) Addr() string {
	return kadWork.Node.Address.Address
}

func newKadwork(work testqueue.Work) (*KadWork, error) {
	graph := work.Ctx.Value(GraphKey).(graphs.Graph)
	verifyFunc := work.Ctx.Value(VerifyFuncKey).(peertls.PeerCertVerificationFunc)

	addr := string(work.Item)
	inspector, err := NewInspector(addr, *IdentityPath)
	if err != nil {
		work.Log.Error("unable to create inspector:",
			zap.String("address", addr),
			zap.Error(err),
		)
		return nil, err
	}


	return &KadWork{
		Work: work,

		Inspector:  inspector,
		VerifyFunc: verifyFunc,
		Graph:      graph,

		Address: string(work.Item),
	}, nil
}

func mapKad(work testqueue.Work) error {
	kadWork, err := newKadwork(work)
	if err != nil {
		work.Log.Error("unable to process kad work:", zap.Error(err))
		return nil
	}

	// lookup self?
	_, seen := kadWork.seenNodes[addr]
	if seen {
		// graph add edge in birectional graphs
		kadWork.mu.Unlock()
		return nil
	}

	dumpNodes(kadWork)
	getNodeIdentities(kadWork)
	graphNode(kadWork)

	return nil
}

func getNodeIdentities(work *KadWork) {
	for {
		var verifyErr error
		kadDialer := kademlia.NewDialer(nil, work.Inspector.transportClient)
		peerIdent, err := kadDialer.FetchPeerIdentity(work.Ctx, *work.Node)
		if err != nil {
			work.Log.Error("unable to fetch peer identity:",
				zap.String("node id", work.Node.Id.String()[:7]),
				zap.Error(err),
			)
		} else {
			verifyErr = work.Verify(nil, [][]*x509.Certificate{append(append([]*x509.Certificate{peerIdent.Leaf}, peerIdent.CA), peerIdent.RestChain...)})
		}

		work.mu.Lock()
		if node, seen := work.seenNodes[work.Addr()]; seen {
			node.ident = peerIdent
			node.verifyErr = verifyErr
		} else {
			work.Log.Error("concurrency error: tried to fetch identity for unseen node")
		}
		work.mu.Unlock()
	}
}

func dumpNodes(work *KadWork) {
	res, err := work.Inspector.kadclient.FindNear(work.Ctx, &pb.FindNearRequest{
		Start: storj.NodeID{},
		Limit: 100000,
	})
	if err != nil {
		work.Log.Error("unable to find near on kad:",
			zap.String("address", work.Addr()),
			zap.Error(err),
		)
	}

	logError := func(msg string, node *pb.Node, err error) {
		if err != nil {
			work.Log.Error(msg,
				zap.String("address", node.Address.Address),
				zap.String("node id", node.Id.String()[:7]),
				zap.Error(err),
			)
		}
	}

	for _, neighbor := range res.Nodes {
		neighborAddr := neighbor.Address.Address

		work.mu.Lock()

		work.seenNodes[work.Addr()] = kadWorkItem{}
		neighborBytes, err := neighbor.XXX_Marshal([]byte{}, true)
		logError("unable to marshal neighbor:", neighbor, err)

		err = work.identityQueue.Enqueue(neighborBytes)
		logError("unable to enqueue in identity queue:", neighbor, err)

		work.mu.Unlock()
		err = work.Queue.Enqueue(storage.Value(neighborAddr))
		logError("unable to enqueue in work queue:", neighbor, err)
	}
}

func graphNode(work *KadWork) {

}

//func walk(ctx context.Context, next string, queue *Queue) error {
func (wq *WorkerQueue) walk(f walkFunc) {
	ctx, cancelWalk := context.WithCancel(wq.ctx)
	group, walkCtx := errgroup.WithContext(ctx)

	// read from queue
	createWorkers(wq.workers, func() error)

	err := group.Wait()
	cancelWalk()

	for _, node := range res.Nodes {
		newAddr := node.Address.Address
		addEdge := func(a, b string) {
			var edge graphs.Edge
			switch {
			//case verifyErr != nil:
			//	edge = dot.NewEdge(next, newAddr, colorUnsigned, "unsigned")
			case err != nil:
				edge = dot.NewEdge(next, newAddr, colorErr, err.Error())
			default:
				edge = dot.NewEdge(next, newAddr, colorSigned, "signed")
			}

			if err := wq.graph.AddEdge(edge); err != nil {
				wq.log.Print(err)
			}
		}

		if _, ok := wq.seen[newAddr]; ok {
			// NB: only applicable for directional graphs
			addEdge(newAddr, next)
			continue
		}

		addEdge(next, newAddr)

		wq.seen[newAddr] = struct{}{}
		wq.addrs = append(wq.addrs, newAddr)

		wq.walk(newAddr)
	}
}
