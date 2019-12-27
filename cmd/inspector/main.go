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
	"time"

	prompt "github.com/segmentio/go-prompt"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/storj/pkg/process"
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
		Short: "CLI for interacting with Storj network",
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
	paymentsCmd = &cobra.Command{
		Use:   "payments",
		Short: "commands for payments",
	}
	prepareInvoiceRecordsCmd = &cobra.Command{
		Use:   "prepare-invoice-records <period>",
		Short: "Prepares invoice project records that will be used during invoice line items creation",
		Args:  cobra.MinimumNArgs(1),
		RunE:  prepareInvoiceRecords,
	}
	createInvoiceItemsCmd = &cobra.Command{
		Use:   "create-invoice-items",
		Short: "Creates stripe invoice line items for not consumed project records",
		RunE:  createInvoiceItems,
	}
	createInvoicesCmd = &cobra.Command{
		Use:   "create-invoices",
		Short: "Creates stripe invoices for all stripe customers known to satellite",
		RunE:  createInvoices,
	}
)

// Inspector gives access to overlay.
type Inspector struct {
	conn           *rpc.Conn
	identity       *identity.FullIdentity
	overlayclient  pb.DRPCOverlayInspectorClient
	irrdbclient    pb.DRPCIrreparableInspectorClient
	healthclient   pb.DRPCHealthInspectorClient
	paymentsClient pb.DRPCPaymentsClient
}

// NewInspector creates a new gRPC inspector client for access to overlay.
func NewInspector(address, path string) (*Inspector, error) {
	ctx := context.Background()

	id, err := identity.Config{
		CertPath: fmt.Sprintf("%s/identity.cert", path),
		KeyPath:  fmt.Sprintf("%s/identity.key", path),
	}.Load()
	if err != nil {
		return nil, ErrIdentity.Wrap(err)
	}

	conn, err := rpc.NewDefaultDialer(nil).DialAddressUnencrypted(ctx, address)
	if err != nil {
		return &Inspector{}, ErrInspectorDial.Wrap(err)
	}

	return &Inspector{
		conn:           conn,
		identity:       id,
		overlayclient:  pb.NewDRPCOverlayInspectorClient(conn.Raw()),
		irrdbclient:    pb.NewDRPCIrreparableInspectorClient(conn.Raw()),
		healthclient:   pb.NewDRPCHealthInspectorClient(conn.Raw()),
		paymentsClient: pb.NewDRPCPaymentsClient(conn.Raw()),
	}, nil
}

// Close closes the inspector.
func (i *Inspector) Close() error { return i.conn.Close() }

// ObjectHealth gets information about the health of an object on the network
func ObjectHealth(cmd *cobra.Command, args []string) (err error) {
	ctx := context.Background()

	i, err := NewInspector(*Addr, *IdentityPath)
	if err != nil {
		return ErrArgs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, i.Close()) }()

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
	defer func() { err = errs.Combine(err, i.Close()) }()

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
	defer func() { err = errs.Combine(err, i.Close()) }()

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

func prepareInvoiceRecords(cmd *cobra.Command, args []string) error {
	i, err := NewInspector(*Addr, *IdentityPath)
	if err != nil {
		return ErrInspectorDial.Wrap(err)
	}

	defer func() { err = errs.Combine(err, i.Close()) }()

	period, err := parseDateString(args[0])
	if err != nil {
		return ErrArgs.New("invalid period specified: %v", err)
	}

	_, err = i.paymentsClient.PrepareInvoiceRecords(context.Background(),
		&pb.PrepareInvoiceRecordsRequest{
			Period: period,
		},
	)
	if err != nil {
		return err
	}

	fmt.Println("successfully created invoice project records")
	return nil
}

func createInvoiceItems(cmd *cobra.Command, args []string) error {
	i, err := NewInspector(*Addr, *IdentityPath)
	if err != nil {
		return ErrInspectorDial.Wrap(err)
	}

	defer func() { err = errs.Combine(err, i.Close()) }()

	_, err = i.paymentsClient.ApplyInvoiceRecords(context.Background(), &pb.ApplyInvoiceRecordsRequest{})
	if err != nil {
		return err
	}

	fmt.Println("successfully created invoice line items")
	return nil
}

func createInvoices(cmd *cobra.Command, args []string) error {
	i, err := NewInspector(*Addr, *IdentityPath)
	if err != nil {
		return ErrInspectorDial.Wrap(err)
	}

	defer func() { err = errs.Combine(err, i.Close()) }()

	_, err = i.paymentsClient.CreateInvoices(context.Background(), &pb.CreateInvoicesRequest{})
	if err != nil {
		return err
	}

	fmt.Println("successfully created invoices")
	return nil
}

// parseDateString parses provided date string and returns corresponding time.Time.
func parseDateString(s string) (time.Time, error) {
	values := strings.Split(s, "/")

	if len(values) != 2 {
		return time.Time{}, errs.New("invalid date format %s, use mm/yyyy", s)
	}

	month, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil {
		return time.Time{}, errs.New("can not parse month: %v", err)
	}
	year, err := strconv.ParseInt(values[1], 10, 64)
	if err != nil {
		return time.Time{}, errs.New("can not parse year: %v", err)
	}

	date := time.Date(int(year), time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	if date.Year() != int(year) || date.Month() != time.Month(month) || date.Day() != 1 {
		return date, errs.New("dates mismatch have %s result %s", s, date)
	}

	return date, nil
}

func init() {
	rootCmd.AddCommand(statsCmd)
	rootCmd.AddCommand(irreparableCmd)
	rootCmd.AddCommand(healthCmd)
	rootCmd.AddCommand(paymentsCmd)

	healthCmd.AddCommand(objectHealthCmd)
	healthCmd.AddCommand(segmentHealthCmd)

	paymentsCmd.AddCommand(prepareInvoiceRecordsCmd)
	paymentsCmd.AddCommand(createInvoiceItemsCmd)
	paymentsCmd.AddCommand(createInvoicesCmd)

	objectHealthCmd.Flags().StringVar(&CSVPath, "csv-path", "stdout", "csv path where command output is written")

	irreparableCmd.Flags().Int32Var(&irreparableLimit, "limit", 50, "max number of results per page")

	flag.Parse()
}

func main() {
	process.Exec(rootCmd)
}
