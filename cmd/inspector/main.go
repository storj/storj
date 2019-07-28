// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
	prompt "github.com/segmentio/go-prompt"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/kademlia/routinggraph"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/uplink/eestream"
)

var (
	// Addr is the address of peer from command flags
	Addr = flag.String("address", "127.0.0.1:7778", "address of peer to inspect")

	// IdentityPath is the path to the identity the inspector should use for network communication
	IdentityPath = flag.String("identity-path", "", "path to the identity certificate for use on the network")

	// CSVPath is the csv path where command output is written
	CSVPath string

	// ErrInspectorDial throws when there are errors dialing the inspector server
	ErrInspectorDial = errs.Class("error dialing inspector server:")

	// ErrRequest is for gRPC request errors after dialing
	ErrRequest = errs.Class("error processing request:")

	// ErrIdentity is for errors during identity creation for this CLI
	ErrIdentity = errs.Class("error creating identity:")

	// ErrArgs throws when there are errors with CLI args
	ErrArgs = errs.Class("error with CLI args:")

	irreparableLimit int32

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
	healthCmd = &cobra.Command{
		Use:   "health",
		Short: "commands for querying health of a stored data",
	}
	irreparableCmd = &cobra.Command{
		Use:   "irreparable",
		Short: "list segments in irreparable database",
		RunE:  getSegments,
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
	nodeInfoCmd = &cobra.Command{
		Use:   "node-info <node_id>",
		Short: "get node info directly from node",
		Args:  cobra.MinimumNArgs(1),
		RunE:  NodeInfo,
	}
	dumpNodesCmd = &cobra.Command{
		Use:   "dump-nodes",
		Short: "dump all nodes in the routing table",
		RunE:  DumpNodes,
	}
	drawTableCmd = &cobra.Command{
		Use:   "routing-graph",
		Short: "Dumps a graph of the routing table in the dot format",
		RunE:  DrawTableAsGraph,
	}
	objectHealthCmd = &cobra.Command{
		Use:   "object <project-id> <bucket> <encrypted-path>",
		Short: "Get stats about an object's health",
		Args:  cobra.MinimumNArgs(3),
		RunE:  ObjectHealth,
	}
	segmentHealthCmd = &cobra.Command{
		Use:   "segment <project-id> <segment-index> <bucket> <encrypted-path>",
		Short: "Get stats about a segment's health",
		Args:  cobra.MinimumNArgs(4),
		RunE:  SegmentHealth,
	}
)

// Inspector gives access to kademlia, overlay cache
type Inspector struct {
	identity      *identity.FullIdentity
	kadclient     pb.KadInspectorClient
	overlayclient pb.OverlayInspectorClient
	irrdbclient   pb.IrreparableInspectorClient
	healthclient  pb.HealthInspectorClient
}

// NewInspector creates a new gRPC inspector client for access to kad,
// overlay cache
func NewInspector(address, path string) (*Inspector, error) {
	ctx := context.Background()

	id, err := identity.Config{
		CertPath: fmt.Sprintf("%s/identity.cert", path),
		KeyPath:  fmt.Sprintf("%s/identity.key", path),
	}.Load()
	if err != nil {
		return nil, ErrIdentity.Wrap(err)
	}

	conn, err := transport.DialAddressInsecure(ctx, address)
	if err != nil {
		return &Inspector{}, ErrInspectorDial.Wrap(err)
	}

	return &Inspector{
		identity:      id,
		kadclient:     pb.NewKadInspectorClient(conn),
		overlayclient: pb.NewOverlayInspectorClient(conn),
		irrdbclient:   pb.NewIrreparableInspectorClient(conn),
		healthclient:  pb.NewHealthInspectorClient(conn),
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

// NodeInfo get node info directly from the node with provided Node ID
func NodeInfo(cmd *cobra.Command, args []string) (err error) {
	i, err := NewInspector(*Addr, *IdentityPath)
	if err != nil {
		return ErrInspectorDial.Wrap(err)
	}

	// first lookup the node to get its address
	n, err := i.kadclient.LookupNode(context.Background(), &pb.LookupNodeRequest{
		Id: args[0],
	})
	if err != nil {
		return ErrRequest.Wrap(err)
	}

	// now ask the node directly for its node info
	info, err := i.kadclient.NodeInfo(context.Background(), &pb.NodeInfoRequest{
		Id:      n.GetNode().Id,
		Address: n.GetNode().GetAddress(),
	})
	if err != nil {
		return ErrRequest.Wrap(err)
	}

	fmt.Println(prettyPrint(info))

	return nil
}

// DrawTableAsGraph outputs the table routing as a graph
func DrawTableAsGraph(cmd *cobra.Command, args []string) (err error) {
	i, err := NewInspector(*Addr, *IdentityPath)
	if err != nil {
		return ErrInspectorDial.Wrap(err)
	}
	// retrieve buckets
	info, err := i.kadclient.GetBucketList(context.Background(), &pb.GetBucketListRequest{})
	if err != nil {
		return ErrRequest.Wrap(err)
	}

	err = routinggraph.Draw(os.Stdout, info)
	if err != nil {
		return ErrRequest.Wrap(err)
	}

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

// ObjectHealth gets information about the health of an object on the network
func ObjectHealth(cmd *cobra.Command, args []string) (err error) {
	ctx := context.Background()

	i, err := NewInspector(*Addr, *IdentityPath)
	if err != nil {
		return ErrArgs.Wrap(err)
	}

	startAfterSegment := int64(0) // start from first segment
	endBeforeSegment := int64(0)  // No end, so we stop when we've hit limit or arrived at the last segment
	limit := int64(0)             // No limit, so we stop when we've arrived at the last segment

	switch len(args) {
	case 6:
		limit, err = strconv.ParseInt(args[5], 10, 64)
		if err != nil {
			return ErrRequest.Wrap(err)
		}
		fallthrough
	case 5:
		endBeforeSegment, err = strconv.ParseInt(args[4], 10, 64)
		if err != nil {
			return ErrRequest.Wrap(err)
		}
		fallthrough
	case 4:
		startAfterSegment, err = strconv.ParseInt(args[3], 10, 64)
		if err != nil {
			return ErrRequest.Wrap(err)
		}
		fallthrough
	default:
	}

	req := &pb.ObjectHealthRequest{
		ProjectId:         []byte(args[0]),
		Bucket:            []byte(args[1]),
		EncryptedPath:     []byte(args[2]),
		StartAfterSegment: startAfterSegment,
		EndBeforeSegment:  endBeforeSegment,
		Limit:             int32(limit),
	}

	resp, err := i.healthclient.ObjectHealth(ctx, req)
	if err != nil {
		return ErrRequest.Wrap(err)
	}

	f, err := csvOutput()
	if err != nil {
		return err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			fmt.Printf("error closing file: %+v\n", err)
		}
	}()

	w := csv.NewWriter(f)
	defer w.Flush()

	redundancy, err := eestream.NewRedundancyStrategyFromProto(resp.GetRedundancy())
	if err != nil {
		return ErrRequest.Wrap(err)
	}

	if err := printRedundancyTable(w, redundancy); err != nil {
		return err
	}

	if err := printSegmentHealthAndNodeTables(w, redundancy, resp.GetSegments()); err != nil {
		return err
	}

	return nil
}

// SegmentHealth gets information about the health of a segment on the network
func SegmentHealth(cmd *cobra.Command, args []string) (err error) {
	ctx := context.Background()

	i, err := NewInspector(*Addr, *IdentityPath)
	if err != nil {
		return ErrArgs.Wrap(err)
	}

	segmentIndex, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return ErrRequest.Wrap(err)
	}

	req := &pb.SegmentHealthRequest{
		ProjectId:     []byte(args[0]),
		SegmentIndex:  segmentIndex,
		Bucket:        []byte(args[2]),
		EncryptedPath: []byte(args[3]),
	}

	resp, err := i.healthclient.SegmentHealth(ctx, req)
	if err != nil {
		return ErrRequest.Wrap(err)
	}

	f, err := csvOutput()
	if err != nil {
		return err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			fmt.Printf("error closing file: %+v\n", err)
		}
	}()

	w := csv.NewWriter(f)
	defer w.Flush()

	redundancy, err := eestream.NewRedundancyStrategyFromProto(resp.GetRedundancy())
	if err != nil {
		return ErrRequest.Wrap(err)
	}

	if err := printRedundancyTable(w, redundancy); err != nil {
		return err
	}

	if err := printSegmentHealthAndNodeTables(w, redundancy, []*pb.SegmentHealth{resp.GetHealth()}); err != nil {
		return err
	}

	return nil
}

func csvOutput() (*os.File, error) {
	if CSVPath == "stdout" {
		return os.Stdout, nil
	}

	return os.Create(CSVPath)
}

func printSegmentHealthAndNodeTables(w *csv.Writer, redundancy eestream.RedundancyStrategy, segments []*pb.SegmentHealth) error {
	segmentTableHeader := []string{
		"Segment Index", "Healthy Nodes", "Unhealthy Nodes", "Offline Nodes",
	}

	if err := w.Write(segmentTableHeader); err != nil {
		return fmt.Errorf("error writing record to csv: %s", err)
	}

	currentNodeIndex := 1                     // start at index 1 to leave first column empty
	nodeIndices := make(map[storj.NodeID]int) // to keep track of node positions for node table
	// Add each segment to the segmentTable
	for _, segment := range segments {
		healthyNodes := segment.HealthyIds               // healthy nodes with pieces currently online
		unhealthyNodes := segment.UnhealthyIds           // unhealthy nodes with pieces currently online
		offlineNodes := segment.OfflineIds               // offline nodes
		segmentIndexPath := string(segment.GetSegment()) // path formatted Segment Index

		row := []string{
			segmentIndexPath,
			strconv.FormatInt(int64(len(healthyNodes)), 10),
			strconv.FormatInt(int64(len(unhealthyNodes)), 10),
			strconv.FormatInt(int64(len(offlineNodes)), 10),
		}

		if err := w.Write(row); err != nil {
			return fmt.Errorf("error writing record to csv: %s", err)
		}

		allNodes := append(healthyNodes, unhealthyNodes...)
		allNodes = append(allNodes, offlineNodes...)
		for _, id := range allNodes {
			if nodeIndices[id] == 0 {
				nodeIndices[id] = currentNodeIndex
				currentNodeIndex++
			}
		}
	}

	if err := w.Write([]string{}); err != nil {
		return fmt.Errorf("error writing record to csv: %s", err)
	}

	numNodes := len(nodeIndices)
	nodeTableHeader := make([]string, numNodes+1)
	for id, i := range nodeIndices {
		nodeTableHeader[i] = id.String()
	}
	if err := w.Write(nodeTableHeader); err != nil {
		return fmt.Errorf("error writing record to csv: %s", err)
	}

	// Add online/offline info to the node table
	for _, segment := range segments {
		row := make([]string, numNodes+1)
		for _, id := range segment.HealthyIds {
			i := nodeIndices[id]
			row[i] = "healthy"
		}
		for _, id := range segment.UnhealthyIds {
			i := nodeIndices[id]
			row[i] = "unhealthy"
		}
		for _, id := range segment.OfflineIds {
			i := nodeIndices[id]
			row[i] = "offline"
		}
		row[0] = string(segment.GetSegment())
		if err := w.Write(row); err != nil {
			return fmt.Errorf("error writing record to csv: %s", err)
		}
	}

	return nil
}

func printRedundancyTable(w *csv.Writer, redundancy eestream.RedundancyStrategy) error {
	total := redundancy.TotalCount()                  // total amount of pieces we generated (n)
	required := redundancy.RequiredCount()            // minimum required stripes for reconstruction (k)
	optimalThreshold := redundancy.OptimalThreshold() // amount of pieces we need to store to call it a success (o)
	repairThreshold := redundancy.RepairThreshold()   // amount of pieces we need to drop to before triggering repair (m)

	redundancyTable := [][]string{
		{"Total Pieces (n)", "Minimum Required (k)", "Optimal Threshold (o)", "Repair Threshold (m)"},
		{strconv.Itoa(total), strconv.Itoa(required), strconv.Itoa(optimalThreshold), strconv.Itoa(repairThreshold)},
		{},
	}

	for _, row := range redundancyTable {
		if err := w.Write(row); err != nil {
			return fmt.Errorf("error writing record to csv: %s", err)
		}
	}

	return nil
}

func getSegments(cmd *cobra.Command, args []string) error {
	if irreparableLimit <= int32(0) {
		return ErrArgs.New("limit must be greater than 0")
	}

	i, err := NewInspector(*Addr, *IdentityPath)
	if err != nil {
		return ErrInspectorDial.Wrap(err)
	}
	var lastSeenSegmentPath = []byte{}

	// query DB and paginate results
	for {
		req := &pb.ListIrreparableSegmentsRequest{
			Limit:               irreparableLimit,
			LastSeenSegmentPath: lastSeenSegmentPath,
		}
		res, err := i.irrdbclient.ListIrreparableSegments(context.Background(), req)
		if err != nil {
			return ErrRequest.Wrap(err)
		}

		if len(res.Segments) == 0 {
			break
		}
		lastSeenSegmentPath = res.Segments[len(res.Segments)-1].Path

		objects := sortSegments(res.Segments)
		// format and print segments
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		err = enc.Encode(objects)
		if err != nil {
			return err
		}

		length := int32(len(res.Segments))
		if length >= irreparableLimit {
			if !prompt.Confirm("\nNext page? (y/n)") {
				break
			}
		}
	}
	return nil
}

// sortSegments by the object they belong to
func sortSegments(segments []*pb.IrreparableSegment) map[string][]*pb.IrreparableSegment {
	objects := make(map[string][]*pb.IrreparableSegment)
	for _, seg := range segments {
		pathElements := storj.SplitPath(string(seg.Path))

		// by removing the segment index, we can easily sort segments into a map of objects
		pathElements = append(pathElements[:1], pathElements[2:]...)
		objPath := strings.Join(pathElements, "/")
		objects[objPath] = append(objects[objPath], seg)
	}
	return objects
}

func init() {
	rootCmd.AddCommand(kadCmd)
	rootCmd.AddCommand(statsCmd)
	rootCmd.AddCommand(irreparableCmd)
	rootCmd.AddCommand(healthCmd)

	kadCmd.AddCommand(countNodeCmd)
	kadCmd.AddCommand(pingNodeCmd)
	kadCmd.AddCommand(lookupNodeCmd)
	kadCmd.AddCommand(nodeInfoCmd)
	kadCmd.AddCommand(dumpNodesCmd)
	kadCmd.AddCommand(drawTableCmd)

	healthCmd.AddCommand(objectHealthCmd)
	healthCmd.AddCommand(segmentHealthCmd)

	objectHealthCmd.Flags().StringVar(&CSVPath, "csv-path", "stdout", "csv path where command output is written")

	irreparableCmd.Flags().Int32Var(&irreparableLimit, "limit", 50, "max number of results per page")

	flag.Parse()
}

func main() {
	process.Exec(rootCmd)
}
