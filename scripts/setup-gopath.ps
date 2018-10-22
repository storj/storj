$home = $env:USERPROFILE

mkdir -p $home/bin $home/cache
$env:PATH=$env:PATH + ";$home/bin"

$env:GOSPACE_ROOT="$env:GOPATH"
$env:GOSPACE_PKG="storj.io/storj"
$env:GOSPACE_REPO="git@github.com:storj/storj/git"

New-Item "$env:GOPATH/src/storj.io" -ItemType "directory"
Move-Item "$env:GOPATH/src/github.com/storj/storj" "$env:GOPATH/src/storj.io/storj"

# setup gospace
Invoke-WebRequest -Uri "https://github.com/storj/gospace/releases/download/v0.0.1/gospace_windows_amd64.exe" -OutFile "$home/bin"

# find module dependency hash
$modhash = gospace hash

# download dependencies, if we don't have them in cache
if (!(Test-Path $home/cache/$modhash.zip)) {
    gospace zip-vendor $home/cache/$modhash.zip
}

# unpack the dependencies into gopath
gospace unzip-vendor $home/cache/$modhash.zip
gospace flatten-vendor

$env:TRAVIS_BUILD_DIR="$env:GOPATH/src/storj.io/storj"
Set-Location $env:TRAVIS_BUILD_DIR
