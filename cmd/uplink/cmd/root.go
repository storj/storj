// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"storj.io/common/fpath"
	"storj.io/common/storj"
	"storj.io/private/cfgstruct"
	"storj.io/private/process"
	"storj.io/storj/private/version/checker"
	"storj.io/uplink"
	privateAccess "storj.io/uplink/private/access"
)

const advancedFlagName = "advanced"

// UplinkFlags configuration flags.
type UplinkFlags struct {
	Config

	Version checker.Config

	PBKDFConcurrency int `help:"Unfortunately, up until v0.26.2, keys generated from passphrases depended on the number of cores the local CPU had. If you entered a passphrase with v0.26.2 earlier, you'll want to set this number to the number of CPU cores your computer had at the time. This flag may go away in the future. For new installations the default value is highly recommended." default:"0"`
}

var (
	cfg     UplinkFlags
	confDir string

	defaults = cfgstruct.DefaultsFlag(RootCmd)

	// Error is the class of errors returned by this package.
	Error = errs.Class("uplink")
	// ErrAccessFlag is used where the `--access` flag is registered but not supported.
	ErrAccessFlag = Error.New("--access flag not supported with `setup` and `import` subcommands")
)

func init() {
	defaultConfDir := fpath.ApplicationDir("storj", "uplink")
	cfgstruct.SetupFlag(zap.L(), RootCmd, &confDir, "config-dir", defaultConfDir, "main directory for uplink configuration")

	// NB: more-help flag is always retrieved using `findBoolFlagEarly()`
	RootCmd.PersistentFlags().BoolVar(new(bool), advancedFlagName, false, "if used in with -h, print advanced flags help")

	setBasicFlags(RootCmd.PersistentFlags(), "config-dir", advancedFlagName)
	setUsageFunc(RootCmd)
}

var cpuProfile = flag.String("profile.cpu", "", "file path of the cpu profile to be created")
var memoryProfile = flag.String("profile.mem", "", "file path of the memory profile to be created")

// RootCmd represents the base CLI command when called without any subcommands.
var RootCmd = &cobra.Command{
	Use:                "uplink",
	Short:              "The Storj client-side CLI",
	Args:               cobra.OnlyValidArgs,
	PersistentPreRunE:  combineCobraFuncs(startCPUProfile, modifyFlagDefaults),
	PersistentPostRunE: stopAndWriteProfile,
}

func addCmd(cmd *cobra.Command, root *cobra.Command) *cobra.Command {
	root.AddCommand(cmd)

	process.Bind(cmd, &cfg, defaults, cfgstruct.ConfDir(getConfDir()))

	return cmd
}

func (cliCfg *UplinkFlags) getProject(ctx context.Context, encryptionBypass bool) (_ *uplink.Project, err error) {
	access, err := cfg.GetAccess()
	if err != nil {
		return nil, err
	}

	uplinkCfg := uplink.Config{}
	uplinkCfg.UserAgent = cliCfg.Client.UserAgent
	uplinkCfg.DialTimeout = cliCfg.Client.DialTimeout

	if encryptionBypass {
		err = privateAccess.EnablePathEncryptionBypass(access)
		if err != nil {
			return nil, Error.Wrap(err)
		}
	}
	project, err := uplinkCfg.OpenProject(ctx, access)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return project, nil
}

func closeProject(project *uplink.Project) {
	if err := project.Close(); err != nil {
		fmt.Printf("error closing project: %+v\n", err)
	}
}

func convertError(err error, path fpath.FPath) error {
	if storj.ErrBucketNotFound.Has(err) {
		return fmt.Errorf("bucket not found: %s", path.Bucket())
	}

	if storj.ErrObjectNotFound.Has(err) {
		return fmt.Errorf("object not found: %s", path.String())
	}

	return err
}

func startCPUProfile(cmd *cobra.Command, args []string) error {
	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			return err
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			return err
		}
	}
	return nil
}

func stopAndWriteProfile(cmd *cobra.Command, args []string) error {
	if *cpuProfile != "" {
		pprof.StopCPUProfile()
	}
	if *memoryProfile != "" {
		return writeMemoryProfile()
	}
	return nil
}

