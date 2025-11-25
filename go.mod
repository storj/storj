module storj.io/storj

go 1.24.7

require (
	cloud.google.com/go v0.121.1
	cloud.google.com/go/profiler v0.4.0
	cloud.google.com/go/pubsub/v2 v2.0.0
	cloud.google.com/go/secretmanager v1.14.5
	cloud.google.com/go/spanner v1.76.1
	cloud.google.com/go/storage v1.53.0
	github.com/alessio/shellescape v1.2.2
	github.com/alicebob/miniredis/v2 v2.13.3
	github.com/blang/semver v3.5.1+incompatible
	github.com/blang/semver/v4 v4.0.0
	github.com/bmkessler/fastdiv v0.0.0-20190227075523-41d5178f2044
	github.com/calebcase/tmpfile v1.0.3
	github.com/coreos/go-oidc/v3 v3.11.0
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc
	github.com/dgraph-io/badger/v4 v4.5.0
	github.com/dsnet/try v0.0.3
	github.com/fatih/color v1.15.0
	github.com/go-oauth2/oauth2/v4 v4.4.2
	github.com/gogo/protobuf v1.3.2
	github.com/golang-jwt/jwt v3.2.1+incompatible
	github.com/google/go-cmp v0.7.0
	github.com/googleapis/go-sql-spanner v1.11.1-0.20250214171559-1bccea5dfec5
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/schema v1.4.1
	github.com/jackc/pgerrcode v0.0.0-20201024163028-a0d42d470451
	github.com/jackc/pgtype v1.14.1
	github.com/jackc/pgx/v5 v5.6.0
	github.com/jtolds/monkit-hw/v2 v2.0.0-20250117140252-1a544613ac79
	github.com/jtolio/mito v0.0.0-20230523171229-d78ef06bb77b
	github.com/jtolio/noiseconn v0.0.0-20230301220541-88105e6c8ac6
	github.com/klauspost/compress v1.17.11
	github.com/linkedin/goavro/v2 v2.13.1
	github.com/loov/hrtime v1.0.3
	github.com/mattn/go-sqlite3 v1.14.28
	github.com/nsf/jsondiff v0.0.0-20200515183724-f29ed568f4ce
	github.com/nsf/termbox-go v0.0.0-20200418040025-38ba6e5628f1
	github.com/oschwald/maxminddb-golang v1.12.0
	github.com/pquerna/otp v1.3.0
	github.com/prometheus/client_golang v1.20.5
	github.com/prometheus/common v0.55.0
	github.com/redis/go-redis/v9 v9.5.1
	github.com/shirou/gopsutil/v3 v3.21.3
	github.com/shopspring/decimal v1.2.0
	github.com/spacemonkeygo/monkit/v3 v3.0.25-0.20251022131615-eb24eb109368
	github.com/spacemonkeygo/tlshowdy v0.0.0-20160207005338-8fa2cec1d7cd
	github.com/spf13/cobra v1.8.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.18.2
	github.com/stretchr/testify v1.11.1
	github.com/stripe/stripe-go/v81 v81.3.1
	github.com/vbauerster/mpb/v8 v8.4.0
	github.com/zeebo/assert v1.3.1
	github.com/zeebo/blake3 v0.2.3
	github.com/zeebo/clingy v0.0.0-20230602044025-906be850f10d
	github.com/zeebo/errs v1.4.0
	github.com/zeebo/errs/v2 v2.0.5
	github.com/zeebo/ini v0.0.0-20210514163846-cc8fbd8d9599
	github.com/zeebo/mwc v0.0.6
	github.com/zeebo/structs v1.0.3-0.20230601144555-f2db46069602
	github.com/zeebo/sudo v1.0.2
	github.com/zeebo/xxh3 v1.0.2
	github.com/zyedidia/generic v1.2.1
	go.etcd.io/bbolt v1.3.5
	go.opentelemetry.io/otel v1.35.0
	go.opentelemetry.io/otel/metric v1.35.0
	go.opentelemetry.io/otel/sdk v1.35.0
	go.opentelemetry.io/otel/trace v1.35.0
	go.uber.org/mock v0.5.2
	go.uber.org/zap v1.27.0
	golang.org/x/crypto v0.45.0
	golang.org/x/exp v0.0.0-20240719175910-8a7402abbf56
	golang.org/x/net v0.47.0
	golang.org/x/oauth2 v0.30.0
	golang.org/x/sync v0.18.0
	golang.org/x/sys v0.38.0
	golang.org/x/term v0.37.0
	golang.org/x/text v0.31.0
	golang.org/x/time v0.12.0
	google.golang.org/api v0.233.0
	google.golang.org/grpc v1.72.0
	google.golang.org/protobuf v1.36.6
	gopkg.in/segmentio/analytics-go.v3 v3.1.0
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/api v0.33.3
	k8s.io/apimachinery v0.33.3
	k8s.io/client-go v0.33.3
	storj.io/common v0.0.0-20251022143549-19bf6a9f274a
	storj.io/drpc v0.0.35-0.20250513201419-f7819ea69b55
	storj.io/eventkit v0.0.0-20250410172343-61f26d3de156
	storj.io/minmaxheap v0.0.0-20250403032542-1e24a6fe9c16
	storj.io/monkit-jaeger v0.0.0-20250523220404-454c1b072fad
	storj.io/uplink v1.13.2-0.20250807183920-f49c2319cb74
)

