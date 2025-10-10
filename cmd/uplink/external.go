// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/spacemonkeygo/monkit/v3/collect"
	"github.com/spacemonkeygo/monkit/v3/present"
	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/term"

	"storj.io/common/experiment"
	"storj.io/common/rpc/rpctracing"
	"storj.io/common/sync2/mpscqueue"
	"storj.io/common/tracing"
	"storj.io/common/version"
	"storj.io/eventkit"
	jaeger "storj.io/monkit-jaeger"
)

type external struct {
	interactive bool  // controls if interactive input is allowed
	analytics   *bool // enables sending analytics

	dirs struct {
		loaded  bool   // true if Setup has been called
		current string // current config directory
		legacy  string // old config directory
	}

	migration struct {
		migrated bool  // true if a migration has been attempted
		err      error // any error from the migration attempt
	}

	config struct {
		loaded bool                // true if the existing config file is successfully loaded
		values map[string][]string // the existing configuration
	}

	access struct {
		loaded      bool              // true if we've successfully loaded access.json
		defaultName string            // default access name to use from accesses
		accesses    map[string]string // map of all of the stored accesses
	}

	tracing struct {
		traceID      int64             // if non-zero, sets outgoing traces to the given id
		traceAddress string            // if non-zero, sampled spans are sent to this trace collector address.
		tags         map[string]string // coma separated k=v pairs to be added to the trace
		sample       float64           // the chance (number between 0 and 1.0) to send samples to the server.
		verbose      bool              // flag to print out tracing information (like the used trace id)
	}

	debug struct {
		pprofFile       string
		traceFile       string
		monkitTraceFile string
		monkitStatsFile string
	}

	events struct {
		address string // if non-zero, events are sent to this address.
	}
}

func newExternal() *external {
	return &external{}
}

func (ex *external) Setup(f clingy.Flags) {
	ex.interactive = f.Flag(
		"interactive", "Controls if interactive input is allowed", true,
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
		clingy.Advanced,
	).(bool)

	ex.dirs.current = f.Flag(
		"config-dir", "Directory that stores the configuration",
		appDir(false, defaultUplinkSubdir()...),
	).(string)

	ex.dirs.legacy = f.Flag(
		"legacy-config-dir", "Directory that stores legacy configuration. Only used during migration",
		appDir(true, defaultUplinkSubdir()...),
		clingy.Advanced,
	).(string)

	ex.tracing.traceID = f.Flag(
		"trace-id", "Specify a trace id manually. This should be globally unique. "+
			"Usually you don't need to set it, and it will be automatically generated.", int64(0),
		clingy.Transform(transformInt64),
		clingy.Advanced,
	).(int64)

	ex.tracing.sample = f.Flag(
		"trace-sample", "The chance (between 0 and 1.0) to report tracing information. Set to 1 to always send it.", float64(0),
		clingy.Transform(transformFloat64),
		clingy.Advanced,
	).(float64)

	ex.tracing.verbose = f.Flag(
		"trace-verbose", "Flag to print out used trace ID", false,
		clingy.Transform(strconv.ParseBool),
		clingy.Advanced,
	).(bool)

	ex.tracing.traceAddress = f.Flag(
		"trace-addr", "Specify where to send traces", "agent.tracing.datasci.storj.io:5775",
		clingy.Advanced,
	).(string)

	ex.tracing.tags = f.Flag(
		"trace-tags", "comma separated k=v pairs to be added to distributed traces", map[string]string{},
		clingy.Advanced,
		clingy.Transform(func(val string) (map[string]string, error) {
			res := map[string]string{}
			for _, kv := range strings.Split(val, ",") {
				parts := strings.SplitN(kv, "=", 2)
				res[parts[0]] = parts[1]
			}
			return res, nil
		}),
	).(map[string]string)

	ex.events.address = f.Flag(
		"events-addr", "Specify where to send events", "eventkitd.datasci.storj.io:9002",
		clingy.Advanced,
	).(string)

	ex.debug.pprofFile = f.Flag(
		"debug-pprof", "File to collect Golang pprof profiling data", "",
		clingy.Advanced,
	).(string)

	ex.debug.traceFile = f.Flag(
		"debug-trace", "File to collect Golang trace data", "",
		clingy.Advanced,
	).(string)

	ex.debug.monkitTraceFile = f.Flag(
		"debug-monkit-trace", "File to collect Monkit trace data. Understands file extensions .json and .svg", "",
		clingy.Advanced,
	).(string)

	ex.debug.monkitStatsFile = f.Flag(
		"debug-monkit-stats", "File to collect Monkit stats", "",
		clingy.Advanced,
	).(string)

	ex.analytics = f.Flag(
		"analytics", "Whether to send usage information to Storj", nil,
		clingy.Transform(strconv.ParseBool), clingy.Optional, clingy.Boolean,
		clingy.Advanced,
	).(*bool)

	ex.dirs.loaded = true
}

