module storj.io/storj

go 1.13

require (
	github.com/alessio/shellescape v1.2.2
	github.com/alicebob/miniredis/v2 v2.13.3
	github.com/bmizerany/assert v0.0.0-20160611221934-b7ed37b82869 // indirect
	github.com/btcsuite/btcutil v1.0.3-0.20201208143702-a53e38424cce
	github.com/calebcase/tmpfile v1.0.2
	github.com/cheggaaa/pb/v3 v3.0.5
	github.com/fatih/color v1.9.0
	github.com/go-redis/redis/v8 v8.7.1
	github.com/gogo/protobuf v1.3.2
	github.com/google/go-cmp v0.5.5
	github.com/google/pprof v0.0.0-20200229191704-1ebb73c60ed3 // indirect
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/schema v1.2.0
	github.com/graphql-go/graphql v0.7.9
	github.com/jackc/pgconn v1.8.0
	github.com/jackc/pgerrcode v0.0.0-20201024163028-a0d42d470451
	github.com/jackc/pgtype v1.6.2
	github.com/jackc/pgx/v4 v4.10.1
	github.com/jtolds/monkit-hw/v2 v2.0.0-20191108235325-141a0da276b3
	github.com/loov/hrtime v1.0.3
	github.com/mattn/go-sqlite3 v1.14.8
	github.com/nsf/jsondiff v0.0.0-20200515183724-f29ed568f4ce
	github.com/nsf/termbox-go v0.0.0-20200418040025-38ba6e5628f1
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pquerna/otp v1.3.0
	github.com/segmentio/backo-go v0.0.0-20200129164019-23eae7c10bd3 // indirect
	github.com/shopspring/decimal v1.2.0
	github.com/spacemonkeygo/monkit/v3 v3.0.14
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.7.0
	github.com/stripe/stripe-go/v72 v72.51.0
	github.com/vivint/infectious v0.0.0-20200605153912-25a574ae18a3
	github.com/xtgo/uuid v0.0.0-20140804021211-a0b114877d4c // indirect
	github.com/zeebo/assert v1.3.0
	github.com/zeebo/clingy v0.0.0-20210622223751-00a909f86ea9
	github.com/zeebo/errs v1.2.2
	github.com/zeebo/ini v0.0.0-20210331155437-86af75b4f524
	go.etcd.io/bbolt v1.3.5
	go.uber.org/multierr v1.6.0 // indirect
	go.uber.org/zap v1.16.0
	golang.org/x/crypto v0.0.0-20210415154028-4f45737414dc
	golang.org/x/net v0.0.0-20210428140749-89ef3d95e781
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20210510120138-977fb7262007
	golang.org/x/term v0.0.0-20201126162022-7de9c90e9dd1
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e
	google.golang.org/api v0.20.0 // indirect
	google.golang.org/genproto v0.0.0-20200526211855-cb27e3aa2013 // indirect
	gopkg.in/segmentio/analytics-go.v3 v3.1.0
	gopkg.in/yaml.v3 v3.0.0-20200313102051-9f266ea9e77c
	storj.io/common v0.0.0-20210826155949-75e6d164aff6
	storj.io/drpc v0.0.24
	storj.io/monkit-jaeger v0.0.0-20210426161729-debb1cbcbbd7
	storj.io/private v0.0.0-20210625132526-af46b647eda5
	storj.io/uplink v1.5.0-rc.1.0.20210820085250-799c214b35ac
)
