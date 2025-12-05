// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"storj.io/common/cfgstruct"
	"storj.io/common/fpath"
	"storj.io/common/identity"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/process"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/private/revocation"
	"storj.io/storj/satellite/jobq"
)

// Config holds the toplevel configuration for jobqtool.
type Config struct {
	Identity identity.Config
	Server   string `help:"address of the job queue server" default:"localhost:15781" testDefault:"$HOST:0"`
	TLS      tlsopts.Config
}

// ImportConfig holds the configuration for jobqtool's import subcommand.
type ImportConfig struct {
	Config
	MaxImport int `help:"maximum number of jobs to import in a single batch" default:"1000"`
}

// StatConfig holds the configuration for jobqtool's stat subcommand.
type StatConfig struct {
	Config
	Histogram bool `help:"Stat should include histogram statistic" default:"true"`
}

// PeekConfig holds the configuration for jobqtool's peek subcommand.
type PeekConfig struct {
	Config
	Count int `help:"number of upcoming jobs to inspect" default:"20"`
}

var (
	confDir     string
	identityDir string

	runCfg    Config
	importCfg ImportConfig
	peekCfg   PeekConfig
	statCfg   StatConfig

	rootCmd = &cobra.Command{
		Use:          "jobqtool",
		Short:        "job queue tool",
		SilenceUsage: true,
	}
	lenCmd = &cobra.Command{
		Use:   "len [<placement>]",
		Short: "query lengths of job queues (repair and retry) for the given placement. (If no placement given, sum queues for all placements.)",
		RunE:  lenCommand,
		Args:  cobra.MaximumNArgs(1),
	}
	truncateCmd = &cobra.Command{
		Use:   "truncate <placement>",
		Short: "empty job queues (repair and retry) for the given placement",
		RunE:  truncateCommand,
		Args:  cobra.ExactArgs(1),
	}
	statCmd = &cobra.Command{
		Use:   "stat [<placement>]",
		Short: "query statistics of job queues (repair and retry) for the given placement. (If no placement given, query all placements.)",
		RunE:  statCommand,
		Args:  cobra.MaximumNArgs(1),
	}
	peekCmd = &cobra.Command{
		Use:   "peek [<placement>]",
		Short: "observe upcoming jobs in the queue for the given placement. Do not remove them from the queue(s). (If no placement given, get jobs from any placement.)",
		RunE:  peekCommand,
		Args:  cobra.MaximumNArgs(1),
	}
	importCmd = &cobra.Command{
		Use:   "import <file>",
		Short: "import jobs from a CSV file (format: <placement>,<streamID>,<position>,<segment_health>[,<last_attempted_at>])",
		RunE:  importCommand,
		Args:  cobra.ExactArgs(1),
	}
	cleanCmd = &cobra.Command{
		Use:   "clean <timestamp> [<placement>]",
		Short: "remove all jobs older than the given timestamp (given as relative duration '-24h') or ISO 8601. (If no placement given, clean queues for all placements.)",
		RunE:  cleanCommand,
		Args:  cobra.RangeArgs(1, 2),
	}
	trimCmd = &cobra.Command{
		Use:   "trim <healthThreshold> [<placement>]",
		Short: "remove all jobs with health above the given threshold from the queue for the given placement. (If no placement given, trim all placements.)",
		RunE:  trimCommand,
		Args:  cobra.RangeArgs(1, 2),
	}
)

