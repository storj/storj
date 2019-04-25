// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build ignore

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/mfridman/tparse/parse"
)

var xunit = flag.String("out", "", "xunit output file")

func main() {
	flag.Parse()

	if *xunit == "" {
		fmt.Fprintf(os.Stderr, "xunit file not specified\n")
		os.Exit(1)
	}

	var buffer bytes.Buffer
	stdin := io.TeeReader(os.Stdin, &buffer)

	pkgs, err := ProcessWithEcho(stdin)
	if err != nil {
		if err == parse.ErrNotParseable {
			fmt.Fprintf(os.Stderr, "tparse error: no parseable events: call go test with -json flag\n\n")
		} else {
			fmt.Fprintf(os.Stderr, "tparse error: %v\n\n", err)
		}
	}
	defer os.Exit(pkgs.ExitCode())

	output, err := os.Create(*xunit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create error: %v\n\n", err)
		return
	}
	defer func() {
		if err := output.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "close error: %v\n\n", err)
		}
	}()

	_, _ = output.Write([]byte(xml.Header))

	encoder := xml.NewEncoder(output)
	encoder.Indent("", "\t")
	defer encoder.Flush()

	encoder.EncodeToken(xml.StartElement{Name: xml.Name{Local: "testsuites"}, Attr: nil})
	defer encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "testsuites"}})

	for _, pkg := range pkgs {
		failed := pkg.TestsByAction(parse.ActionFail)
		skipped := pkg.TestsByAction(parse.ActionSkip)
		passed := pkg.TestsByAction(parse.ActionPass)

		skipped = withoutEmptyName(skipped)

		all := []*parse.Test{}
		all = append(all, failed...)
		all = append(all, skipped...)
		all = append(all, passed...)

		if !pkg.HasPanic && (pkg.NoTests || len(all) == 0) {
			continue
		}

		func() {
			encoder.EncodeToken(xml.StartElement{
				Name: xml.Name{Local: "testsuite"},
				Attr: []xml.Attr{
					{xml.Name{Local: "name"}, pkg.Summary.Package},
					{xml.Name{Local: "time"}, fmt.Sprintf("%.2f", pkg.Summary.Elapsed)},

					{xml.Name{Local: "tests"}, strconv.Itoa(len(all))},
					{xml.Name{Local: "failures"}, strconv.Itoa(len(failed))},
					{xml.Name{Local: "skips"}, strconv.Itoa(len(skipped))},
				},
			})
			defer encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "testsuite"}})

			if pkg.HasPanic {
				encoder.EncodeToken(xml.StartElement{
					Name: xml.Name{Local: "testcase"},
					Attr: []xml.Attr{
						{xml.Name{Local: "classname"}, pkg.Summary.Package},
						{xml.Name{Local: "name"}, "Panic"},
					},
				})
				encoder.EncodeToken(xml.StartElement{Name: xml.Name{Local: "failure"}, Attr: nil})
				encoder.EncodeToken(xml.CharData(eventOutput(pkg.PanicEvents)))
				encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "failure"}})

				encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "testcase"}})
			}

			for _, t := range all {
				t.SortEvents()
				func() {
					encoder.EncodeToken(xml.StartElement{
						Name: xml.Name{Local: "testcase"},
						Attr: []xml.Attr{
							{xml.Name{Local: "classname"}, t.Package},
							{xml.Name{Local: "name"}, t.Name},
							{xml.Name{Local: "time"}, fmt.Sprintf("%.2f", t.Elapsed())},
						},
					})
					defer encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "testcase"}})

					encoder.EncodeToken(xml.StartElement{xml.Name{Local: "system-out"}, nil})
					encoder.EncodeToken(xml.CharData(eventOutput(t.Events)))
					encoder.EncodeToken(xml.EndElement{xml.Name{Local: "system-out"}})

					switch t.Status() {
					case parse.ActionSkip:
						encoder.EncodeToken(xml.StartElement{
							Name: xml.Name{Local: "skipped"},
							Attr: []xml.Attr{
								{xml.Name{Local: "message"}, t.Stack()},
							},
						})
						encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "skipped"}})
					case parse.ActionFail:
						encoder.EncodeToken(xml.StartElement{Name: xml.Name{Local: "failure"}, Attr: nil})
						encoder.EncodeToken(xml.CharData(t.Stack()))
						encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "failure"}})
					}
				}()
			}
		}()
	}
}

func eventOutput(events parse.Events) string {
	var out strings.Builder
	for _, event := range events {
		out.WriteString(event.Output)
	}
	return out.String()
}

func withoutEmptyName(tests []*parse.Test) []*parse.Test {
	out := tests[:0]
	for _, test := range tests {
		if test.Name != "" {
			out = append(out, test)
		}
	}
	return out
}

// Code based on: https://github.com/mfridman/tparse/blob/master/parse/process.go#L27
func ProcessWithEcho(r io.Reader) (parse.Packages, error) {
	pkgs := parse.Packages{}

	var hasRace bool

	var scan bool
	var badLines int

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		// Scan up-to 50 lines for a parseable event, if we get one, expect
		// no errors to follow until EOF.
		event, err := parse.NewEvent(scanner.Bytes())
		if err != nil {
			badLines++
			if scan || badLines > 50 {
				switch err.(type) {
				case *json.SyntaxError:
					return nil, parse.ErrNotParseable
				default:
					return nil, err
				}
			}
			continue
		}
		scan = true

		if line := strings.TrimRightFunc(event.Output, unicode.IsSpace); line != "" {
			fmt.Fprintln(os.Stdout, line)
		}

		pkg, ok := pkgs[event.Package]
		if !ok {
			pkg = parse.NewPackage()
			pkgs[event.Package] = pkg
		}

		if event.IsPanic() {
			pkg.HasPanic = true
			pkg.Summary.Action = parse.ActionFail
			pkg.Summary.Package = event.Package
			pkg.Summary.Test = event.Test
		}
		// Short circuit output when panic is detected.
		if pkg.HasPanic {
			pkg.PanicEvents = append(pkg.PanicEvents, event)
			continue
		}

		if event.IsRace() {
			hasRace = true
		}

		if event.IsCached() {
			pkg.Cached = true
		}

		if event.NoTestFiles() {
			pkg.NoTestFiles = true
			// Manually mark [no test files] as "pass", because the go test tool reports the
			// package Summary action as "skip".
			pkg.Summary.Package = event.Package
			pkg.Summary.Action = parse.ActionPass
		}
		if event.NoTestsWarn() {
			// One or more tests within the package contains no tests.
			pkg.NoTestSlice = append(pkg.NoTestSlice, event)
		}

		if event.NoTestsToRun() {
			// Only pkgs marked as "pass" will contain a summary line appended with [no tests to run].
			// This indicates one or more tests is marked as having no tests to run.
			pkg.NoTests = true
			pkg.Summary.Package = event.Package
			pkg.Summary.Action = parse.ActionPass
		}

		if event.LastLine() {
			pkg.Summary = event
			continue
		}

		cover, ok := event.Cover()
		if ok {
			pkg.Cover = true
			pkg.Coverage = cover
		}

		if !event.Discard() {
			pkg.AddEvent(event)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("bufio scanner error: %v", err)
	}
	if !scan {
		return nil, parse.ErrNotParseable
	}
	if hasRace {
		return nil, parse.ErrRaceDetected
	}

	return pkgs, nil
}
