// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"

	"storj.io/common/cfgstruct"
	"storj.io/common/fpath"
	"storj.io/common/identity"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/process"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/storj/multinode"
	"storj.io/storj/multinode/multinodedb"
	"storj.io/storj/multinode/nodes"
	"storj.io/storj/private/multinodeauth"
	_ "storj.io/storj/web/multinode" // This embeds multinode assets.
)

// Config defines multinode configuration.
type Config struct {
	Database string `help:"multinode database connection string" default:"sqlite3://file:$CONFDIR/master.db"`

	multinode.Config
}

var (
	rootCmd = &cobra.Command{
		Use:   "multinode",
		Short: "Multinode Dashboard",
	}
	runCmd = &cobra.Command{
		Use:   "run",
		Short: "Run the multinode dashboard",
		RunE:  cmdRun,
	}
	setupCmd = &cobra.Command{
		Use:         "setup",
		Short:       "Create config files",
		RunE:        cmdSetup,
		Annotations: map[string]string{"type": "setup"},
	}

	addCmd = &cobra.Command{
		Use:   "add [file]",
		Short: "Add storage node(s) from file or stdin to multinode dashboard",
		RunE:  cmdAdd,
		Args:  cobra.MaximumNArgs(1),
		Example: `
# add nodes from json file containing array of nodes data
$ multinode add nodes.json

# add node from json file containing a single node object
$ multinode add node.json

# read nodes data from stdin
$ cat nodes.json | multinode add -
`,
	}

	runCfg   Config
	setupCfg Config
	addCfg   struct {
		NodeID        string `help:"ID of the storage node" default:""`
		Name          string `help:"Name of the storage node" default:""`
		APISecret     string `help:"API Secret of the storage node" default:""`
		PublicAddress string `help:"Public IP Address of the storage node" default:""`

		Config
	}
	confDir     string
	identityDir string
)

func main() {
	logger, _, _ := process.NewLogger("multinode")
	zap.ReplaceGlobals(logger)

	process.ExecCustomDebug(rootCmd)
}