func init() {
	defaultConfDir := fpath.ApplicationDir("storj", "jobqtool")
	defaultIdentityDir := fpath.ApplicationDir("storj", "identity", "jobqtool")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &confDir, "config-dir", defaultConfDir, "main directory for jobqtool configuration")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &identityDir, "identity-dir", defaultIdentityDir, "main directory for jobqtool identity credentials")
	defaults := cfgstruct.DefaultsFlag(rootCmd)
	process.Bind(rootCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(lenCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(truncateCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(statCmd, &statCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(peekCmd, &peekCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(importCmd, &importCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(cleanCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(trimCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	rootCmd.AddCommand(lenCmd)
	rootCmd.AddCommand(truncateCmd)
	rootCmd.AddCommand(statCmd)
	rootCmd.AddCommand(peekCmd)
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(trimCmd)
}

func prepareConnection(ctx context.Context, cfg Config) (*jobq.Client, error) {
	identity, err := cfg.Identity.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load identity: %w", err)
	}
	revocationDB, err := revocation.OpenDBFromCfg(ctx, cfg.TLS)
	if err != nil {
		return nil, fmt.Errorf("creating revocation database: %w", err)
	}
	tlsOpts, err := tlsopts.NewOptions(identity, cfg.TLS, revocationDB)
	if err != nil {
		return nil, fmt.Errorf("TLS options: %w", err)
	}

	serverNodeURL, err := storj.ParseNodeURL(cfg.Server)
	if err != nil {
		return nil, fmt.Errorf("invalid server URL %q: %w", cfg.Server, err)
	}
	dialer := jobq.NewDialer(tlsOpts)

	conn, err := dialer.DialNodeURL(ctx, serverNodeURL)
	if err != nil {
		return nil, err
	}

	return jobq.WrapConn(conn), nil
}

func lenCommand(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	drpcConn, err := prepareConnection(ctx, runCfg)
	if err != nil {
		return err
	}
	defer func() { _ = drpcConn.Close() }()

	var repairLen, retryLen int64
	if len(args) > 0 {
		placement, err := strconv.ParseInt(args[0], 10, 16)
		if err != nil {
			return fmt.Errorf("invalid placement %q: %w", args[0], err)
		}
		repairLen, retryLen, err = drpcConn.Len(ctx, storj.PlacementConstraint(placement))
		if err != nil {
			return fmt.Errorf("querying queue length: %w", err)
		}
	} else {
		repairLen, retryLen, err = drpcConn.LenAll(ctx)
		if err != nil {
			return fmt.Errorf("querying queue lengths: %w", err)
		}
	}
	fmt.Printf("repair %d\nretry %d\n", repairLen, retryLen)
	return nil
}

func truncateCommand(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	drpcConn, err := prepareConnection(ctx, runCfg)
	if err != nil {
		return err
	}
	defer func() { _ = drpcConn.Close() }()

	placement, err := strconv.ParseInt(args[0], 10, 16)
	if err != nil {
		return fmt.Errorf("invalid placement %q: %w", args[0], err)
	}
	err = drpcConn.Truncate(ctx, storj.PlacementConstraint(placement))
	if err != nil {
		return fmt.Errorf("failed to truncate queue: %w", err)
	}
	return nil
}

func statCommand(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	drpcConn, err := prepareConnection(ctx, statCfg.Config)
	if err != nil {
		return err
	}
	defer func() { _ = drpcConn.Close() }()

	var jobStats []jobq.QueueStat
	if len(args) > 0 {
		placement, err := strconv.ParseInt(args[0], 10, 16)
		if err != nil {
			return fmt.Errorf("invalid placement %q: %w", args[0], err)
		}
		jobStat, err := drpcConn.Stat(ctx, storj.PlacementConstraint(placement), statCfg.Histogram)
		if err != nil {
			return fmt.Errorf("querying queue statistics: %w", err)
		}
		jobStats = []jobq.QueueStat{jobStat}
	} else {
		jobStats, err = drpcConn.StatAll(ctx, statCfg.Histogram)
		if err != nil {
			return fmt.Errorf("querying all queue statistics: %w", err)
		}
	}
	outputJobs := false
	for _, stat := range jobStats {
		if stat.Count == 0 {
			continue
		}
		outputJobs = true
		if stat.MinAttemptedAt == nil {
			fmt.Printf("placement %d [repair]\n", stat.Placement)
		} else {
			fmt.Printf("placement %d [waiting for retry]\n", stat.Placement)
		}
		fmt.Printf("  count %d\n", stat.Count)
		fmt.Printf("  min inserted at %v\n", stat.MinInsertedAt)
		fmt.Printf("  max inserted at %v\n", stat.MaxInsertedAt)
		if stat.MinAttemptedAt != nil {
			fmt.Printf("  min attempted at %v\n", stat.MinAttemptedAt)
			fmt.Printf("  max attempted at %v\n", stat.MaxAttemptedAt)
		}
		fmt.Printf("  min segment health: %.6f\n", stat.MinSegmentHealth)
		fmt.Printf("  max segment health: %.6f\n", stat.MaxSegmentHealth)
		for _, h := range stat.Histogram {
			fmt.Println("    count:", h.Count, ", healthy:", h.NumNormalizedHealthy, ", retrievable:", h.NumNormalizedRetrievable, ", oop:", h.NumOutOfPlacement, ", exemplar:", h.Exemplar)
		}
	}
	if !outputJobs {
		fmt.Println("(no jobs)")
	}
	return nil
}

func importCommand(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	drpcConn, err := prepareConnection(ctx, importCfg.Config)
	if err != nil {
		return err
	}
	defer func() { _ = drpcConn.Close() }()

	inputFile, err := os.Open(args[0])
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = inputFile.Close() }()

	csvReader := csv.NewReader(inputFile)
	jobs := []jobq.RepairJob{}
	totalPushed := 0
	totalNew := 0

	for {
		record, err := csvReader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("failed to read CSV record: %w", err)
		}
		if record[0] == "placement" && len(jobs) == 0 {
			// it's a header; skip it
			continue
		}

		if len(record) < 4 || len(record) > 5 {
			return fmt.Errorf("invalid CSV record: %q", record)
		}

		placement, err := strconv.ParseInt(record[0], 10, 16)
		if err != nil {
			return fmt.Errorf("invalid placement %q: %w", record[0], err)
		}
		streamID, err := uuid.FromString(record[1])
		if err != nil {
			return fmt.Errorf("could not parse stream ID %q: %w", record[1], err)
		}
		position, err := strconv.ParseUint(record[2], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid position %q: %w", record[2], err)
		}
		segmentHealth, err := strconv.ParseFloat(record[3], 64)
		if err != nil {
			return fmt.Errorf("invalid segment health %q: %w", record[3], err)
		}
		var lastAttemptedAt time.Time
		if len(record) > 4 {
			lastAttemptedAt, err = time.Parse(time.RFC3339, record[4])
			if err != nil {
				return fmt.Errorf("invalid last attempted at %q: %w", record[4], err)
			}
		}

		jobs = append(jobs, jobq.RepairJob{
			ID:              jobq.SegmentIdentifier{StreamID: streamID, Position: position},
			Placement:       uint16(placement),
			Health:          segmentHealth,
			LastAttemptedAt: uint64(lastAttemptedAt.Unix()),
		})

		if len(jobs) == importCfg.MaxImport {
			wasNew, err := drpcConn.PushBatch(ctx, jobs)
			if err != nil {
				return fmt.Errorf("failed to import jobs: %w", err)
			}
			totalPushed += len(jobs)
			totalNew += count(wasNew)
			jobs = jobs[:0]
		}
	}

	if len(jobs) > 0 {
		wasNew, err := drpcConn.PushBatch(ctx, jobs)
		if err != nil {
			return fmt.Errorf("failed to import jobs: %w", err)
		}
		totalPushed += len(jobs)
		totalNew += count(wasNew)
	}
	fmt.Printf("imported %d jobs (%d were new, %d updated)\n", totalPushed, totalNew, totalPushed-totalNew)

	return nil
}

func count(bools []bool) int {
	var count int
	for _, b := range bools {
		if b {
			count++
		}
	}
	return count
}

func peekCommand(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	drpcConn, err := prepareConnection(ctx, peekCfg.Config)
	if err != nil {
		return err
	}
	defer func() { _ = drpcConn.Close() }()

	var jobs []jobq.RepairJob
	if len(args) > 0 {
		var placement int64
		placement, err = strconv.ParseInt(args[0], 10, 16)
		if err != nil {
			return fmt.Errorf("invalid placement %q: %w", args[0], err)
		}
		jobs, err = drpcConn.Peek(ctx, peekCfg.Count, []storj.PlacementConstraint{storj.PlacementConstraint(placement)}, nil)
	} else {
		jobs, err = drpcConn.Peek(ctx, peekCfg.Count, nil, nil)
	}
	if err != nil {
		return fmt.Errorf("failed to peek upcoming jobs: %w", err)
	}

	fmt.Printf("%-5s | %-36s | %-20s | %-20s | %-20s | %s\n", "plcmt", "streamID", "position", "inserted_at", "attempted_at", "health")
	for _, job := range jobs {
		insertTime := time.Unix(int64(job.InsertedAt), 0).UTC().Format(time.RFC3339)
		attemptTime := time.Unix(int64(job.LastAttemptedAt), 0).UTC().Format(time.RFC3339)
		fmt.Printf("%5d | %36s | %20d | %20s | %20s | %f\n", job.Placement, job.ID.StreamID, job.ID.Position, insertTime, attemptTime, job.Health)
	}
	return nil
}

func cleanCommand(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	drpcConn, err := prepareConnection(ctx, runCfg)
	if err != nil {
		return err
	}
	defer func() { _ = drpcConn.Close() }()

	var timestamp time.Time
	timestampStr := args[0]
	if timestampStr[0] == '-' {
		duration, err := time.ParseDuration(timestampStr)
		if err != nil {
			return fmt.Errorf("invalid relative duration %q: %w", timestampStr, err)
		}
		timestamp = time.Now().Add(duration)
	} else {
		timeFormats := []string{"2006-01-02T15:04:05Z0700", "2006-01-02 15:04:05Z0700", "2006-01-02"}
		for _, format := range timeFormats {
			t, err := time.Parse(format, timestampStr)
			if err == nil {
				timestamp = t
				break
			}
		}
		if timestamp.IsZero() {
			return fmt.Errorf("could not parse timestamp %q", timestampStr)
		}
	}

	var removed int32
	if len(args) > 1 {
		var placement int64
		placement, err = strconv.ParseInt(args[1], 10, 16)
		if err != nil {
			return fmt.Errorf("invalid placement %q: %w", args[1], err)
		}
		removed, err = drpcConn.Clean(ctx, storj.PlacementConstraint(placement), timestamp)
	} else {
		removed, err = drpcConn.CleanAll(ctx, timestamp)
	}
	if err != nil {
		return fmt.Errorf("failed to clean: %w", err)
	}
	fmt.Printf("removed %d jobs\n", removed)
	return nil
}

func trimCommand(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	drpcConn, err := prepareConnection(ctx, runCfg)
	if err != nil {
		return err
	}
	defer func() { _ = drpcConn.Close() }()

	healthThreshold, err := strconv.ParseFloat(args[0], 64)
	if err != nil {
		return fmt.Errorf("could not parse health threshold %q: %w", args[0], err)
	}

	var removed int32
	if len(args) > 1 {
		var placement int64
		placement, err = strconv.ParseInt(args[1], 10, 16)
		if err != nil {
			return fmt.Errorf("invalid placement %q: %w", args[1], err)
		}
		removed, err = drpcConn.Trim(ctx, storj.PlacementConstraint(placement), healthThreshold)
	} else {
		removed, err = drpcConn.TrimAll(ctx, healthThreshold)
	}
	if err != nil {
		return fmt.Errorf("failed to trim: %w", err)
	}
	fmt.Printf("removed %d jobs\n", removed)
	return nil
}

func main() {
	logger, atomicLevel, _ := process.NewLogger("jobqtool")
	atomicLevel.SetLevel(zap.WarnLevel)
	zap.ReplaceGlobals(logger)

	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %+v\n", err)
		os.Exit(1)
	}
}
