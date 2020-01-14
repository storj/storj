// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cfgstruct

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/storj"
)

func assertEqual(actual, expected interface{}) {
	if !reflect.DeepEqual(actual, expected) {
		panic(fmt.Sprintf("expected %v, got %v", expected, actual))
	}
}

func TestBind(t *testing.T) {
	f := pflag.NewFlagSet("test", pflag.PanicOnError)
	var c struct {
		String   string         `default:""`
		Bool     bool           `releaseDefault:"false" devDefault:"true"`
		Int64    int64          `releaseDefault:"0" devDefault:"1"`
		Int      int            `default:"0"`
		Uint64   uint64         `default:"0"`
		Uint     uint           `default:"0"`
		Float64  float64        `default:"0"`
		Size     memory.Size    `default:"0"`
		Duration time.Duration  `default:"0"`
		NodeURL  storj.NodeURL  `releaseDefault:"" devDefault:""`
		NodeURLs storj.NodeURLs `releaseDefault:"" devDefault:""`
		Struct   struct {
			AnotherString string `default:""`
		}
		Fields [10]struct {
			AnotherInt int `default:"0"`
		}
	}
	Bind(f, &c, UseReleaseDefaults())

	assertEqual(c.String, string(""))
	assertEqual(c.Bool, bool(false))
	assertEqual(c.Int64, int64(0))
	assertEqual(c.Int, int(0))
	assertEqual(c.Uint64, uint64(0))
	assertEqual(c.Uint, uint(0))
	assertEqual(c.Float64, float64(0))
	assertEqual(c.Size, memory.Size(0))
	assertEqual(c.Duration, time.Duration(0))
	assertEqual(c.NodeURL, storj.NodeURL{})
	assertEqual(c.NodeURLs, storj.NodeURLs(nil))
	assertEqual(c.Struct.AnotherString, string(""))
	assertEqual(c.Fields[0].AnotherInt, int(0))
	assertEqual(c.Fields[3].AnotherInt, int(0))

	node1, err := storj.NodeIDFromString("12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S")
	require.NoError(t, err)
	node2, err := storj.NodeIDFromString("12L9ZFwhzVpuEKMUNUqkaTLGzwY9G24tbiigLiXpmZWKwmcNDDs")
	require.NoError(t, err)

	err = f.Parse([]string{
		"--string=1",
		"--bool=true",
		"--int64=1",
		"--int=1",
		"--uint64=1",
		"--uint=1",
		"--float64=1",
		"--size=1MiB",
		"--duration=1h",
		"--node-url=12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S@mars.tardigrade.io:7777",
		"--node-ur-ls=12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S@mars.tardigrade.io:7777,12L9ZFwhzVpuEKMUNUqkaTLGzwY9G24tbiigLiXpmZWKwmcNDDs@jupiter.tardigrade.io:7777",
		"--struct.another-string=1",
		"--fields.03.another-int=1"})
	if err != nil {
		panic(err)
	}
	assertEqual(c.String, string("1"))
	assertEqual(c.Bool, bool(true))
	assertEqual(c.Int64, int64(1))
	assertEqual(c.Int, int(1))
	assertEqual(c.Uint64, uint64(1))
	assertEqual(c.Uint, uint(1))
	assertEqual(c.Float64, float64(1))
	assertEqual(c.Size, memory.MiB)
	assertEqual(c.Duration, time.Hour)
	assertEqual(c.NodeURL, storj.NodeURL{ID: node1, Address: "mars.tardigrade.io:7777"})
	assertEqual(c.NodeURLs, storj.NodeURLs{
		storj.NodeURL{ID: node1, Address: "mars.tardigrade.io:7777"},
		storj.NodeURL{ID: node2, Address: "jupiter.tardigrade.io:7777"},
	})
	assertEqual(c.Struct.AnotherString, string("1"))
	assertEqual(c.Fields[0].AnotherInt, int(0))
	assertEqual(c.Fields[3].AnotherInt, int(1))
}

func TestConfDir(t *testing.T) {
	f := pflag.NewFlagSet("test", pflag.PanicOnError)
	var c struct {
		String    string `default:"-$CONFDIR+"`
		MyStruct1 struct {
			String    string `default:"1${CONFDIR}2"`
			MyStruct2 struct {
				String string `default:"2${CONFDIR}3"`
			}
		}
	}
	Bind(f, &c, UseReleaseDefaults(), ConfDir("confpath"))
	assertEqual(f.Lookup("string").DefValue, "-confpath+")
	assertEqual(f.Lookup("my-struct1.string").DefValue, "1confpath2")
	assertEqual(f.Lookup("my-struct1.my-struct2.string").DefValue, "2confpath3")
}

