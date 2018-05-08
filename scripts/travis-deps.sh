set -x

mkdir -p $HOME/gopath-staging
cd $HOME/gopath-staging
git clone --depth=1 --recursive --shallow-submodules https://github.com/storj/storj-vendor.git .
./setup.sh
mkdir -p src/storj.io
mv $HOME/gopath/src/github.com/storj/storj src/storj.io
rm -rf $HOME/gopath
mv $HOME/gopath{-staging,}
export TRAVIS_BUILD_DIR=$HOME/gopath/src/storj.io/storj
cd $TRAVIS_BUILD_DIR

set +x