require (
	cel.dev/expr v0.20.0 // indirect
	cloud.google.com/go/auth v0.16.1 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/bigquery v1.66.2 // indirect
	cloud.google.com/go/compute/metadata v0.6.0 // indirect
	cloud.google.com/go/iam v1.5.2 // indirect
	cloud.google.com/go/longrunning v0.6.7 // indirect
	cloud.google.com/go/monitoring v1.24.0 // indirect
	github.com/GoogleCloudPlatform/grpc-gcp-go/grpcgcp v1.5.2 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.27.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric v0.51.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.51.0 // indirect
	github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/alicebob/gopher-json v0.0.0-20200520072559-a9ecdc9d1d3a // indirect
	github.com/andybalholm/brotli v1.0.6 // indirect
	github.com/apache/arrow/go/v15 v15.0.2 // indirect
	github.com/apache/thrift v0.17.0 // indirect
	github.com/bmizerany/assert v0.0.0-20160611221934-b7ed37b82869 // indirect
	github.com/boombuler/barcode v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cncf/xds/go v0.0.0-20250121191232-2f005788dc42 // indirect
	github.com/dgraph-io/ristretto/v2 v2.0.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/elastic/gosigar v0.14.3 // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/envoyproxy/go-control-plane/envoy v1.32.4 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.2.1 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/flynn/noise v1.0.0 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/go-jose/go-jose/v4 v4.0.5 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/flatbuffers v24.3.25+incompatible // indirect
	github.com/google/gnostic-models v0.6.9 // indirect
	github.com/google/gopacket v1.1.19 // indirect
	github.com/google/pprof v0.0.0-20241029153458-d1b30febd7db // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.6 // indirect
	github.com/googleapis/gax-go/v2 v2.14.1 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jackc/pgio v1.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgproto3/v2 v2.3.2 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/pgx/v4 v4.15.0 // indirect
	github.com/jackc/puddle/v2 v2.2.1 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/jtolds/tracetagger/v2 v2.0.0-rc5 // indirect
	github.com/jtolio/crawlspace v0.0.0-20231116162947-3ec5cc6b36c5 // indirect
	github.com/jtolio/crawlspace/tools v0.0.0-20231116162947-3ec5cc6b36c5 // indirect
	github.com/klauspost/cpuid/v2 v2.2.5 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pelletier/go-toml/v2 v2.1.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.18 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/quic-go/quic-go v0.57.1 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/rogpeppe/go-internal v1.13.1 // indirect
	github.com/sagikazarmark/locafero v0.4.0 // indirect
	github.com/sagikazarmark/slog-shim v0.1.0 // indirect
	github.com/segmentio/backo-go v0.0.0-20200129164019-23eae7c10bd3 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spacemonkeygo/spacelog v0.0.0-20180420211403-2296661a0572 // indirect
	github.com/spf13/afero v1.11.0 // indirect
	github.com/spf13/cast v1.6.0 // indirect
	github.com/spiffe/go-spiffe/v2 v2.5.0 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/tklauser/go-sysconf v0.3.4 // indirect
	github.com/tklauser/numcpus v0.2.1 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/xtgo/uuid v0.0.0-20140804021211-a0b114877d4c // indirect
	github.com/yuin/gopher-lua v0.0.0-20191220021717-ab39c6098bdb // indirect
	github.com/zeebo/admission/v3 v3.0.3 // indirect
	github.com/zeebo/float16 v0.1.0 // indirect
	github.com/zeebo/goof v0.0.0-20230907150950-e9457bc94477 // indirect
	github.com/zeebo/incenc v0.0.0-20180505221441-0d92902eec54 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/detectors/gcp v1.35.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.60.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.60.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.35.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/mod v0.29.0 // indirect
	golang.org/x/telemetry v0.0.0-20251008203120-078029d740a8 // indirect
	golang.org/x/tools v0.38.0 // indirect
	golang.org/x/xerrors v0.0.0-20240903120638-7835f813f4da // indirect
	google.golang.org/genproto v0.0.0-20250303144028-a0af3efb3deb // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250425173222-7b384671a197 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250505200425-f936aa4a68b2 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-openapi v0.0.0-20250318190949-c8a335a9a2ff // indirect
	k8s.io/utils v0.0.0-20241104100929-3ea5e8cea738 // indirect
	sigs.k8s.io/json v0.0.0-20241010143419-9aa6b5e7a4b3 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.6.0 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
	storj.io/infectious v0.0.2 // indirect
	storj.io/picobuf v0.0.4 // indirect
)
