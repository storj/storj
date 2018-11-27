// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"context"
<<<<<<< HEAD
	"fmt"
=======
	"os"
>>>>>>> replaces makeUplinkPath with applicationDir func
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/miniogw"
	"storj.io/storj/pkg/storage/buckets"
	"storj.io/storj/pkg/storj"
)

// Config is miniogw.Config configuration
type Config struct {
	miniogw.Config
}

var cfg Config

// CLICmd represents the base CLI command when called without any subcommands
var CLICmd = &cobra.Command{
	Use:   "uplink",
	Short: "The Storj client-side CLI",
}

// GWCmd represents the base gateway command when called without any subcommands
var GWCmd = &cobra.Command{
	Use:   "gateway",
	Short: "The Storj client-side S3 gateway",
}

func applicationDir(subdir ...string) string {
	for i := range subdir {
		if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
			subdir[i] = strings.Title(subdir[i])
		} else {
			subdir[i] = strings.ToLower(subdir[i])
		}
	}
	home := os.Getenv("HOME")
	appdir := filepath.Join(home, "Library", "Application Support")

	switch runtime.GOOS {
	case "windows":
		for _, env := range []string{"AppData", "AppDataLocal", "UserProfile", "Home"} {
			val := os.Getenv(env)
			if val != "" {
				appdir = val
				break
			}
		}
	case "darwin":
		// Mac standards: https://developer.apple.com/library/archive/documentation/FileManagement/Conceptual/FileSystemProgrammingGuide/MacOSXDirectories/MacOSXDirectories.html
		appdir = filepath.Join(home, "Library", "Application Support")
	case "linux":
		fallthrough
	default:
		// XDG standards: https://specifications.freedesktop.org/basedir-spec/latest/ar01s03.html
		appdir = os.Getenv("XDG_DATA_HOME")
		if appdir == "" {
			appdir = home
		}
	}
	return filepath.Join(append([]string{appdir}, subdir...)...)
}

func addCmd(cmd *cobra.Command, root *cobra.Command) *cobra.Command {
	root.AddCommand(cmd)

	defaultConfDir := applicationDir("storj", "uplink")
	cfgstruct.Bind(cmd.Flags(), &cfg, cfgstruct.ConfDir(defaultConfDir))
	cmd.Flags().String("config", filepath.Join(defaultConfDir, "config.yaml"), "path to configuration")
	return cmd
}

// BucketStore loads the buckets.Store
func (c *Config) BucketStore(ctx context.Context) (buckets.Store, error) {
	identity, err := c.Load()
	if err != nil {
		return nil, err
	}

	return c.GetBucketStore(ctx, identity)
}

func convertError(err error, path fpath.FPath) error {
	if storj.ErrBucketNotFound.Has(err) {
		return fmt.Errorf("Bucket not found: %s", path.Bucket())
	}

	if storj.ErrObjectNotFound.Has(err) {
		return fmt.Errorf("Object not found: %s", path.String())
	}

	return err
}
