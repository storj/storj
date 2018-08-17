# Storj V3 Netowrk

[![Go Report Card](https://goreportcard.com/badge/github.com/storj/storj)](https://goreportcard.com/report/github.com/storj/storj)
[![Go Doc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](http://godoc.org/github.com/storj/storj)
[![Coverage Status](https://coveralls.io/repos/github/storj/storj/badge.svg?branch=master)](https://coveralls.io/github/storj/storj?branch=master)

<img src="https://github.com/storj/storj/raw/master/logo/logo.png" width="100">

Storj is in the midst of a rearchitecture. Please stay tuned for our v3 whitepaper!

----

Storj is a platform, token, and suite of decentralized applications that allows you to store data in a secure and decentralized manner. Your files are encrypted, shredded into little pieces and stored in a global decentralized network of computers. Luckily, we also support allowing you (and only you) to recover them!

## Table of Contents

- [Storj CLI](#storjcli)
- [AWS S3 CLI](#awss3cli)
- [Run Storj Locally](#storjlocal)
- [Support](#support)


# Start Using Storj


#### Download the latest release

Go here to download the latest build
// TODO: add link when a build is released
// TODO for how to run the release

## Using Storj via the Storj CLI <a name="storjcli"></a>

#### Configure the Storj CLI
1) In a new terminal setup the Storj CLI: ```$ storj setup```
2) Edit the API Key, overlay address, and pointer db address fields in the Storj CLI config file located at ```~/.storj/cli/config.yaml``` with values from the captplanet config file located at ```~/.storj/capt/config.yaml```

#### Test out some Storj CLI commands!

1) Create a bucket: ```$ storj mb s3://bucket-name```
2) Upload an object: ```$ storj cp ~/Desktop/your-large-file.mp4 s3://bucket-name```
3) List objects in a bucket: ```$ storj ls s3://bucket-name/ ```
4) Download an object: ```$ storj cp s3://bucket-name/your-large-file.mp4 ~/Desktop/your-large-file.mp4```
6) Delete an object: ```$ storj rm s3://bucket-name/your-large-file.mp4```

---

## Using Storj via the AWS S3 CLI <a name="awss3cli"></a>

#### Configure AWS CLI

Download and install the AWS S3 CLI: https://docs.aws.amazon.com/cli/latest/userguide/installing.html

In a new terminal session configure the AWS S3 CLI:
```bash
$ aws configure
AWS Access Key ID [None]: insecure-dev-access-key
AWS Secret Access Key [None]: insecure-dev-secret-key
Default region name [None]: us-east-1
Default output format [None]: 
$ aws configure set default.s3.multipart_threshold 1TB  # until we support multipart
```

#### Test out some AWS S3 CLI commands! 

1) Create a bucket: ```$ aws s3 --endpoint=http://localhost:7777/ mb s3://bucket-name```
2) Upload an object: ```$ aws s3 --endpoint=http://localhost:7777/ cp ~/Desktop/your-large-file.mp4 s3://bucket-name```
3) List objects in a bucket: ```$ aws s3 --endpoint=http://localhost:7777/ ls s3://bucket-name/ ```
4) Download an object: ```$ aws s3 --endpoint=http://localhost:7777/ cp s3://bucket-name/your-large-file.mp4 ~/Desktop/your-large-file.mp4```
5) Generate a URL for an object: ``` $ aws s3 --endpoint=http://localhost:7777/ presign s3://bucket-name/your-large-file.mp4```
6) Delete an object: ```$ aws s3 --endpoint=http://localhost:7777/ rm s3://bucket-name/your-large-file.mp4```

For more information about the AWS s3 CLI visit: https://docs.aws.amazon.com/cli/latest/reference/s3/index.html


# Start Contributing to Storj <a name="storjlocal"></a>

### Install required packages

First, install git and golang. We currently support Debian-based and Mac operating systems

#### Debian based (like Ubuntu)

Download and install the latest release of go https://golang.org/

```
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

### Install all dependencies

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

## Support <a name="support"></a>

If you have any questions or suggestions please reach out to us on [Rocketchat](https://community.storj.io/) or [Twitter](https://twitter.com/storjproject).
