# Storj

[![Go Report Card](https://goreportcard.com/badge/github.com/storj/storj)](https://goreportcard.com/report/github.com/storj/storj)
[![Go Doc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](http://godoc.org/github.com/storj/storj)
[![Coverage Status](https://coveralls.io/repos/github/storj/storj/badge.svg?branch=master)](https://coveralls.io/github/storj/storj?branch=master)

<img src="https://github.com/storj/storj/raw/master/logo/logo.png" width="100">

Storj is in the midst of a rearchitecture. Please stay tuned for our v3 whitepaper!

----

Storj is a platform, token, and suite of decentralized applications that allows you to store data in a secure and decentralized manner. Your files are encrypted, shredded into little pieces and stored in a global decentralized network of computers. Luckily, we also support allowing you (and only you) to recover them!

# Start Using Storj

### Download the latest release

Go here to download the latest build
// TODO: add link when a build is released
// TODO for how to run the release

### Configure AWS CLI

In a new terminal session:

Download and install the AWS CLI: https://docs.aws.amazon.com/cli/latest/userguide/installing.html

Configure the AWS CLI:
```bash
$ aws configure
AWS Access Key ID [None]: insecure-dev-access-key
AWS Secret Access Key [None]: insecure-dev-secret-key
Default region name [None]: us-east-1
Default output format [None]: 
$ aws configure set default.s3.multipart_threshold 1TB  # until we support multipart
```
Test some commands:

### Upload an Object

```bash
$ aws s3 --endpoint=http://localhost:7777/ cp ~/Desktop/your-large-file.mp4 s3://bucket/your-large-file.mp4
```

### Download an Object

```bash
$ aws s3 --endpoint=http://localhost:7777/ cp s3://bucket/your-large-file.mp4 ~/Desktop/your-large-file.mp4
```

### List Objects

```bash
aws s3 --endpoint=http://localhost:7777/ ls s3://bucket/ --recursive
```


### Delete Objects in a Bucket

```bash
aws s3 --endpoint=http://localhost:7777/ rm --recursive  s3://bucket/
```

### Generate a URL for an Object

```bash
aws s3 --endpoint=http://localhost:7777/ presign s3://bucket/your-large-file.mp4
```

For more information about the AWS s3 CLI visit: https://docs.aws.amazon.com/cli/latest/reference/s3/index.html


# Start Contibuting to Storj

### Install required packages

First, install git and golang. We currently support Debian-based and Mac operating systems

#### Debian based (like Ubuntu)

Download and install the latest release of go https://golang.org/

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

### Start the network

```bash
$ go install -v storj.io/storj/cmd/captplanet
$ captplanet setup
$ captplanet run
```

### Try out some commands via Storj CLI or AWS CLI

### Run unit tests

```bash
go test storj.io/storj/...
```

You can execute only a single test package. For example: `go test storj.io/storj/pkg/kademlia`. Add -v for more informations about the executed unit tests.

## Support

If you have any questions or suggestions please reach out to us on [Rocketchat](https://community.storj.io/) or [Twitter](https://twitter.com/storjproject).


