set -x

mkdir -p ~/bin ~/cache
export PATH=~/bin:$PATH

export GOPATH=$HOME/storj

export GOSPACE_ROOT=$GOPATH
export GOSPACE_PKG=storj.io/storj
export GOSPACE_REPO=git@github.com:storj/storj/git

mkdir -p $GOPATH/src/storj.io
mv $HOME/gopath/src/github.com/storj/storj $GOPATH/src/storj.io
rm -rf $HOME/gopath

# TODO: setup gospace
mv $GOPATH/src/storj.io/storj/scripts/gospace ~/bin
# TODO: setup cache
mv $GOPATH/src/storj.io/storj/scripts/storj-vendor.zip ~/cache/storj-vendor.zip

gospace unzip-vendor ~/cache/storj-vendor.zip
gospace flatten-vendor
gospace update

export TRAVIS_BUILD_DIR=$GOPATH/src/storj.io/storj
cd $TRAVIS_BUILD_DIR

mkdir -p $HOME/awscli
pushd $HOME/awscli

curl "https://s3.amazonaws.com/aws-cli/awscli-bundle.zip" -o "awscli-bundle.zip"
unzip awscli-bundle.zip
./awscli-bundle/install -b ~/bin/aws

popd

set +x