func writeMemoryProfile() error {
	f, err := os.Create(*memoryProfile)
	if err != nil {
		return err
	}
	runtime.GC()
	if err := pprof.WriteHeapProfile(f); err != nil {
		return err
	}
	return f.Close()
}

// convertAccessesForViper converts map[string]string to map[string]interface{}.
//
// This is a little hacky but viper deserializes accesses into a map[string]interface{}
// and complains if we try and override with map[string]string{}.
func convertAccessesForViper(from map[string]string) map[string]interface{} {
	to := make(map[string]interface{})
	for key, value := range from {
		to[key] = value
	}
	return to
}

func modifyFlagDefaults(cmd *cobra.Command, args []string) (err error) {
	levelFlag := cmd.Flag("log.level")
	if levelFlag != nil && !levelFlag.Changed {
		err := flag.Set("log.level", zapcore.WarnLevel.String())
		if err != nil {
			return Error.Wrap(errs.Combine(errs.New("unable to set log level flag"), err))
		}
	}
	return nil
}

func combineCobraFuncs(funcs ...func(*cobra.Command, []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) (err error) {
		for _, fn := range funcs {
			if err = fn(cmd, args); err != nil {
				return err
			}
		}
		return err
	}
}

/*	`setUsageFunc` is a bit unconventional but cobra didn't leave much room for
	extensibility here. `cmd.SetUsageTemplate` is fairly useless for our case without
	the ability to add to the template's function map (see: https://golang.org/pkg/text/template/#hdr-Functions).

	Because we can't alter what `cmd.Usage` generates, we have to edit it afterwards.
	In order to hook this function *and* get the usage string, we have to juggle the
	`cmd.usageFunc` between our hook and `nil`, so that we can get the usage string
	from the default usage func.
*/
func setUsageFunc(cmd *cobra.Command) {
	if findBoolFlagEarly(advancedFlagName) {
		return
	}

	reset := func() (set func()) {
		original := cmd.UsageFunc()
		cmd.SetUsageFunc(nil)

		return func() {
			cmd.SetUsageFunc(original)
		}
	}

	cmd.SetUsageFunc(func(cmd *cobra.Command) error {
		set := reset()
		usageStr := cmd.UsageString()
		defer set()

		usageScanner := bufio.NewScanner(bytes.NewBufferString(usageStr))

		var basicFlags []string
		cmd.Flags().VisitAll(func(flag *pflag.Flag) {
			basic, ok := flag.Annotations[cfgstruct.BasicHelpAnnotationName]
			if ok && len(basic) == 1 && basic[0] == "true" {
				basicFlags = append(basicFlags, flag.Name)
			}
		})

		for usageScanner.Scan() {
			line := usageScanner.Text()
			trimmedLine := strings.TrimSpace(line)

			var flagName string
			if _, err := fmt.Sscanf(trimmedLine, "--%s", &flagName); err != nil {
				fmt.Println(line)
				continue
			}

			// TODO: properly filter flags with short names
			if !strings.HasPrefix(trimmedLine, "--") {
				fmt.Println(line)
			}

			for _, basicFlag := range basicFlags {
				if basicFlag == flagName {
					fmt.Println(line)
				}
			}
		}
		return nil
	})
}

func findBoolFlagEarly(flagName string) bool {
	for i, arg := range os.Args {
		arg := arg
		argHasPrefix := func(format string, args ...interface{}) bool {
			return strings.HasPrefix(arg, fmt.Sprintf(format, args...))
		}

		if !argHasPrefix("--%s", flagName) {
			continue
		}

		// NB: covers `--<flagName> false` usage
		if i+1 != len(os.Args) {
			next := os.Args[i+1]
			if next == "false" {
				return false
			}
		}

		if !argHasPrefix("--%s=false", flagName) {
			return true
		}
	}
	return false
}

func setBasicFlags(flagset interface{}, flagNames ...string) {
	for _, name := range flagNames {
		cfgstruct.SetBoolAnnotation(flagset, name, cfgstruct.BasicHelpAnnotationName, true)
	}
}
