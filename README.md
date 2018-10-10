# Storj V3 Network

[![Go Report Card](https://goreportcard.com/badge/github.com/storj/storj)](https://goreportcard.com/report/github.com/storj/storj)
[![Go Doc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](http://godoc.org/github.com/storj/storj)
[![Coverage Status](https://coveralls.io/repos/github/storj/storj/badge.svg?branch=master)](https://coveralls.io/github/storj/storj?branch=master)

<img src="https://github.com/storj/storj/raw/master/logo/logo.png" width="100">

Storj is building a decentralized cloud storage network and is launching in 
early 2019.

----

Storj is an S3 compatible platform and suite of decentralized applications that 
allows you to store data in a secure and decentralized manner. Your files are
encrypted, broken into little pieces and stored in a global decentralized
network of computers. Luckily, we also support allowing you (and only you) to
retrieve those files!

## Table of Contents

- [Contributing](#start-contributing-to-storj)
- [Using via Storj CLI](#start-using-storj-via-the-storj-cli)
- [Using via AWS S3 CLI](#start-using-storj-via-the-aws-s3-cli)
- [License](#license)
- [Support](#support)

# Start Contributing to Storj

### Install required packages

Download and install the latest release of Go, at least Go 1.11:
https://golang.org/

You will also need Git. (`brew install git`, `apt-get install git`, etc).
If you're building on Windows, you also need to install and have gcc setup correctly (easiest way via mingw64).

Install git and golang. We support Linux, Mac, and Windows operating
systems. Other operating systems supported by Go are probably not much
additional work.

### Download and compile Storj

> **Aside about GOPATH**: Go 1.11 supports a new feature called Go modules,
> and Storj has adopted Go module support. If you've used previous Go versions,
> Go modules no longer require a GOPATH environment variable. Go by default
> falls back to the old behavior if you check out code inside of the directory
> referenced by your GOPATH variable, so make sure to use another directory,
> `unset GOPATH` entirely, or set `GO111MODULE=on` before continuing with these
> instructions. If you don't have a GOPATH set, you can ignore this aside.

```bash
git clone git@github.com:storj/storj storj
cd storj
go install -v ./cmd/...
```

### Configure a test network

```bash
~/go/bin/captplanet setup
```

### Start the test network

```bash
~/go/bin/captplanet run
```

### Run unit tests

```bash
go test -v ./...
```

You can execute only a single test package if you like. For example:
`go test ./pkg/kademlia`. Add `-v` for more informations about the executed unit
tests.

# Start Using Storj via the Storj CLI

#### Configure the Storj CLI
1) In a new terminal setup the Storj CLI: ```$ storj setup```
2) Edit the API Key, overlay address, and pointer db address fields in the Storj
CLI config file located at ```~/.storj/cli/config.yaml``` with values from the
captplanet config file located at ```~/.storj/capt/config.yaml```

#### Test out some Storj CLI commands!

1) Create a bucket: ```$ storj mb s3://bucket-name```
2) Upload an object: ```$ storj cp ~/Desktop/your-large-file.mp4 s3://bucket-name```
3) List objects in a bucket: ```$ storj ls s3://bucket-name/ ```
4) Download an object: ```$ storj cp s3://bucket-name/your-large-file.mp4 ~/Desktop/your-large-file.mp4```
6) Delete an object: ```$ storj rm s3://bucket-name/your-large-file.mp4```


# Start Using Storj via the AWS S3 CLI

#### Configure AWS CLI

Download and install the AWS S3 CLI: https://docs.aws.amazon.com/cli/latest/userguide/installing.html

In a new terminal session configure the AWS S3 CLI:
```bash
$ aws configure
AWS Access Key ID [None]: insecure-dev-access-key
AWS Secret Access Key [None]: insecure-dev-secret-key
Default region name [None]: us-east-1
Default output format [None]:
```

#### Test out some AWS S3 CLI commands!

1) Create a bucket: ```$ aws s3 --endpoint=http://localhost:7777/ mb s3://bucket-name```
2) Upload an object: ```$ aws s3 --endpoint=http://localhost:7777/ cp ~/Desktop/your-large-file.mp4 s3://bucket-name```
3) List objects in a bucket: ```$ aws s3 --endpoint=http://localhost:7777/ ls s3://bucket-name/ ```
4) Download an object: ```$ aws s3 --endpoint=http://localhost:7777/ cp s3://bucket-name/your-large-file.mp4 ~/Desktop/your-large-file.mp4```
5) Generate a URL for an object: ``` $ aws s3 --endpoint=http://localhost:7777/ presign s3://bucket-name/your-large-file.mp4```
6) Delete an object: ```$ aws s3 --endpoint=http://localhost:7777/ rm s3://bucket-name/your-large-file.mp4```

For more information about the AWS s3 CLI visit: https://docs.aws.amazon.com/cli/latest/reference/s3/index.html

# License

The network under construction (this repo) is currently licensed with the 
[AGPLv3](https://www.gnu.org/licenses/agpl-3.0.en.html) license. Once the network 
reaches beta phase, we will be licensing all client-side code via the 
[Apache v2](https://www.apache.org/licenses/LICENSE-2.0) license.

For code released under the AGPLv3, we request that contributors sign 
[our Contributor License Agreement (CLA)](https://docs.google.com/forms/d/e/1FAIpQLSdVzD5W8rx-J_jLaPuG31nbOzS8yhNIIu4yHvzonji6NeZ4ig/viewform) so that we can relicense the
code under Apache v2, or other licenses in the future.

# Support

If you have any questions or suggestions please reach out to us on
[Rocketchat](https://community.storj.io/) or
[Twitter](https://twitter.com/storjproject).
