// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/common/fpath"
	"storj.io/common/macaroon"
	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
)

var shareCfg struct {
	DisallowReads     bool     `default:"false" help:"if true, disallow reads" basic-help:"true"`
	DisallowWrites    bool     `default:"false" help:"if true, disallow writes" basic-help:"true"`
	DisallowLists     bool     `default:"false" help:"if true, disallow lists" basic-help:"true"`
	DisallowDeletes   bool     `default:"false" help:"if true, disallow deletes" basic-help:"true"`
	Readonly          bool     `default:"false" help:"implies disallow_writes and disallow_deletes" basic-help:"true"`
	Writeonly         bool     `default:"false" help:"implies disallow_reads and disallow_lists" basic-help:"true"`
	NotBefore         string   `help:"disallow access before this time (e.g. '+2h', '2020-01-02T15:01:01-01:00')" basic-help:"true"`
	NotAfter          string   `help:"disallow access after this time (e.g. '+2h', '2020-01-02T15:01:01-01:00')" basic-help:"true"`
	AllowedPathPrefix []string `help:"whitelist of path prefixes to require, overrides the [allowed-path-prefix] arguments"`
	ExportTo          string   `default:"" help:"path to export the shared access to" basic-help:"true"`

	// Share requires information about the current access
	AccessConfig
}

func init() {
	// We skip the use of addCmd here because we only want the configuration options listed
	// above, and addCmd adds a whole lot more than we want.

	shareCmd := &cobra.Command{
		Use:   "share [ALLOWED_PATH_PREFIX]...",
		Short: "Shares restricted access to objects.",
		RunE:  shareMain,
	}
	RootCmd.AddCommand(shareCmd)

	process.Bind(shareCmd, &shareCfg, defaults, cfgstruct.ConfDir(getConfDir()))
}

func parseHumanDate(date string, now time.Time) (*time.Time, error) {
	switch {
	case date == "":
		return nil, nil
	case date == "now":
		return &now, nil
	case date[0] == '+':
		d, err := time.ParseDuration(date[1:])
		t := now.Add(d)
		return &t, errs.Wrap(err)
	case date[0] == '-':
		d, err := time.ParseDuration(date[1:])
		t := now.Add(-d)
		return &t, errs.Wrap(err)
	default:
		t, err := time.Parse(time.RFC3339, date)
		return &t, errs.Wrap(err)
	}
}

// shareMain is the function executed when shareCmd is called
func shareMain(cmd *cobra.Command, args []string) (err error) {
	now := time.Now()

	notBefore, err := parseHumanDate(shareCfg.NotBefore, now)
	if err != nil {
		return err
	}
	notAfter, err := parseHumanDate(shareCfg.NotAfter, now)
	if err != nil {
		return err
	}

	if len(shareCfg.AllowedPathPrefix) == 0 {
		// if the --allowed-path-prefix flag is not set,
		// use any arguments as allowed path prefixes
		for _, arg := range args {
			shareCfg.AllowedPathPrefix = append(shareCfg.AllowedPathPrefix, strings.Split(arg, ",")...)
		}
	}

	var restrictions []libuplink.EncryptionRestriction
	for _, path := range shareCfg.AllowedPathPrefix {
		p, err := fpath.New(path)
		if err != nil {
			return err
		}
		if p.IsLocal() {
			return errs.New("required path must be remote: %q", path)
		}

		restrictions = append(restrictions, libuplink.EncryptionRestriction{
			Bucket:     p.Bucket(),
			PathPrefix: p.Path(),
		})
	}

	access, err := shareCfg.GetAccess()
	if err != nil {
		return err
	}
	key, encAccess := access.APIKey, access.EncryptionAccess

	if len(restrictions) > 0 {
		key, encAccess, err = encAccess.Restrict(key, restrictions...)
		if err != nil {
			return err
		}
	}

	caveat, err := macaroon.NewCaveat()
	if err != nil {
		return err
	}

	caveat.DisallowDeletes = shareCfg.DisallowDeletes || shareCfg.Readonly
	caveat.DisallowLists = shareCfg.DisallowLists || shareCfg.Writeonly
	caveat.DisallowReads = shareCfg.DisallowReads || shareCfg.Writeonly
	caveat.DisallowWrites = shareCfg.DisallowWrites || shareCfg.Readonly
	caveat.NotBefore = notBefore
	caveat.NotAfter = notAfter

	key, err = key.Restrict(caveat)
	if err != nil {
		return err
	}

	newAccess := &libuplink.Scope{
		SatelliteAddr:    access.SatelliteAddr,
		APIKey:           key,
		EncryptionAccess: encAccess,
	}

	newAccessData, err := newAccess.Serialize()
	if err != nil {
		return err
	}

	fmt.Println("Sharing access to satellite", access.SatelliteAddr)
	fmt.Println("=========== ACCESS RESTRICTIONS ==========================================================")
	fmt.Println("Reads     :", formatPermission(!caveat.GetDisallowReads()))
	fmt.Println("Writes    :", formatPermission(!caveat.GetDisallowWrites()))
	fmt.Println("Lists     :", formatPermission(!caveat.GetDisallowLists()))
	fmt.Println("Deletes   :", formatPermission(!caveat.GetDisallowDeletes()))
	fmt.Println("Not Before:", formatTimeRestriction(caveat.NotBefore))
	fmt.Println("Not After :", formatTimeRestriction(caveat.NotAfter))
	fmt.Println("Paths     :", formatPaths(restrictions))
	fmt.Println("=========== SERIALIZED ACCESS WITH THE ABOVE RESTRICTIONS TO SHARE WITH OTHERS ===========")
	fmt.Println("Access    :", newAccessData)

	if shareCfg.ExportTo != "" {
		// convert to an absolute path, mostly for output purposes.
		exportTo, err := filepath.Abs(shareCfg.ExportTo)
		if err != nil {
			return Error.Wrap(err)
		}
		if err := ioutil.WriteFile(exportTo, []byte(newAccessData+"\n"), 0600); err != nil {
			return Error.Wrap(err)
		}
		fmt.Println("Exported to:", exportTo)
	}
	return nil
}

func formatPermission(allowed bool) string {
	if allowed {
		return "Allowed"
	}
	return "Disallowed"
}

func formatTimeRestriction(t *time.Time) string {
	if t == nil {
		return "No restriction"
	}
	return formatTime(*t)
}

func formatPaths(restrictions []libuplink.EncryptionRestriction) string {
	if len(restrictions) == 0 {
		return "WARNING! The entire project is shared!"
	}

	var paths []string
	for _, restriction := range restrictions {
		path := "sj://" + restriction.Bucket
		if len(restriction.PathPrefix) == 0 {
			path += " (entire bucket)"
		} else {
			path += "/" + restriction.PathPrefix
		}
		paths = append(paths, path)
	}

	return strings.Join(paths, "\n            ")
}
