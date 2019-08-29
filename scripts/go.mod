module storj.io/storj/scripts

go 1.12

require (
	github.com/AlekSi/gocov-xml v0.0.0-20190121064608-3a14fb1c4737
	github.com/akavel/rsrc v0.8.0 // indirect
	github.com/axw/gocov v1.0.0
	github.com/ckaznocha/protoc-gen-lint v0.2.1
	github.com/go-bindata/go-bindata v3.1.2+incompatible
	github.com/gogo/protobuf v1.2.1
	github.com/golang/protobuf v1.3.2 // indirect
	github.com/golangci/golangci-lint v1.17.1
	github.com/josephspurrier/goversioninfo v0.0.0-20190209210621-63e6d1acd3dd
	github.com/kylelemons/godebug v1.1.0
	github.com/loov/leakcheck v0.0.3
	github.com/mfridman/tparse v0.7.4
	github.com/nilslice/protolock v0.14.0
	github.com/zeebo/errs v1.2.2
	golang.org/x/tools v0.0.0-20190829051458-42f498d34c4d
	gopkg.in/spacemonkeygo/dbx.v1 v1.0.0-20190212172312-3af5e1fc2659
)

// golangci-lint has some dependencies on incorrectly published modules so we
// use replace directives to have them go to the correct pseudo-version for the
// hash that is desired.
//
// this was done by inspecting an error like
//
// 	go: github.com/golangci/go-tools@v0.0.0-20180109140146-af6baa5dc196: unexpected status
//
// and adding a replace directive like
//
// 	replace github.com/golangci/go-tools v0.0.0-20180109140146-af6baa5dc196 => github.com/golangci/go-tools af6baa5dc196
//
// and running `go mod tidy`.
replace (
	github.com/go-critic/go-critic v0.0.0-20181204210945-1df300866540 => github.com/go-critic/go-critic v0.3.5-0.20190526074819-1df300866540
	github.com/golangci/errcheck v0.0.0-20181003203344-ef45e06d44b6 => github.com/golangci/errcheck v0.0.0-20181223084120-ef45e06d44b6
	github.com/golangci/go-tools v0.0.0-20180109140146-af6baa5dc196 => github.com/golangci/go-tools v0.0.0-20190318060251-af6baa5dc196
	github.com/golangci/gofmt v0.0.0-20181105071733-0b8337e80d98 => github.com/golangci/gofmt v0.0.0-20181222123516-0b8337e80d98
	github.com/golangci/gosec v0.0.0-20180901114220-66fb7fc33547 => github.com/golangci/gosec v0.0.0-20190211064107-66fb7fc33547
	github.com/golangci/ineffassign v0.0.0-20180808204949-42439a7714cc => github.com/golangci/ineffassign v0.0.0-20190609212857-42439a7714cc
	github.com/golangci/lint-1 v0.0.0-20180610141402-ee948d087217 => github.com/golangci/lint-1 v0.0.0-20190420132249-ee948d087217
	mvdan.cc/unparam v0.0.0-20190124213536-fbb59629db34 => mvdan.cc/unparam v0.0.0-20190209190245-fbb59629db34
)
