# Storj V3 Network

[![Go Report Card](https://goreportcard.com/badge/github.com/storj/storj)](https://goreportcard.com/report/github.com/storj/storj)
[![Go Doc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](http://godoc.org/github.com/storj/storj)
[![Coverage Status](https://coveralls.io/repos/github/storj/storj/badge.svg?branch=master)](https://coveralls.io/github/storj/storj?branch=master)

<img src="https://github.com/storj/storj/raw/master/resources/logo.png" width="100">

Storj is building a decentralized cloud storage network and is launching in
early 2019.

----

Storj is an S3-compatible platform and suite of decentralized applications that
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

All of our code for Storj v3 is open source. Have a code change you think would make Storj better? Please send a pull request along! Make sure to sign our [Contributor License Agreement (CLA)](https://docs.google.com/forms/d/e/1FAIpQLSdVzD5W8rx-J_jLaPuG31nbOzS8yhNIIu4yHvzonji6NeZ4ig/viewform) first. See our [license section](#license) for more details.

Have comments, bug reports, or suggestions? Want to propose a PR before hand-crafting it? Jump on to our [Rocketchat](https://community.storj.io) and join the [#dev channel](https://community.storj.io/channel/dev) to say hi to the developer community and to talk to the Storj core team.

# Installation

### Install required packages

To get started running Storj locally, download and install the latest release of Go (at least Go 1.11) at [golang.org](https://golang.org).

You will also need [Git](https://git-scm.com/). (`brew install git`, `apt-get install git`, etc).
If you're building on Windows, you also need to install and have [gcc](https://gcc.gnu.org/install/binaries.html) setup correctly.

We support Linux, Mac, and Windows operating systems. Other operating systems supported by Go should also be able to run Storj.

### Download and compile Storj

> **Aside about GOPATH**:  If you don't have a GOPATH set, you can ignore this 
> aside. Go 1.11 supports a new feature called Go modules,
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

You can run all of the unit tests:

```bash
go test -v ./...
```

You can also execute only a single test package if you like. For example:
`go test ./pkg/kademlia`. Add `-v` for more informations about the executed unit
tests.

# Start using Storj

We have another repository dedicated to [documentation and tutorials](https://github.com/storj/docs).
Check out these three tutorials:

 * [Using the Storj Test Network](https://github.com/storj/docs/blob/master/test-network.md)
 * [Using the Uplink CLI](https://github.com/storj/docs/blob/master/uplink-cli.md)
 * [Using the S3 Gateway](https://github.com/storj/docs/blob/master/s3-gateway.md)

# License

The network under construction (this repo) is currently licensed with the
[AGPLv3](https://www.gnu.org/licenses/agpl-3.0.en.html) license. Once the network
reaches beta phase, we will be licensing all client-side code via the
[Apache v2](https://www.apache.org/licenses/LICENSE-2.0) license.

For code released under the AGPLv3, we request that contributors sign our
[Contributor License Agreement (CLA)](https://docs.google.com/forms/d/e/1FAIpQLSdVzD5W8rx-J_jLaPuG31nbOzS8yhNIIu4yHvzonji6NeZ4ig/viewform) so that we can relicense the
code under Apache v2, or other licenses in the future.

# Support

If you have any questions or suggestions please reach out to us on
[Rocketchat](https://community.storj.io/) or
[Twitter](https://twitter.com/storjproject).
