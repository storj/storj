# Storj V3 Network

[![Go Report Card](https://goreportcard.com/badge/github.com/storj/storj)](https://goreportcard.com/report/github.com/storj/storj)
[![Go Doc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](http://godoc.org/github.com/storj/storj)
[![Coverage Status](https://coveralls.io/repos/github/storj/storj/badge.svg?branch=master)](https://coveralls.io/github/storj/storj?branch=master)

<img src="https://github.com/storj/storj/raw/master/resources/logo.png" width="100">

Storj is building a decentralized cloud storage network and is launching in
early 2019.

----

Storj is an S3 compatible platform and suite of decentralized applications that
allows you to store data in a secure and decentralized manner. Your files are
encrypted, broken into little pieces and stored in a global decentralized
network of computers. Luckily, we also support allowing you (and only you) to
retrieve those files!

## Table of Contents

- [Contributing](#contributing-to-storj)
- [Installation](#installation)
- [Using via Storj CLI](#start-using-storj-via-the-uplink-cli)
- [Using via AWS S3 CLI](#start-using-storj-via-the-aws-s3-cli)
- [License](#license)
- [Support](#support)

# Contributing to Storj

At the moment, all of our code here for v3 is open source. Have a code change you think would make Storj better? We are definitely open to pull requests. Send them along!

Have comments, bug reports, or suggestions? Want to propose a PR before hand-crafting it? Jump on to [our Rocketchat](https://community.storj.io) to join the community and to talk to the Storj core team.

# Installation

### Install required packages

To get started running Storj locally, download and install the latest release of Go (at least Go 1.11) at [golang.org](https://golang.org).

You will also need [Git](https://git-scm.com/). (`brew install git`, `apt-get install git`, etc).
If you're building on Windows, you also need to install and have [gcc](https://gcc.gnu.org/install/binaries.html) setup correctly.

We support Linux, Mac, and Windows operating systems. Other operating systems supported by Go should also be able to run Storj.

### Download and compile Storj

> **Aside about GOPATH**:  If you don't have a GOPATH set, you can ignore this > aside. Go 1.11 supports a new feature called Go modules,
> and Storj has adopted Go module support. If you've used previous Go versions,
> Go modules no longer require a GOPATH environment variable. Go by default
> falls back to the old behavior if you check out code inside of the directory
> referenced by your GOPATH variable, so make sure to use another directory,
> `unset GOPATH` entirely, or set `GO111MODULE=on` before continuing with these
> instructions.

First, clone this repository.

```bash
git clone git@github.com:storj/storj storj
cd storj
```

Then, let's install Storj.

```bash
go install -v ./cmd/...
```

Done!

### Working with the test network

Our test network daemon is called CaptPlanet. First, configure and run it:

```bash
~/go/bin/captplanet setup
~/go/bin/captplanet run
```

Then, you can run all of the unit tests:

```bash
go test -v ./...
```

You can also execute only a single test package if you like. For example:
`go test ./pkg/kademlia`. Add `-v` for more informations about the executed unit
tests.

More options can be shown by running `~/go/bin/captplanet --help`.

# Start Using Storj via the Uplink CLI

### Configure the Uplink CLI

1) In a new terminal window, setup the uplink CLI: ```$ uplink setup```. Keep `captplanet` running, as it ensures you have a test network to bounce data off of.
2) Edit the API Key, overlay address, and pointer db address fields in the Storj
CLI config file located at ```~/.uplink/cli/config.yaml``` with values from the
`captplanet` config file located at ```~/.uplink/capt/config.yaml```

### Test out some Uplink CLI commands!

1) Create a bucket: ```$ uplink mb s3://bucket-name```
2) Upload an object: ```$ uplink cp ~/Desktop/your-large-file.mp4 s3://bucket-name```
3) List objects in a bucket: ```$ uplink ls s3://bucket-name/ ```
4) Download an object: ```$ uplink cp s3://bucket-name/your-large-file.mp4 ~/Desktop/your-large-file.mp4```
6) Delete an object: ```$ uplink rm s3://bucket-name/your-large-file.mp4```

# Start Using Storj via the AWS S3 CLI

### Configure AWS CLI

Download and install the AWS S3 CLI: https://docs.aws.amazon.com/cli/latest/userguide/installing.html

In a new terminal session configure the AWS S3 CLI:
```bash
$ aws configure
AWS Access Key ID [None]: insecure-dev-access-key
AWS Secret Access Key [None]: insecure-dev-secret-key
Default region name [None]: us-east-1
Default output format [None]:
```

### Test out some AWS S3 CLI commands!

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
