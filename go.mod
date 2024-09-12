module storj.io/storj

go 1.21

toolchain go1.21.0

require (
	cloud.google.com/go v0.115.1
	cloud.google.com/go/secretmanager v1.14.0
	cloud.google.com/go/spanner v1.67.0
	github.com/alessio/shellescape v1.2.2
	github.com/alicebob/miniredis/v2 v2.13.3
	github.com/blang/semver v3.5.1+incompatible
	github.com/calebcase/tmpfile v1.0.3
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc
	github.com/dgraph-io/badger/v4 v4.2.0
	github.com/dsnet/try v0.0.3
	github.com/fatih/color v1.15.0
	github.com/go-oauth2/oauth2/v4 v4.4.2
	github.com/gogo/protobuf v1.3.2
	github.com/golang/mock v1.6.0
	github.com/google/go-cmp v0.6.0
	github.com/googleapis/go-sql-spanner v1.7.1
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/schema v1.2.0
	github.com/jackc/pgerrcode v0.0.0-20201024163028-a0d42d470451
	github.com/jackc/pgtype v1.14.1
	github.com/jackc/pgx/v5 v5.6.0
	github.com/jtolds/monkit-hw/v2 v2.0.0-20191108235325-141a0da276b3
	github.com/jtolio/mito v0.0.0-20230523171229-d78ef06bb77b
	github.com/jtolio/noiseconn v0.0.0-20230301220541-88105e6c8ac6
	github.com/klauspost/compress v1.17.7
	github.com/loov/hrtime v1.0.3
	github.com/mattn/go-sqlite3 v1.14.19
	github.com/nsf/jsondiff v0.0.0-20200515183724-f29ed568f4ce
	github.com/nsf/termbox-go v0.0.0-20200418040025-38ba6e5628f1
	github.com/oschwald/maxminddb-golang v1.12.0
	github.com/pquerna/otp v1.3.0
	github.com/redis/go-redis/v9 v9.5.1
	github.com/shirou/gopsutil/v3 v3.21.3
	github.com/shopspring/decimal v1.2.0
	github.com/spacemonkeygo/monkit/v3 v3.0.23
	github.com/spacemonkeygo/tlshowdy v0.0.0-20160207005338-8fa2cec1d7cd
	github.com/spf13/cobra v1.8.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.18.2
	github.com/stretchr/testify v1.9.0
	github.com/stripe/stripe-go/v75 v75.8.0
	github.com/vbauerster/mpb/v8 v8.4.0
	github.com/zeebo/assert v1.3.1
	github.com/zeebo/blake3 v0.2.3
	github.com/zeebo/clingy v0.0.0-20230602044025-906be850f10d
	github.com/zeebo/errs v1.3.0
	github.com/zeebo/errs/v2 v2.0.5
	github.com/zeebo/ini v0.0.0-20210514163846-cc8fbd8d9599
	github.com/zeebo/structs v1.0.3-0.20230601144555-f2db46069602
	github.com/zyedidia/generic v1.2.1
	go.etcd.io/bbolt v1.3.5
	go.uber.org/zap v1.27.0
	golang.org/x/crypto v0.26.0
	golang.org/x/exp v0.0.0-20231006140011-7918f672742d
	golang.org/x/net v0.28.0
	golang.org/x/oauth2 v0.22.0
	golang.org/x/sync v0.8.0
	golang.org/x/sys v0.24.0
	golang.org/x/term v0.23.0
	golang.org/x/text v0.17.0
	golang.org/x/time v0.6.0
	google.golang.org/api v0.195.0
	google.golang.org/grpc v1.66.0
	google.golang.org/protobuf v1.34.2
	gopkg.in/segmentio/analytics-go.v3 v3.1.0
	gopkg.in/yaml.v3 v3.0.1
	storj.io/common v0.0.0-20240911161944-293dbaeb442c
	storj.io/drpc v0.0.35-0.20240709171858-0075ac871661
	storj.io/eventkit v0.0.0-20240415002644-1d9596fee086
	storj.io/monkit-jaeger v0.0.0-20240221095020-52b0792fa6cd
	storj.io/uplink v1.13.2-0.20240821111301-f296d41bedbb
)

