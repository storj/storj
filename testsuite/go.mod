module storj.io/storj/testsuite

go 1.16

replace storj.io/storj => ../

require (
<<<<<<< HEAD
	github.com/go-rod/rod v0.101.5
	github.com/stretchr/testify v1.7.0
	github.com/zeebo/errs v1.2.2
	go.uber.org/zap v1.18.1
	storj.io/common v0.0.0-20210805230333-1ba6d8c2bfb1
	storj.io/storj v1.35.3
=======
	github.com/go-rod/rod v0.100.0
	github.com/stretchr/testify v1.7.0
	github.com/zeebo/errs v1.2.2
	go.uber.org/zap v1.17.0
	storj.io/common v0.0.0-20210805073808-8e0feb09e92a
	storj.io/storj v0.0.0-00010101000000-000000000000
>>>>>>> 7796e9c8a... integration:  add uitest options for running gateway-mt and authservice
)