func transformInt64(x string) (int64, error) {
	return strconv.ParseInt(x, 0, 64)
}

func transformFloat64(x string) (float64, error) {
	return strconv.ParseFloat(x, 64)
}

func (ex *external) AccessInfoFile() (string, error) {
	return filepath.Join(ex.dirs.current, "access.json"), nil
}

func (ex *external) ConfigFile() (string, error) {
	return filepath.Join(ex.dirs.current, "config.ini"), nil
}

func (ex *external) legacyConfigFile() string { return filepath.Join(ex.dirs.legacy, "config.yaml") }

// Dynamic is called by clingy to look up values for global flags not specified on the command
// line. This call lets us fill in values from config files or environment variables.
func (ex *external) Dynamic(name string) (vals []string, err error) {
	key := "UPLINK_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
	if val, ok := os.LookupEnv(key); ok {
		return []string{val}, nil
	}

	// if we have not yet loaded the directories, we should not try to migrate
	// and load the current config.
	if !ex.dirs.loaded {
		return nil, nil
	}

	// allow errors from migration and configuration loading so that calls to
	// `uplink setup` can happen and write out a new configuration.
	if err := ex.migrate(); err != nil {
		return nil, nil //nolint
	}
	if err := ex.loadConfig(); err != nil {
		return nil, nil //nolint
	}

	return ex.config.values[name], nil
}

func (ex *external) analyticsEnabled() bool {
	if ex.analytics != nil {
		return *ex.analytics
	}
	// N.B.: saveInitialConfig prompts the user if they want analytics enabled.
	// In the past, even after prompting for this, we did not write out their
	// answer in the config. Instead, what has historically happened is that
	// if the user said yes, we wrote out an empty config, and if the user
	// said no, we wrote out:
	//
	//     [metrics]
	//     addr =
	//
	// So, if the new value (analytics.enabled) exists at all, we prefer that.
	// Otherwise, we need to check for the existence of metrics.addr and if it
	// is an empty value to determine if analytics are disabled. At some point
	// in the future after enough upgrades have happened, perhaps we can switch
	// to just analytics.enabled. Unfortunately, an entirely empty config file
	// is precisely the config file we've been writing out if a user opts in
	// to analytics, so we are only going to have analytics disabled if (a)
	// analytics.enabled says so, or absent that, if the config file's final
	// specification for metrics.addr is the empty string.
	val, err := ex.Dynamic("analytics.enabled")
	if err != nil {
		return false
	}
	if len(val) > 0 {
		enabled, err := strconv.ParseBool(val[len(val)-1])
		if err != nil {
			return false
		}
		return enabled
	}
	val, err = ex.Dynamic("metrics.addr")
	if err != nil {
		return false
	}
	return len(val) == 0 || val[len(val)-1] != ""
}