func init() {
	defaultConfDir := fpath.ApplicationDir("storj", "multinode")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &confDir, "config-dir", defaultConfDir, "main directory for multinode configuration")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &identityDir, "identity-dir", "", "main directory for multinode identity credentials")
	defaults := cfgstruct.DefaultsFlag(rootCmd)

	// Ignoring errors since MarkDeprecated only errors if the flag
	// doesn't exist or no deprecated message is provided.
	// and MarkHidden only errors if the flag doesn't exist.
	_ = rootCmd.PersistentFlags().MarkDeprecated("identity-dir", "multinode no longer requires an identity key")
	_ = rootCmd.PersistentFlags().MarkHidden("identity-dir")

	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(addCmd)

	process.Bind(runCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(setupCmd, &setupCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir), cfgstruct.SetupMode())
	process.Bind(addCmd, &addCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	setupDir, err := filepath.Abs(confDir)
	if err != nil {
		return err
	}

	valid, _ := fpath.IsValidSetupDir(setupDir)
	if !valid {
		return fmt.Errorf("multinode configuration already exists (%v)", setupDir)
	}

	err = os.MkdirAll(setupDir, 0700)
	if err != nil {
		return err
	}

	return process.SaveConfig(cmd, filepath.Join(setupDir, "config.yaml"))
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	identity, err := getIdentity(ctx, &runCfg)
	if err != nil {
		log.Error("failed to load identity", zap.Error(err))
		return errs.New("failed to load identity: %+v", err)
	}

	if err := process.InitMetrics(ctx, log, monkit.Default, process.MetricsIDFromHostname(log), process.UDPDestination); err != nil {
		log.Warn("Failed to initialize telemetry", zap.Error(err))
	}

	db, err := multinodedb.Open(ctx, log.Named("db"), runCfg.Database)
	if err != nil {
		return errs.New("error connecting to master database on multinode: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()
	if err := db.MigrateToLatest(ctx); err != nil {
		return err
	}

	peer, err := multinode.New(log, identity, runCfg.Config, db)
	if err != nil {
		return err
	}

	runError := peer.Run(ctx)
	closeError := peer.Close()
	return errs.Combine(runError, closeError)
}

func cmdAdd(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	identity, err := getIdentity(ctx, &addCfg.Config)
	if err != nil {
		return errs.New("failed to load identity: %+v", err)
	}

	db, err := multinodedb.Open(ctx, log.Named("db"), addCfg.Database)
	if err != nil {
		return errs.New("error connecting to master database on multinode: %+v", err)
	}

	tlsConfig := tlsopts.Config{
		UsePeerCAWhitelist: false,
		PeerIDVersions:     "0",
	}

	tlsOptions, err := tlsopts.NewOptions(identity, tlsConfig, nil)
	if err != nil {
		return err
	}

	dialer := rpc.NewDefaultDialer(tlsOptions)

	var nodeList []nodes.Node

	hasRequiredFlags := addCfg.NodeID != "" && addCfg.APISecret != "" && addCfg.PublicAddress != ""

	if len(args) == 0 && !hasRequiredFlags {
		return errs.New("--node-id, --api-secret and --public-address flags are required if no file is provided")
	}

	if hasRequiredFlags {
		nodeID, err := storj.NodeIDFromString(addCfg.NodeID)
		if err != nil {
			return err
		}
		apiSecret, err := multinodeauth.SecretFromBase64(addCfg.APISecret)
		if err != nil {
			return err
		}
		nodeList = []nodes.Node{
			{
				ID:            nodeID,
				PublicAddress: addCfg.PublicAddress,
				APISecret:     apiSecret,
				Name:          addCfg.Name,
			},
		}
	} else {
		path := args[0]
		var nodesJSONData []byte
		if path == "-" {
			stdin := cmd.InOrStdin()
			data, err := io.ReadAll(stdin)
			if err != nil {
				return err
			}
			nodesJSONData = data
		} else {
			nodesJSONData, err = os.ReadFile(path)
			if err != nil {
				return err
			}
		}

		nodeList, err = unmarshalJSONNodes(nodesJSONData)
		if err != nil {
			return err
		}
	}

	for _, node := range nodeList {
		if _, err := db.Nodes().Get(ctx, node.ID); err == nil {
			return errs.New("Node with ID %s is already added to the multinode dashboard", node.ID)
		}

		service := nodes.NewService(log, dialer, db.Nodes())
		err = service.Add(ctx, node)
		if err != nil {
			return err
		}
	}

	return nil
}

// decodeUTF16or8 decodes the b as UTF-16 if the special byte order mark is present.
func decodeUTF16or8(b []byte) ([]byte, error) {
	r := bytes.NewReader(b)
	// fallback to r if no BOM sequence is located in the source text.
	t := unicode.BOMOverride(transform.Nop)
	return io.ReadAll(transform.NewReader(r, t))
}

func unmarshalJSONNodes(nodesJSONData []byte) ([]nodes.Node, error) {
	var nodesInfo []nodes.Node
	var err error

	nodesJSONData, err = decodeUTF16or8(nodesJSONData)
	if err != nil {
		return nil, err
	}
	nodesJSONData = bytes.TrimLeft(nodesJSONData, " \t\r\n")

	switch {
	case len(nodesJSONData) > 0 && nodesJSONData[0] == '[': // data is json array
		err := json.Unmarshal(nodesJSONData, &nodesInfo)
		if err != nil {
			return nil, err
		}
	case len(nodesJSONData) > 0 && nodesJSONData[0] == '{': // data is json object
		var singleNode nodes.Node
		err := json.Unmarshal(nodesJSONData, &singleNode)
		if err != nil {
			return nil, err
		}
		nodesInfo = []nodes.Node{singleNode}
	default:
		return nil, errs.New("invalid JSON format")
	}

	return nodesInfo, nil
}

func getIdentity(ctx context.Context, cfg *Config) (*identity.FullIdentity, error) {
	// for backwards compatibility reasons, check if an identity was provided.
	if cfgstruct.FindIdentityDirParam() != "" {
		ident, err := cfg.Identity.Load()
		if err == nil {
			return ident, nil
		}
		zap.L().Error("failed to load identity.", zap.Error(err))
		zap.L().Info("generating new identity.")
	}
	// generate new identity
	return identity.NewFullIdentity(ctx, identity.NewCAOptions{
		Difficulty:  0,
		Concurrency: 1,
	})
}
