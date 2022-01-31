// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/common/base58"
	"storj.io/common/pb"
	"storj.io/private/cfgstruct"
	"storj.io/private/process"
)

var importCfg struct {
	Overwrite        bool   `default:"false" help:"if true, allows an access to be overwritten" source:"flag"`
	SatelliteAddress string `default:"" help:"updates satellite address in imported Access" hidden:"true"`

	UplinkFlags
}

func init() {
	importCmd := &cobra.Command{
		Use:         "import [NAME] (ACCESS | FILE)",
		Short:       "Imports an access into configuration. Configuration will be created if doesn't exists.",
		Args:        cobra.RangeArgs(1, 2),
		RunE:        importMain,
		Annotations: map[string]string{"type": "setup"},
	}
	RootCmd.AddCommand(importCmd)

	// We don't really want all of the uplink flags on this command but
	// otherwise, there is difficulty getting the config to load right since
	// configuration/flag code assumes it needs to load/persist everything from
	// flags.
	// TODO: revisit after the configuration/flag code is refactored.
	process.Bind(importCmd, &importCfg, defaults, cfgstruct.ConfDir(confDir))

	// NB: access is not supported by `setup` or `import`
	cfgstruct.SetBoolAnnotation(importCmd.Flags(), "access", cfgstruct.BasicHelpAnnotationName, false)
}

// importMain is the function executed when importCmd is called.
func importMain(cmd *cobra.Command, args []string) (err error) {
	if cmd.Flag("access").Changed {
		return ErrAccessFlag
	}

	saveConfig := func(saveConfigOption process.SaveConfigOption) error {
		path := filepath.Join(confDir, process.DefaultCfgFilename)
		exists, err := fileExists(path)
		if err != nil {
			return Error.Wrap(err)
		}
		if !exists {
			if err := createConfigFile(path); err != nil {
				return err
			}
		}

		return process.SaveConfig(cmd, path,
			saveConfigOption,
			process.SaveConfigRemovingDeprecated())
	}

	// one argument means we are importing into main 'access' field without name
	if len(args) == 1 {
		overwritten := false
		if importCfg.Access != "" {
			if !importCfg.Overwrite {
				return Error.New("%s", "default access already exists")
			}
			overwritten = true
		}

		accessData, err := findAccess(args[0])
		if err != nil {
			return Error.Wrap(err)
		}

		if importCfg.SatelliteAddress != "" {
			newAccessData, err := updateSatelliteAddress(importCfg.SatelliteAddress, accessData)
			if err != nil {
				return Error.Wrap(err)
			}
			accessData = newAccessData
		}

		if err := saveConfig(process.SaveConfigWithOverride("access", accessData)); err != nil {
			return err
		}

		if overwritten {
			fmt.Printf("default access overwritten.\n")
		} else {
			fmt.Printf("default access imported.\n")
		}
	} else {
		name := args[0]

		// This is a little hacky but viper deserializes accesses into a map[string]interface{}
		// and complains if we try and override with map[string]string{}.
		accesses := convertAccessesForViper(importCfg.Accesses)

		overwritten := false
		if _, ok := accesses[name]; ok {
			if !importCfg.Overwrite {
				return fmt.Errorf("access %q already exists", name)
			}
			overwritten = true
		}

		accessData, err := findAccess(args[1])
		if err != nil {
			return Error.Wrap(err)
		}

		if importCfg.SatelliteAddress != "" {
			newAccessData, err := updateSatelliteAddress(importCfg.SatelliteAddress, accessData)
			if err != nil {
				return Error.Wrap(err)
			}
			accessData = newAccessData
		}

		// There is no easy way currently to save off a "hidden" configurable into
		// the config file without a larger refactoring. For now, just do a manual
		// override of the access.
		// TODO: revisit when the configuration/flag code makes it easy
		accessKey := "accesses." + name
		if err := saveConfig(process.SaveConfigWithOverride(accessKey, accessData)); err != nil {
			return err
		}

		if overwritten {
			fmt.Printf("access %q overwritten.\n", name)
		} else {
			fmt.Printf("access %q imported.\n", name)
		}
	}

	return nil
}

func findAccess(input string) (access string, err error) {
	// check if parameter is a valid access, otherwise try to read it from file
	if IsSerializedAccess(input) {
		access = input
	} else {
		path := input

		access, err = readFirstUncommentedLine(path)
		if err != nil {
			return "", err
		}

		// Parse the access data to ensure it is well formed
		if !IsSerializedAccess(access) {
			return "", err
		}
	}
	return access, nil
}

func readFirstUncommentedLine(path string) (_ string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, f.Close()) }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line[0] == '#' {
			continue
		}
		return line, nil
	}

	if err := scanner.Err(); err != nil {
		return "", Error.Wrap(err)
	}

	return "", Error.New("no data found")
}

func createConfigFile(path string) error {
	setupDir, err := filepath.Abs(confDir)
	if err != nil {
		return err
	}

	err = os.MkdirAll(setupDir, 0700)
	if err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}

	return f.Close()
}

func fileExists(path string) (bool, error) {
	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return !stat.IsDir(), nil
}

func updateSatelliteAddress(satelliteAddr string, serializedAccess string) (string, error) {
	data, version, err := base58.CheckDecode(serializedAccess)
	if err != nil || version != 0 {
		return "", errors.New("invalid access grant format")
	}
	p := new(pb.Scope)
	if err := pb.Unmarshal(data, p); err != nil {
		return "", err

	}

	p.SatelliteAddr = satelliteAddr
	accessData, err := pb.Marshal(p)
	if err != nil {
		return "", err
	}
	return base58.CheckEncode(accessData, 0), nil
}
