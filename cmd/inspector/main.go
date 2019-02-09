// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
)

var (
	// Addr is the address of peer from command flags
	Addr = flag.String("address", "localhost:7778", "address of peer to inspect")

	// IdentityPath is the path to the identity the inspector should use for network communication
	IdentityPath = flag.String("identity-path", "", "path to the identity certificate for use on the network")

	// ErrInspectorDial throws when there are errors dialing the inspector server
	ErrInspectorDial = errs.Class("error dialing inspector server:")

	// ErrRequest is for gRPC request errors after dialing
	ErrRequest = errs.Class("error processing request:")

	// ErrIdentity is for errors during identity creation for this CLI
	ErrIdentity = errs.Class("error creating identity:")

	// ErrArgs throws when there are errors with CLI args
	ErrArgs = errs.Class("error with CLI args:")

	// Commander CLI
	rootCmd = &cobra.Command{
		Use:   "inspector",
		Short: "CLI for interacting with Storj Kademlia network",
	}
	kadCmd = &cobra.Command{
		Use:   "kad",
		Short: "commands for kademlia/overlay cache",
	}
	statsCmd = &cobra.Command{
		Use:   "statdb",
		Short: "commands for statdb",
	}
	countNodeCmd = &cobra.Command{
		Use:   "count",
		Short: "count nodes in kademlia and overlay",
		RunE:  CountNodes,
	}
	pingNodeCmd = &cobra.Command{
		Use:   "ping <node_id> <ip:port>",
		Short: "ping node at provided ID",
		Args:  cobra.MinimumNArgs(2),
		RunE:  PingNode,
	}
	lookupNodeCmd = &cobra.Command{
		Use:   "lookup <node_id>",
		Short: "lookup a node by ID only",
		Args:  cobra.MinimumNArgs(1),
		RunE:  LookupNode,
	}
	dumpNodesCmd = &cobra.Command{
		Use:   "dump-nodes",
		Short: "dump all nodes in the routing table",
		RunE:  DumpNodes,
	}
	getStatsCmd = &cobra.Command{
		Use:   "getstats <node_id>",
		Short: "Get node stats",
		Args:  cobra.MinimumNArgs(1),
		RunE:  GetStats,
	}
	getCSVStatsCmd = &cobra.Command{
		Use:   "getcsvstats <path to node ID csv file>",
		Short: "Get node stats from csv",
		Args:  cobra.MinimumNArgs(1),
		RunE:  GetCSVStats,
	}
	createStatsCmd = &cobra.Command{
		// TODO: add args to usage
		Use:   "createstats",
		Short: "Create node with stats",
		Args:  cobra.MinimumNArgs(5), // id, auditct, auditsuccessct, uptimect, uptimesuccessct
		RunE:  CreateStats,
	}
	createCSVStatsCmd = &cobra.Command{
		// TODO: add args to usage
		Use:   "createcsvstats",
		Short: "Create node stats from csv",
		Args:  cobra.MinimumNArgs(1),
		RunE:  CreateCSVStats,
	}
)

// Inspector gives access to kademlia and overlay cache
type Inspector struct {
	identity      *identity.FullIdentity
	kadclient     pb.KadInspectorClient
	overlayclient pb.OverlayInspectorClient
	statdbclient  pb.StatDBInspectorClient
}

// NewInspector creates a new gRPC inspector server for access to kad
// and the overlay cache
func NewInspector(address, path string) (*Inspector, error) {
	ctx := context.Background()

	id, err := identity.Config{
		CertPath: fmt.Sprintf("%s/identity.cert", path),
		KeyPath:  fmt.Sprintf("%s/identity.key", path),
	}.Load()
	if err != nil {
		return nil, ErrIdentity.Wrap(err)
	}

	tlsOpts, err := tlsopts.NewOptions(id, tlsopts.Config{})
	if err != nil {
		return nil, err
	}

	tc := transport.NewClient(tlsOpts)
	conn, err := tc.DialAddress(ctx, address)
	if err != nil {
		return &Inspector{}, ErrInspectorDial.Wrap(err)
	}

	return &Inspector{
		identity:      id,
		kadclient:     pb.NewKadInspectorClient(conn),
		overlayclient: pb.NewOverlayInspectorClient(conn),
		statdbclient:  pb.NewStatDBInspectorClient(conn),
	}, nil
}

