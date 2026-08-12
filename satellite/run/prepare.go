// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/satellite/compensation"
	"storj.io/storj/satellite/geoip"
)

// PrepareCmdConfig configures the invoice prepare command.
type PrepareCmdConfig struct {
	ForceMandatory  bool     `help:"Force mandatory fields in invoices"`
	Concurrency     int      `help:"Concurrent workers to check IP address" default:"10"`
	GeoIpDbs        []string `help:"GeoIP databases to use for IP address lookup" default:"GeoLite2-City.mmdb"`
	SkipOFAC        bool     `help:"Skip OFAC checks"`
	AllowUnscreened bool     `help:"Write payouts even if some nodes could not be OFAC-screened"`
	Invoice         string   `help:"Path to the invoices CSV" required:"true"`
}

// Prepare is the command that turns invoices into incomplete paystubs and prepayouts.
type Prepare struct {
	log *zap.Logger
	cfg *PrepareCmdConfig
}

// NewPrepare constructs a Prepare command with the provided config.
func NewPrepare(log *zap.Logger, cfg *PrepareCmdConfig) *Prepare {
	return &Prepare{log: log, cfg: cfg}
}

// Run executes the prepare command.
func (p *Prepare) Run() (err error) {
	log := p.log
	if log == nil {
		log = zap.NewNop()
	}

	var geoIPDBs []*geoip.MaxmindDB
	defer func() {
		for _, geoIPDB := range geoIPDBs {
			err = errs.Combine(err, geoIPDB.Close())
		}
	}()
	if !p.cfg.SkipOFAC {
		if len(p.cfg.GeoIpDbs) == 0 {
			return errs.New("at least one GeoIP database must be configured")
		}
		for _, geoIPDBPath := range p.cfg.GeoIpDbs {
			if strings.Contains(geoIPDBPath, "-Country") {
				return errs.New("geo ip database looks like a country database, but a city level db is now required")
			}
			geoIPDB, err := geoip.OpenMaxmindDB(geoIPDBPath)
			if err != nil {
				return err
			}
			geoIPDBs = append(geoIPDBs, geoIPDB)
		}
	}

	invoicesIn, err := os.Open(p.cfg.Invoice)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, invoicesIn.Close()) }()

	ipaystubsPath := makeCSVPath(p.cfg.Invoice, "incompletepaystubs")
	prepayoutsPath := makeCSVPath(p.cfg.Invoice, "prepayouts")

	ipaystubsTmp, err := os.Create(ipaystubsPath + ".tmp")
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() {
		_ = ipaystubsTmp.Close()
		_ = os.Remove(ipaystubsTmp.Name())
	}()

	prepayoutsTmp, err := os.Create(prepayoutsPath + ".tmp")
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() {
		_ = prepayoutsTmp.Close()
		_ = os.Remove(prepayoutsTmp.Name())
	}()

	if err := compensation.Prepare(invoicesIn, ipaystubsTmp, prepayoutsTmp, compensation.PrepareConfig{
		ForceMandatory:  p.cfg.ForceMandatory,
		Concurrency:     p.cfg.Concurrency,
		GeoIPDBs:        geoIPDBs,
		SkipOFAC:        p.cfg.SkipOFAC,
		AllowUnscreened: p.cfg.AllowUnscreened,
		Log:             log,
	}); err != nil {
		return err
	}

	if err := ipaystubsTmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(ipaystubsTmp.Name(), ipaystubsPath); err != nil {
		return err
	}

	if err := prepayoutsTmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(prepayoutsTmp.Name(), prepayoutsPath); err != nil {
		return err
	}

	return nil
}

func makeCSVPath(in, out string) string {
	ext := filepath.Ext(in)
	dir, name := filepath.Split(in[:len(in)-len(ext)])

	out = strings.ToLower(out)
	name = strings.ToLower(name)

	var segments []string
	for _, segment := range strings.Split(name, "-") {
		if segment != "" {
			segments = append(segments, segment)
		}
	}

	// remove the last element
	if len(segments) > 0 {
		segments = segments[:len(segments)-1]
	}

	segments = append(segments, out)
	return dir + strings.Join(segments, "-") + ext
}
