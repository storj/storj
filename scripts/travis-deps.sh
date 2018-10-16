set -x

export GOPATH=$HOME/storj

export GOSPACE_ROOT=$GOPATH
export GOSPACE_PKG=storj.io/storj
export GOSPACE_REPO=git@github.com:storj/storj/git

mkdir -p $GOPATH/src/storj.io
mv $HOME/gopath/src/github.com/storj/storj $GOPATH/src/storj.io
rm -rf $HOME/gopath

./$GOPATH/src/storj.io/storj/scripts/gospace unzip-vendor ./$GOPATH/src/storj.io/storj/scripts/storj-vendor.zip
./$GOPATH/src/storj.io/storj/scripts/gospace flatten-vendor
./$GOPATH/src/storj.io/storj/scripts/gospace update

export TRAVIS_BUILD_DIR=$GOPATH/src/storj.io/storj
cd $TRAVIS_BUILD_DIR

mkdir -p $HOME/awscli
pushd $HOME/awscli

curl "https://s3.amazonaws.com/aws-cli/awscli-bundle.zip" -o "awscli-bundle.zip"
unzip awscli-bundle.zip
./awscli-bundle/install -b ~/bin/aws
export PATH=~/bin:$PATH

popd

set +x
