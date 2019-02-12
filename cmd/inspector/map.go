package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"os"
	"storj.io/storj/pkg/process"
	"text/tabwriter"
	"time"

	"storj.io/storj/pkg/pb"
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

type network struct{
	ctx context.Context
	addrs []string
	seen map[string]struct{}
	//seen map[string]int
	out *tabwriter.Writer
}

func mapCmd(cmd *cobra.Command, args []string) error {
	if *IdentityPath == "" {
		return ErrArgs.New("--identity-path required")
	}

	ctx, _ := context.WithTimeout(process.Ctx(cmd), 5*time.Second)
	n := &network{
		ctx: ctx,
		//seen: map[string]int{args[0]: 0},
		seen: map[string]struct{}{args[0]: {}},
		addrs: []string{args[0]},
		out: tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0),
	}
	//_, _ = fmt.Fprintf(n.out, "Address\tRoute count\n")
	_,_ = fmt.Fprintf(n.out, "digraph StorjNetwork {\n")
	if err := n.walk(args[0]); err != nil {
		fmt.Printf("error: %s\n", err.Error())
	}
	_, _ = fmt.Fprintf(n.out, "}\n")
	_ = n.out.Flush()

	//dot := buildDot(queue.routes)
	//fmt.Println(dot)

	return nil
}

//func walk(ctx context.Context, next string, queue *Queue) error {
func (n *network) walk(next string) error {
	clientErrs := errs.Group{}
	select {
	case <-n.ctx.Done():
		return nil
	default:
		inspector, err := NewInspector(next, *IdentityPath)
		if err != nil {
			//return err
			return nil
		}

		inspector.kadclient.

		res, err := inspector.kadclient.FindNear(n.ctx, &pb.FindNearRequest{
			Start: storj.NodeID{},
			Limit: 100000,
		})
		if err != nil {
			//return err
			return nil
		}

		//nextIndex := n.seen[next]
		//_, _ = fmt.Fprintf(n.out, "%s\t%d\n", next, len(res.Nodes))
		for _, node := range res.Nodes {
			newAddr := node.Address.Address

			if _, ok := n.seen[newAddr]; ok {
				//_, _ = fmt.Fprintf(n.out, "\"%d\"\t->\t\"%d\"\n", nextIndex, newIndex)
				//_, _ = fmt.Fprintf(n.out, "\"%s\"\t->\t\"%s\"\n", next, newAddr)
				continue
			}

			//newIndex := len(n.addrs)

			//_, _ = fmt.Fprintf(n.out, "\"%d\" [fillcolor=\"#2683ff\" fontcolor=\"white\" style=filled]\n", newIndex)
			_, _ = fmt.Fprintf(n.out, "\"%s\" [fillcolor=\"#2683ff\" fontcolor=\"white\" style=filled]\n", newAddr)
			//_, _ = fmt.Fprintf(n.out, "\"%d\"\t->\t\"%d\"\n", nextIndex, newIndex)
			_, _ = fmt.Fprintf(n.out, "\"%s\"\t->\t\"%s\"\n", next, newAddr)

			//n.seen[newAddr] = newIndex
			n.seen[newAddr] = struct{}{}
			n.addrs = append(n.addrs, newAddr)

			if err := n.walk(newAddr); err != nil {
				clientErrs.Add(err)
			}
		}
	}

	return clientErrs.Err()
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
		name:     graph,
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