require (
	cel.dev/expr v0.15.0 // indirect
	cloud.google.com/go/auth v0.9.1 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.4 // indirect
	cloud.google.com/go/bigquery v1.62.0 // indirect
	cloud.google.com/go/compute/metadata v0.5.0 // indirect
	cloud.google.com/go/iam v1.2.0 // indirect
	cloud.google.com/go/longrunning v0.6.0 // indirect
	cloud.google.com/go/profiler v0.4.0 // indirect
	github.com/GoogleCloudPlatform/grpc-gcp-go/grpcgcp v1.5.0 // indirect
	github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/alicebob/gopher-json v0.0.0-20200520072559-a9ecdc9d1d3a // indirect
	github.com/andybalholm/brotli v1.0.6 // indirect
	github.com/apache/arrow/go/v15 v15.0.2 // indirect
	github.com/apache/thrift v0.17.0 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/bmizerany/assert v0.0.0-20160611221934-b7ed37b82869 // indirect
	github.com/bmkessler/fastdiv v0.0.0-20190227075523-41d5178f2044 // indirect
	github.com/boombuler/barcode v1.0.1 // indirect
	github.com/census-instrumentation/opencensus-proto v0.4.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudfoundry/gosigar v1.1.0 // indirect
	github.com/cncf/xds/go v0.0.0-20240423153145-555b57ec207b // indirect
	github.com/dgraph-io/ristretto v0.1.1 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/envoyproxy/go-control-plane v0.12.1-0.20240621013728-1eb8caab5155 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.0.4 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/flynn/noise v1.0.0 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/golang-jwt/jwt v3.2.1+incompatible // indirect
	github.com/golang/glog v1.2.1 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/flatbuffers v23.5.26+incompatible // indirect
	github.com/google/pprof v0.0.0-20230602150820-91b7bce49751 // indirect
	github.com/google/s2a-go v0.1.8 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.2 // indirect
	github.com/googleapis/gax-go/v2 v2.13.0 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jackc/pgio v1.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgproto3/v2 v2.3.2 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/pgx/v4 v4.15.0 // indirect
	github.com/jackc/puddle/v2 v2.2.1 // indirect
	github.com/jtolds/tracetagger/v2 v2.0.0-rc5 // indirect
	github.com/jtolio/crawlspace v0.0.0-20231116162947-3ec5cc6b36c5 // indirect
	github.com/jtolio/crawlspace/tools v0.0.0-20231116162947-3ec5cc6b36c5 // indirect
	github.com/klauspost/cpuid/v2 v2.2.5 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/onsi/ginkgo/v2 v2.9.5 // indirect
	github.com/pelletier/go-toml/v2 v2.1.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.18 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/quic-go/qtls-go1-20 v0.4.1 // indirect
	github.com/quic-go/quic-go v0.40.1 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/rogpeppe/go-internal v1.12.0 // indirect
	github.com/sagikazarmark/locafero v0.4.0 // indirect
	github.com/sagikazarmark/slog-shim v0.1.0 // indirect
	github.com/segmentio/backo-go v0.0.0-20200129164019-23eae7c10bd3 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spacemonkeygo/spacelog v0.0.0-20180420211403-2296661a0572 // indirect
	github.com/spf13/afero v1.11.0 // indirect
	github.com/spf13/cast v1.6.0 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/tklauser/go-sysconf v0.3.4 // indirect
	github.com/tklauser/numcpus v0.2.1 // indirect
	github.com/xtgo/uuid v0.0.0-20140804021211-a0b114877d4c // indirect
	github.com/yuin/gopher-lua v0.0.0-20191220021717-ab39c6098bdb // indirect
	github.com/zeebo/admission/v3 v3.0.3 // indirect
	github.com/zeebo/float16 v0.1.0 // indirect
	github.com/zeebo/goof v0.0.0-20230907150950-e9457bc94477 // indirect
	github.com/zeebo/incenc v0.0.0-20180505221441-0d92902eec54 // indirect
	github.com/zeebo/mwc v0.0.4 // indirect
	github.com/zeebo/sudo v1.0.2 // indirect
	github.com/zeebo/xxh3 v1.0.2 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.52.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.53.0 // indirect
	go.opentelemetry.io/otel v1.28.0 // indirect
	go.opentelemetry.io/otel/metric v1.28.0 // indirect
	go.opentelemetry.io/otel/trace v1.28.0 // indirect
	go.uber.org/mock v0.3.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/mod v0.18.0 // indirect
	golang.org/x/tools v0.22.0 // indirect
	golang.org/x/xerrors v0.0.0-20240716161551-93cc26a95ae9 // indirect
	google.golang.org/genproto v0.0.0-20240827150818-7e3bb234dfed // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240823204242-4ba0660f739c // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240827150818-7e3bb234dfed // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	storj.io/infectious v0.0.2 // indirect
	storj.io/picobuf v0.0.3 // indirect
)
