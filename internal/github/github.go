package main

import (
	"fmt"
	"go.uber.org/zap"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
)

type GithubCfg struct {
	Mine bool `default:"true" help:"if true, restricts PR commands to PRs your github user has opened"`
	ClientConfig
}

var (
	rootCmd = &cobra.Command{
		Use:   "github",
		Short: "command for interacting with the github API",
	}

	setupCmd = &cobra.Command{
		Use:   "setup <username> <password>",
		Short: "create an oauth token and config, and write them to disk",
		Args:  cobra.ExactArgs(2),
		RunE:  cmdSetup,
		Annotations: map[string]string{"setup": "true"},
	}

	//setup (create oauth token using basic auth -- write to config!)
	//watchCmd
	//mergeCmd
	//updateCmd
	//rebuild-failing

	setupCfg GithubCfg

	defaultConfDir = fpath.ApplicationDir("github")
	confDir        string
)

func init() {
	fmt.Println("init")
	rootCmd.AddCommand(setupCmd)

	cfgstruct.BindSetup(setupCmd.Flags(), &setupCfg, cfgstruct.ConfDir(defaultConfDir))
}

func cmdSetup(cmd *cobra.Command, args []string) error {
	fmt.Print("zero")
	setupDir, err := filepath.Abs(confDir)
	if err != nil {
		fmt.Print("one")
		return err
	}

	valid, _ := fpath.IsValidSetupDir(setupDir)
	if !valid {
		fmt.Print("two")
		return fmt.Errorf("github configuration already exists (%v)", setupDir)
	}

	// create oauth token...

	err = os.MkdirAll(setupDir, 0700)
	if err != nil {
		return err
	}

	configFile := filepath.Join(setupDir, "config.yaml")

	if err := process.SaveConfig(cmd.Flags(), configFile, nil); err != nil {
		return err
	}
	return nil
}

func main() {
	fmt.Println("main")
	confDirParam := cfgstruct.FindConfigDirParam()
	if confDirParam != "" {
		defaultConfDir = confDirParam
	}

	rootCmd.PersistentFlags().StringVar(&confDir, "config-dir", defaultConfDir, "main directory for storagenode configuration")
	err := rootCmd.PersistentFlags().SetAnnotation("config-dir", "setup", []string{"true"})
	if err != nil {
		zap.S().Error("Failed to set 'setup' annotation for 'config-dir'")
	}

	fmt.Println("end main")
	process.Exec(rootCmd)
	fmt.Println("actually main")
}