// CountNodes returns the number of nodes in kademlia
func CountNodes(cmd *cobra.Command, args []string) (err error) {
	i, err := NewInspector(*Addr, *IdentityPath)
	if err != nil {
		return ErrInspectorDial.Wrap(err)
	}

	kadcount, err := i.kadclient.CountNodes(context.Background(), &pb.CountNodesRequest{})
	if err != nil {
		return ErrRequest.Wrap(err)
	}

	fmt.Printf("Kademlia node count: %+v\n", kadcount.Count)
	return nil
}

// LookupNode starts a Kademlia lookup for the provided Node ID
func LookupNode(cmd *cobra.Command, args []string) (err error) {
	i, err := NewInspector(*Addr, *IdentityPath)
	if err != nil {
		return ErrInspectorDial.Wrap(err)
	}

	n, err := i.kadclient.LookupNode(context.Background(), &pb.LookupNodeRequest{
		Id: args[0],
	})

	if err != nil {
		return ErrRequest.Wrap(err)
	}

	fmt.Println(prettyPrint(n))

	return nil
}

// DumpNodes outputs a json list of every node in every bucket in the satellite
func DumpNodes(cmd *cobra.Command, args []string) (err error) {
	i, err := NewInspector(*Addr, *IdentityPath)
	if err != nil {
		return ErrInspectorDial.Wrap(err)
	}

	nodes, err := i.kadclient.FindNear(context.Background(), &pb.FindNearRequest{
		Start: storj.NodeID{},
		Limit: 100000,
	})
	if err != nil {
		return err
	}

	fmt.Println(prettyPrint(nodes))

	return nil
}

func prettyPrint(unformatted proto.Message) string {
	m := jsonpb.Marshaler{Indent: "  ", EmitDefaults: true}
	formatted, err := m.MarshalToString(unformatted)
	if err != nil {
		fmt.Println("Error", err)
		os.Exit(1)
	}
	return formatted
}

// PingNode sends a PING RPC across the Kad network to check node availability
func PingNode(cmd *cobra.Command, args []string) (err error) {
	nodeID, err := storj.NodeIDFromString(args[0])
	if err != nil {
		return err
	}

	i, err := NewInspector(*Addr, *IdentityPath)
	if err != nil {
		return ErrInspectorDial.Wrap(err)
	}

	fmt.Printf("Pinging node %s at %s", args[0], args[1])

	p, err := i.kadclient.PingNode(context.Background(), &pb.PingNodeRequest{
		Id:      nodeID,
		Address: args[1],
	})

	var okayString string
	if p != nil && p.Ok {
		okayString = "OK"
	} else {
		okayString = "Error"
	}
	fmt.Printf("\n -- Ping response: %s\n", okayString)
	if err != nil {
		fmt.Printf(" -- Error: %v\n", err)
	}
	return nil
}

// GetStats gets a node's stats from statdb
func GetStats(cmd *cobra.Command, args []string) (err error) {
	i, err := NewInspector(*Addr, *IdentityPath)
	if err != nil {
		return ErrInspectorDial.Wrap(err)
	}

	nodeID, err := storj.NodeIDFromString(args[0])
	if err != nil {
		return err
	}

	res, err := i.statdbclient.GetStats(context.Background(), &pb.GetStatsRequest{
		NodeId: nodeID,
	})
	if err != nil {
		return ErrRequest.Wrap(err)
	}

	fmt.Printf("Stats for ID %s:\n", nodeID)
	fmt.Printf("AuditSuccessRatio: %f, AuditCount: %d, UptimeRatio: %f, UptimeCount: %d,\n",
		res.AuditRatio, res.AuditCount, res.UptimeRatio, res.UptimeCount)
	return nil
}

