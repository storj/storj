// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/common/fpath"
	"storj.io/private/cfgstruct"
	"storj.io/private/process"
	"storj.io/uplink"
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
	BaseURL           string   `default:"https://link.tardigradeshare.io" help:"the base url for link sharing"`

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

func parseHumanDate(date string, now time.Time) (time.Time, error) {
	switch {
	case date == "":
		return time.Time{}, nil
	case date == "now":
		return now, nil
	case date[0] == '+':
		d, err := time.ParseDuration(date[1:])
		t := now.Add(d)
		return t, errs.Wrap(err)
	case date[0] == '-':
		d, err := time.ParseDuration(date[1:])
		t := now.Add(-d)
		return t, errs.Wrap(err)
	default:
		t, err := time.Parse(time.RFC3339, date)
		return t, errs.Wrap(err)
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

	var sharePrefixes []uplink.SharePrefix
	for _, path := range shareCfg.AllowedPathPrefix {
		p, err := fpath.New(path)
		if err != nil {
			return err
		}
		if p.IsLocal() {
			return errs.New("required path must be remote: %q", path)
		}

		sharePrefixes = append(sharePrefixes, uplink.SharePrefix{
			Bucket: p.Bucket(),
			Prefix: p.Path(),
		})
	}

	access, err := shareCfg.GetNewAccess()
	if err != nil {
		return err
	}

	permission := uplink.Permission{}
	permission.AllowDelete = !shareCfg.DisallowDeletes && !shareCfg.Readonly
	permission.AllowList = !shareCfg.DisallowLists && !shareCfg.Writeonly
	permission.AllowDownload = !shareCfg.DisallowReads && !shareCfg.Writeonly
	permission.AllowUpload = !shareCfg.DisallowWrites && !shareCfg.Readonly
	permission.NotBefore = notBefore
	permission.NotAfter = notAfter

	newAccess, err := access.Share(permission, sharePrefixes...)
	if err != nil {
		return err
	}

	newAccessData, err := newAccess.Serialize()
	if err != nil {
		return err
	}

	// TODO extend libuplink to give this value
	// fmt.Println("Sharing access to satellite", access.SatelliteAddr)

	fmt.Println("=========== ACCESS RESTRICTIONS ==========================================================")
	fmt.Println("Download  :", formatPermission(permission.AllowDownload))
	fmt.Println("Upload    :", formatPermission(permission.AllowUpload))
	fmt.Println("Lists     :", formatPermission(permission.AllowList))
	fmt.Println("Deletes   :", formatPermission(permission.AllowDelete))
	fmt.Println("NotBefore :", formatTimeRestriction(permission.NotBefore))
	fmt.Println("NotAfter  :", formatTimeRestriction(permission.NotAfter))
	fmt.Println("Paths     :", formatPaths(sharePrefixes))
	fmt.Println("=========== SERIALIZED ACCESS WITH THE ABOVE RESTRICTIONS TO SHARE WITH OTHERS ===========")
	fmt.Println("Access    :", newAccessData)

	if len(shareCfg.AllowedPathPrefix) == 1 {
		fmt.Println("=========== BROWSER URL ==================================================================")
		p, err := fpath.New(shareCfg.AllowedPathPrefix[0])
		if err != nil {
			return err
		}
		fmt.Println("URL       :", fmt.Sprintf("%s/%s/%s/%s", shareCfg.BaseURL,
			url.PathEscape(newAccessData),
			url.PathEscape(p.Bucket()),
			url.PathEscape(p.Path())))
	} else {
		fmt.Println("=========== BROWSER URL PREFIX ===========================================================")
		fmt.Println("URL       :", fmt.Sprintf("%s/%s", shareCfg.BaseURL,
			url.PathEscape(newAccessData)))
	}

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

func formatTimeRestriction(t time.Time) string {
	if t.IsZero() {
		return "No restriction"
	}
	return formatTime(t)
}

func formatPaths(sharePrefixes []uplink.SharePrefix) string {
	if len(sharePrefixes) == 0 {
		return "WARNING! The entire project is shared!"
	}

	var paths []string
	for _, prefix := range sharePrefixes {
		path := "sj://" + prefix.Bucket
		if len(prefix.Prefix) == 0 {
			path += " (entire bucket)"
		} else {
			path += "/" + prefix.Prefix
		}
		paths = append(paths, path)
	}

	return strings.Join(paths, "\n            ")
}