func TestBindDevDefaults(t *testing.T) {
	f := pflag.NewFlagSet("test", pflag.PanicOnError)
	var c struct {
		String   string         `default:"dev"`
		Bool     bool           `releaseDefault:"false" devDefault:"true"`
		Int64    int64          `releaseDefault:"0" devDefault:"1"`
		Int      int            `default:"2"`
		Uint64   uint64         `default:"3"`
		Uint     uint           `releaseDefault:"0" devDefault:"4"`
		Float64  float64        `default:"5.5"`
		Duration time.Duration  `default:"1h"`
		NodeURL  storj.NodeURL  `releaseDefault:"" devDefault:"12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S@mars.tardigrade.io:7777"`
		NodeURLs storj.NodeURLs `releaseDefault:"" devDefault:"12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S@mars.tardigrade.io:7777,12L9ZFwhzVpuEKMUNUqkaTLGzwY9G24tbiigLiXpmZWKwmcNDDs@jupiter.tardigrade.io:7777"`
		Struct   struct {
			AnotherString string `default:"dev2"`
		}
		Fields [10]struct {
			AnotherInt int `default:"6"`
		}
	}
	Bind(f, &c, UseDevDefaults())

	node1, err := storj.NodeIDFromString("12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S")
	require.NoError(t, err)
	node2, err := storj.NodeIDFromString("12L9ZFwhzVpuEKMUNUqkaTLGzwY9G24tbiigLiXpmZWKwmcNDDs")
	require.NoError(t, err)

	assertEqual(c.String, string("dev"))
	assertEqual(c.Bool, bool(true))
	assertEqual(c.Int64, int64(1))
	assertEqual(c.Int, int(2))
	assertEqual(c.Uint64, uint64(3))
	assertEqual(c.Uint, uint(4))
	assertEqual(c.Float64, float64(5.5))
	assertEqual(c.Duration, time.Hour)
	assertEqual(c.NodeURL, storj.NodeURL{ID: node1, Address: "mars.tardigrade.io:7777"})
	assertEqual(c.NodeURLs, storj.NodeURLs{
		storj.NodeURL{ID: node1, Address: "mars.tardigrade.io:7777"},
		storj.NodeURL{ID: node2, Address: "jupiter.tardigrade.io:7777"},
	})
	assertEqual(c.Struct.AnotherString, string("dev2"))
	assertEqual(c.Fields[0].AnotherInt, int(6))
	assertEqual(c.Fields[3].AnotherInt, int(6))

	node3, err := storj.NodeIDFromString("121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6")
	require.NoError(t, err)

	err = f.Parse([]string{
		"--string=1",
		"--bool=true",
		"--int64=1",
		"--int=1",
		"--uint64=1",
		"--uint=1",
		"--float64=1",
		"--duration=1h",
		"--node-url=121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@saturn.tardigrade.io:7777",
		"--node-ur-ls=121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@saturn.tardigrade.io:7777",
		"--struct.another-string=1",
		"--fields.03.another-int=1"})
	if err != nil {
		panic(err)
	}
	assertEqual(c.String, string("1"))
	assertEqual(c.Bool, bool(true))
	assertEqual(c.Int64, int64(1))
	assertEqual(c.Int, int(1))
	assertEqual(c.Uint64, uint64(1))
	assertEqual(c.Uint, uint(1))
	assertEqual(c.Float64, float64(1))
	assertEqual(c.Duration, time.Hour)
	assertEqual(c.NodeURL, storj.NodeURL{ID: node3, Address: "saturn.tardigrade.io:7777"})
	assertEqual(c.NodeURLs, storj.NodeURLs{storj.NodeURL{ID: node3, Address: "saturn.tardigrade.io:7777"}})
	assertEqual(c.Struct.AnotherString, string("1"))
	assertEqual(c.Fields[0].AnotherInt, int(6))
	assertEqual(c.Fields[3].AnotherInt, int(1))
}

func TestHiddenDev(t *testing.T) {
	f := pflag.NewFlagSet("test", pflag.PanicOnError)
	var c struct {
		String  string      `default:"dev" hidden:"true"`
		String2 string      `default:"dev" hidden:"false"`
		Bool    bool        `releaseDefault:"false" devDefault:"true" hidden:"true"`
		Int64   int64       `releaseDefault:"0" devDefault:"1"`
		Int     int         `default:"2"`
		Size    memory.Size `default:"0" hidden:"true"`
	}
	Bind(f, &c, UseDevDefaults())

	flagString := f.Lookup("string")
	flagStringHide := f.Lookup("string2")
	flagBool := f.Lookup("bool")
	flagInt64 := f.Lookup("int64")
	flagInt := f.Lookup("int")
	flagSize := f.Lookup("size")
	assertEqual(flagString.Hidden, true)
	assertEqual(flagStringHide.Hidden, false)
	assertEqual(flagBool.Hidden, true)
	assertEqual(flagInt64.Hidden, false)
	assertEqual(flagInt.Hidden, false)
	assertEqual(flagSize.Hidden, true)
}

func TestHiddenRelease(t *testing.T) {
	f := pflag.NewFlagSet("test", pflag.PanicOnError)
	var c struct {
		String  string `default:"dev" hidden:"false"`
		String2 string `default:"dev" hidden:"true"`
		Bool    bool   `releaseDefault:"false" devDefault:"true" hidden:"true"`
		Int64   int64  `releaseDefault:"0" devDefault:"1"`
		Int     int    `default:"2"`
	}
	Bind(f, &c, UseReleaseDefaults())

	flagString := f.Lookup("string")
	flagStringHide := f.Lookup("string2")
	flagBool := f.Lookup("bool")
	flagInt64 := f.Lookup("int64")
	flagInt := f.Lookup("int")
	assertEqual(flagString.Hidden, false)
	assertEqual(flagStringHide.Hidden, true)
	assertEqual(flagBool.Hidden, true)
	assertEqual(flagInt64.Hidden, false)
	assertEqual(flagInt.Hidden, false)
}

func TestSource(t *testing.T) {
	var c struct {
		Unset string
		Any   string `source:"any"`
		Flag  string `source:"flag"`
	}

	f := pflag.NewFlagSet("test", pflag.PanicOnError)
	Bind(f, &c, UseReleaseDefaults())

	unset := f.Lookup("unset")
	require.NotNil(t, unset)
	require.Empty(t, unset.Annotations)

	any := f.Lookup("any")
	require.NotNil(t, any)
	require.Equal(t, map[string][]string{
		"source": {"any"},
	}, any.Annotations)

	flag := f.Lookup("flag")
	require.NotNil(t, flag)
	require.Equal(t, map[string][]string{
		"source": {"flag"},
	}, flag.Annotations)
}
