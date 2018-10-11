set -x

mkdir -p $HOME/gopath-staging
cd $HOME/gopath-staging
git clone --recursive https://github.com/storj/storj-vendor.git .
./setup.sh
mkdir -p src/storj.io
mv $HOME/gopath/src/github.com/storj/storj src/storj.io
rm -rf $HOME/gopath
mv $HOME/gopath{-staging,}
export TRAVIS_BUILD_DIR=$HOME/gopath/src/storj.io/storj
cd $TRAVIS_BUILD_DIR

mkdir -p $HOME/awscli
pushd $HOME/awscli

curl "https://s3.amazonaws.com/aws-cli/awscli-bundle.zip" -o "awscli-bundle.zip"
unzip awscli-bundle.zip
./awscli-bundle/install -b ~/bin/aws
export PATH=~/bin:$PATH

popd

set +x