// Wrap is called by clingy with the command to be executed.
func (ex *external) Wrap(ctx context.Context, cmd clingy.Command) (err error) {
	// Please don't put mon.Task()(&ctx)(&err) to here. We need to create the first trace/span after we initialized
	// all the reporters / observers. First span can be created in cmd.Execute.

	if err = ex.migrate(); err != nil {
		return err
	}
	if err = ex.loadConfig(); err != nil {
		return err
	}
	if !ex.config.loaded {
		if err = saveInitialConfig(ctx, ex, ex.interactive, ex.analytics); err != nil {
			return err
		}
	}

	exp := os.Getenv("STORJ_EXPERIMENTAL")
	if exp != "" {
		ctx = experiment.With(ctx, exp)
	}

	if ex.debug.pprofFile != "" {
		var output *os.File
		output, err = os.Create(ex.debug.pprofFile)
		if err != nil {
			return errs.Wrap(err)
		}
		defer func() {
			err = errs.Combine(err, output.Close())
		}()

		err = pprof.StartCPUProfile(output)
		if err != nil {
			return errs.Wrap(err)
		}
		defer pprof.StopCPUProfile()
	}

	if ex.debug.traceFile != "" {
		var output *os.File
		output, err = os.Create(ex.debug.traceFile)
		if err != nil {
			return errs.Wrap(err)
		}
		defer func() {
			err = errs.Combine(err, output.Close())
		}()

		err = trace.Start(output)
		if err != nil {
			return errs.Wrap(err)
		}
		defer trace.Stop()
	}

	if ex.debug.monkitStatsFile != "" {
		fh, err := os.Create(ex.debug.monkitStatsFile)
		if err != nil {
			return errs.Wrap(err)
		}
		defer func() { _ = fh.Close() }()
		defer monkit.Default.Stats(func(key monkit.SeriesKey, field string, val float64) {
			_, _ = fmt.Fprintf(fh, "%v\t%v\t%v\n", key, field, val)
		})
	}

	// N.B.: Tracing is currently disabled by default (sample == 0, traceID == 0) and is
	// something a user can only opt into. as a result, we don't check ex.analyticsEnabled()
	// in this if statement. If we do ever start turning on trace samples by default, we
	// will need to make sure we only do so if ex.analyticsEnabled().
	if ex.tracing.traceAddress != "" && (ex.tracing.sample > 0 || ex.tracing.traceID > 0) {
		versionName := "uplink-release-" + version.Build.Version.String()
		if !version.Build.Release {
			versionName = "uplink-dev"
		}
		collector, err := jaeger.NewThriftCollector(zap.L(), ex.tracing.traceAddress, versionName, nil, 0, 0, 0)
		if err != nil {
			return err
		}
		defer func() {
			_ = collector.Close()
		}()

		defer tracked(ctx, collector.Run)()

		cancel := jaeger.RegisterJaeger(monkit.Default, collector,
			jaeger.Options{
				Fraction: ex.tracing.sample,
				Excluded: tracing.IsExcluded,
			},
		)
		defer cancel()

		if ex.tracing.traceID == 0 {
			if ex.tracing.verbose {
				var printedFirst bool
				monkit.Default.ObserveTraces(func(trace *monkit.Trace) {
					// workaround to hide the traceID of tlsopts.verifyIndentity called from a separated goroutine
					if !printedFirst {
						_, _ = fmt.Fprintf(clingy.Stdout(ctx), "New traceID %x\n", trace.Id())
						printedFirst = true
					}
				})
			}
		} else {
			trace := monkit.NewTrace(ex.tracing.traceID)
			trace.Set(rpctracing.Sampled, true)

			defer mon.Func().RemoteTrace(&ctx, monkit.NewId(), trace)(&err)
		}

		monkit.Default.ObserveTraces(func(trace *monkit.Trace) {
			if hn, err := os.Hostname(); err == nil {
				trace.Set("hostname", hn)
			}
			for k, v := range ex.tracing.tags {
				trace.Set(k, v)
			}
		})
	}

	if ex.analyticsEnabled() && ex.events.address != "" {
		var appname string
		var appversion string
		if version.Build.Release {
			// TODO: eventkit should probably think through
			// application and application version more carefully.
			appname = "uplink-release"
			appversion = version.Build.Version.String()
		} else {
			appname = "uplink-dev"
			appversion = version.Build.Timestamp.Format(time.RFC3339)
		}

		client := eventkit.NewUDPClient(
			appname,
			appversion,
			"",
			ex.events.address,
		)

		defer tracked(ctx, client.Run)()
		eventkit.DefaultRegistry.AddDestination(client)
		eventkit.DefaultRegistry.Scope("init").Event("init")
	}

	var workErr error
	work := func(ctx context.Context) {
		defer mon.Task()(&ctx)(&err)
		workErr = cmd.Execute(ctx)
	}

	var formatter func(io.Writer, []*collect.FinishedSpan) error
	switch {
	default:
		work(ctx)
		return workErr
	case strings.HasSuffix(strings.ToLower(ex.debug.monkitTraceFile), ".svg"):
		formatter = present.SpansToSVG
	case strings.HasSuffix(strings.ToLower(ex.debug.monkitTraceFile), ".json"):
		formatter = present.SpansToJSON
	}

	spans := mpscqueue.New[collect.FinishedSpan]()
	collector := func(s *monkit.Span, err error, panicked bool, finish time.Time) {
		spans.Enqueue(collect.FinishedSpan{
			Span:     s,
			Err:      err,
			Panicked: panicked,
			Finish:   finish,
		})
	}

	defer collect.ObserveAllTraces(monkit.Default, spanCollectorFunc(collector))()
	work(ctx)

	fh, err := os.Create(ex.debug.monkitTraceFile)
	if err != nil {
		return errs.Combine(workErr, err)
	}

	var spanSlice []*collect.FinishedSpan
	for {
		next, ok := spans.Dequeue()
		if !ok {
			break
		}
		spanSlice = append(spanSlice, &next)
	}

	err = formatter(fh, spanSlice)
	return errs.Combine(workErr, err, fh.Close())
}

