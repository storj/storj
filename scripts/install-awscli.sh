set -x

mkdir -p $HOME/awscli
pushd $HOME/awscli

curl "https://s3.amazonaws.com/aws-cli/awscli-bundle.zip" -o "awscli-bundle.zip"
unzip awscli-bundle.zip
./awscli-bundle/install -b ~/bin/aws
cd ~/bin
echo pwd

popd

set +x
