# Storj

[![Go Report Card](https://goreportcard.com/badge/github.com/storj/storj)](https://goreportcard.com/report/github.com/storj/storj)
[![Go Doc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](http://godoc.org/github.com/storj/storj)
[![Coverage Status](https://coveralls.io/repos/github/storj/storj/badge.svg?branch=master)](https://coveralls.io/github/storj/storj?branch=master)

<img src="https://github.com/storj/storj/raw/master/logo/logo.png" width="100">

Storj is in the midst of a rearchitecture. Please stay tuned for our v3 whitepaper!

----

Storj is a platform, token, and suite of decentralized applications that allows you to store data in a secure and decentralized manner. Your files are encrypted, shredded into little pieces and stored in a global decentralized network of computers. Luckily, we also support allowing you (and only you) to recover them!

## To start developing

### Install required packages

First of all, install `git` and `golang`.

#### Debian based (like Ubuntu)

```bash
apt-get install git golang
echo 'export STORJDEV="$HOME/storj"' >> $HOME/.bashrc
echo 'export GOPATH="$STORJDEV:$STORJDEV/vendor"' >> $HOME/.bashrc
echo 'export PATH="$PATH:$STORJDEV/bin"' >> $HOME/.bashrc
source $HOME/.bashrc
```

#### Mac OSX

```bash
brew install git go
if test -e $HOME/.bash_profile
then
	echo 'export STORJDEV="$HOME/storj"' >> $HOME/.bash_profile
	echo 'export GOPATH="$STORJDEV:$STORJDEV/vendor"' >> $HOME/.bash_profile
	echo 'export PATH="$PATH:$STORJDEV/bin"' >> $HOME/.bash_profile
	source $HOME/.bash_profile
else
	echo 'export STORJDEV="$HOME/storj"' >> $HOME/.profile
	echo 'export GOPATH="$STORJDEV:$STORJDEV/vendor"' >> $HOME/.profile
	echo 'export PATH="$PATH:$STORJDEV/bin"' >> $HOME/.profile
	source $HOME/.profile
fi
```

### Install storj

Clone the storj repository. You may want to clone your own fork and branch.

```bash
mkdir -p $STORJDEV/src/storj.io
git clone https://github.com/storj/storj $STORJDEV/src/storj.io/storj
```

#### Install all dependencies

```bash
git clone --recursive https://github.com/storj/storj-vendor $STORJDEV/vendor
rm -rf $STORJDEV/vendor/src/github.com/minio/minio/vendor/github.com/minio/cli
rm -rf $STORJDEV/vendor/src/github.com/minio/minio/vendor/golang.org/x/net/trace
```

### Run unit tests

```bash
go clean -testcache
go test storj.io/storj/...
```

You can execute only a single test package. For example: `go test storj.io/storj/pkg/kademlia`. Add -v for more informations about the executed unit tests.

### Start the network

```bash
$ go install -v storj.io/storj/cmd/captplanet
$ captplanet setup
$ captplanet run
```

### Configure AWS CLI

Download and install the AWS CLI: https://docs.aws.amazon.com/cli/latest/userguide/installing.html

```bash
$ aws configure
AWS Access Key ID [None]: insecure-dev-access-key
AWS Secret Access Key [None]: insecure-dev-secret-key
Default region name [None]: us-east-1
Default output format [None]: 
$ aws configure set default.s3.multipart_threshold 1TB  # until we support multipart
```

### Do an upload

```bash
$ aws s3 --endpoint=http://localhost:7777/ cp large-file s3://bucket/large-file
```

## Support

If you have any questions or suggestions please reach out to us on [Rocketchat](https://community.storj.io/) or [Twitter](https://twitter.com/storjproject).
