package main

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storj"
)

type DOTGraph struct {
	name     string
	routeMap map[string][]string
}

var (
	cmdMap = &cobra.Command{
		Use:   "map <bootstrap address>",
		Short: "builds a map of the network by recursively walking routing tables",
		Args:  cobra.ExactArgs(1),
		RunE:  mapCmd,
	}
)

func init() {
	rootCmd.AddCommand(cmdMap)
}

func mapCmd(cmd *cobra.Command, args []string) error {
	if *IdentityPath == "" {
		return ErrArgs.New("--identity-path required")
	}


	ctx, _ := context.WithTimeout(process.Ctx(cmd), 5*time.Second)
	queue := NewQueue(ctx, 10, args[0], walk)

	<-ctx.Done()
	//walk(ctx, queue, storj.NodeID{}, inspector.kadclient)

	dot := buildDot(queue.routes)
	fmt.Println(dot)

	return nil
}

func walk(ctx context.Context, next string, queue *Queue) error {
	inspector, err := NewInspector(next, *IdentityPath)
	if err != nil {
		return err
	}

	res, err := inspector.kadclient.FindNear(ctx, &pb.FindNearRequest{
		Start: storj.NodeID{},
		Limit: 100000,
	})
	if err != nil {
		//fmt.Printf("error finding %s\n", start.String())
		return err
	}

	//var nodeIDs storj.NodeIDList
	var addrs []string
	for _, node := range res.Nodes {
		//nodeIDs = append(nodeIDs, node.Id)
		addrs = append(addrs, node.Address.String())
	}

	route := route{
		addr: next,
		//neighbors: nodeIDs,
		neighbors: addrs,
	}
	queue.Push(route)

	//next := queue.Pop()
	//walk(ctx, queue, next, client)
	return nil
}

func buildDot(routes routes) string {
	dot := NewDot("storj network")

	for _, route := range routes {
		var neighborAddrs []string
		for _, neighborAddr := range route.neighbors {
			neighborAddrs = append(neighborAddrs, neighborAddr)
		}

		dot.Add(route.addr, neighborAddrs)
	}

	return dot.Print()
}

func NewDot(graph string) *DOTGraph {
	return &DOTGraph{
		name: graph,
		routeMap: make(map[string][]string),
	}
}

func (dot *DOTGraph) Add(parent string, children []string) {
	dot.routeMap[parent] = children
}

func (dot *DOTGraph) Print() string {
	out := new(bytes.Buffer)
	out.WriteString(fmt.Sprintf("digraph %s {\n", dot.name))
	return out.String()
}