// GetCSVStats gets node stats from statdb based on a csv
func GetCSVStats(cmd *cobra.Command, args []string) (err error) {
	i, err := NewInspector(*Addr, *IdentityPath)
	if err != nil {
		return ErrInspectorDial.Wrap(err)
	}

	// get csv
	csvPath := args[0]
	csvFile, _ := os.Open(csvPath)
	reader := csv.NewReader(bufio.NewReader(csvFile))
	for {
		line, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return ErrArgs.Wrap(err)
		}

		nodeID, err := storj.NodeIDFromString(line[0])
		if err != nil {
			return err
		}
		res, err := i.statdbclient.GetStats(context.Background(), &pb.GetStatsRequest{
			NodeId: nodeID,
		})
		if err != nil {
			return ErrRequest.Wrap(err)
		}

		fmt.Printf("Stats for ID %s:\n", nodeID)
		fmt.Printf("AuditSuccessRatio: %f, AuditCount: %d, UptimeRatio: %f, UptimeCount: %d,\n",
			res.AuditRatio, res.AuditCount, res.UptimeRatio, res.UptimeCount)
	}
	return nil
}

// CreateStats creates a node with stats in statdb
func CreateStats(cmd *cobra.Command, args []string) (err error) {
	i, err := NewInspector(*Addr, *IdentityPath)
	if err != nil {
		return ErrInspectorDial.Wrap(err)
	}

	nodeID, err := storj.NodeIDFromString(args[0])
	if err != nil {
		return err
	}
	auditCount, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return ErrArgs.New("audit count must be an int")
	}
	auditSuccessCount, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		return ErrArgs.New("audit success count must be an int")
	}
	uptimeCount, err := strconv.ParseInt(args[3], 10, 64)
	if err != nil {
		return ErrArgs.New("uptime count must be an int")
	}
	uptimeSuccessCount, err := strconv.ParseInt(args[4], 10, 64)
	if err != nil {
		return ErrArgs.New("uptime success count must be an int")
	}

	_, err = i.statdbclient.CreateStats(context.Background(), &pb.CreateStatsRequest{
		NodeId:             nodeID,
		AuditCount:         auditCount,
		AuditSuccessCount:  auditSuccessCount,
		UptimeCount:        uptimeCount,
		UptimeSuccessCount: uptimeSuccessCount,
	})
	if err != nil {
		return ErrRequest.Wrap(err)
	}

	fmt.Printf("Created statdb entry for ID %s\n", nodeID)
	return nil
}

// CreateCSVStats creates node with stats in statdb based on a CSV
func CreateCSVStats(cmd *cobra.Command, args []string) (err error) {
	i, err := NewInspector(*Addr, *IdentityPath)
	if err != nil {
		return ErrInspectorDial.Wrap(err)
	}

	// get csv
	csvPath := args[0]
	csvFile, _ := os.Open(csvPath)
	reader := csv.NewReader(bufio.NewReader(csvFile))
	for {
		line, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return ErrArgs.Wrap(err)
		}

		nodeID, err := storj.NodeIDFromString(line[0])
		if err != nil {
			return err
		}
		auditCount, err := strconv.ParseInt(line[1], 10, 64)
		if err != nil {
			return ErrArgs.New("audit count must be an int")
		}
		auditSuccessCount, err := strconv.ParseInt(line[2], 10, 64)
		if err != nil {
			return ErrArgs.New("audit success count must be an int")
		}
		uptimeCount, err := strconv.ParseInt(line[3], 10, 64)
		if err != nil {
			return ErrArgs.New("uptime count must be an int")
		}
		uptimeSuccessCount, err := strconv.ParseInt(line[4], 10, 64)
		if err != nil {
			return ErrArgs.New("uptime success count must be an int")
		}

		_, err = i.statdbclient.CreateStats(context.Background(), &pb.CreateStatsRequest{
			NodeId:             nodeID,
			AuditCount:         auditCount,
			AuditSuccessCount:  auditSuccessCount,
			UptimeCount:        uptimeCount,
			UptimeSuccessCount: uptimeSuccessCount,
		})
		if err != nil {
			return ErrRequest.Wrap(err)
		}

		fmt.Printf("Created statdb entry for ID %s\n", nodeID)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(kadCmd)
	rootCmd.AddCommand(statsCmd)

	kadCmd.AddCommand(countNodeCmd)
	kadCmd.AddCommand(pingNodeCmd)
	kadCmd.AddCommand(lookupNodeCmd)
	kadCmd.AddCommand(dumpNodesCmd)

	statsCmd.AddCommand(getStatsCmd)
	statsCmd.AddCommand(getCSVStatsCmd)
	statsCmd.AddCommand(createStatsCmd)
	statsCmd.AddCommand(createCSVStatsCmd)

	flag.Parse()
}

func main() {
	process.Exec(rootCmd)
}