type spanCollectorFunc func(*monkit.Span, error, bool, time.Time)

func (f spanCollectorFunc) Start(*monkit.Span) {}

func (f spanCollectorFunc) Finish(s *monkit.Span, err error, panicked bool, finish time.Time) {
	f(s, err, panicked, finish)
}

func tracked(ctx context.Context, cb func(context.Context)) (done func()) {
	ctx, cancel := context.WithCancel(ctx)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		cb(ctx)
		wg.Done()
	}()

	return func() {
		cancel()
		wg.Wait()
	}
}

// PromptInput gets a line of input text from the user and returns an error if
// interactive mode is disabled.
func (ex *external) PromptInput(ctx context.Context, prompt string) (input string, err error) {
	if !ex.interactive {
		return "", errs.New("required user input in non-interactive setting")
	}
	_, _ = fmt.Fprint(clingy.Stdout(ctx), prompt, " ")
	var buf []byte
	var tmp [1]byte
	for {
		_, err := clingy.Stdin(ctx).Read(tmp[:])
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return "", errs.Wrap(err)
		} else if tmp[0] == '\n' {
			break
		}
		buf = append(buf, tmp[0])
	}
	return string(bytes.TrimSpace(buf)), nil
}

// PromptSecret gets a line of secret input from the user twice to ensure that
// it is the same value, and returns an error if interactive mode is disabled
// or if the prompt cannot be put into a mode where the typing is not echoed.
func (ex *external) PromptSecret(ctx context.Context, prompt string) (secret string, err error) {
	if !ex.interactive {
		return "", errs.New("required secret input in non-interactive setting")
	}

	fh, ok := clingy.Stdin(ctx).(interface{ Fd() uintptr })
	if !ok {
		return "", errs.New("unable to request secret from stdin")
	}
	fd := int(fh.Fd())

	for {
		_, _ = fmt.Fprint(clingy.Stdout(ctx), prompt, " ")

		first, err := term.ReadPassword(fd)
		if err != nil {
			return "", errs.New("unable to request secret from stdin: %w", err)
		}
		_, _ = fmt.Fprintln(clingy.Stdout(ctx))

		_, _ = fmt.Fprint(clingy.Stdout(ctx), "Again: ")

		second, err := term.ReadPassword(fd)
		if err != nil {
			return "", errs.New("unable to request secret from stdin: %w", err)
		}
		_, _ = fmt.Fprintln(clingy.Stdout(ctx))

		if string(first) != string(second) {
			_, _ = fmt.Fprintln(clingy.Stdout(ctx), "Values did not match. Try again.")
			_, _ = fmt.Fprintln(clingy.Stdout(ctx))
			continue
		}

		return string(first), nil
	}
}

func defaultUplinkSubdir() []string {
	switch runtime.GOOS {
	case "windows", "darwin":
		return []string{"Storj", "Uplink"}
	default:
		return []string{"storj", "uplink"}
	}
}

// appDir returns best base directory for the currently running operating system. It
// has a legacy bool to have it return the same values that storj.io/common/fpath.ApplicationDir
// would have returned.
func appDir(legacy bool, subdir ...string) string {
	var appdir string
	home := os.Getenv("HOME")

	switch runtime.GOOS {
	case "windows":
		// Windows standards: https://msdn.microsoft.com/en-us/library/windows/apps/hh465094.aspx?f=255&MSPPError=-2147217396
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
		// Linux standards: https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html
		if legacy {
			appdir = os.Getenv("XDG_DATA_HOME")
			if appdir == "" && home != "" {
				appdir = filepath.Join(home, ".local", "share")
			}
		} else {
			appdir = os.Getenv("XDG_CONFIG_HOME")
			if appdir == "" && home != "" {
				appdir = filepath.Join(home, ".config")
			}
		}
	}
	return filepath.Join(append([]string{appdir}, subdir...)...)
}
