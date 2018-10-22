set -x

set

set HOME=%USERPROFILE%

mkdir -p %HOME%/bin %HOME%/cache
set PATH=%HOME%/bin;$PATH

set GOSPACE_ROOT=%GOPATH%
set GOSPACE_PKG=storj.io/storj
set GOSPACE_REPO=git@github.com:storj/storj/git

mkdir -p %GOPATH%/src/storj.io
mv %GOPATH%/src/github.com/storj/storj %GOPATH%/src/storj.io

rem setup gospace
wget -O %HOME%/bin/gospace https://github.com/storj/gospace/releases/download/v0.0.1/gospace_windows_amd64
chmod +x %HOME%/bin/gospace

rem find module dependency hash
FOR /F "tokens=*" %g IN ('*your command*') do (SET MODHASH=%g)

rem download dependencies, if we don't have them in cache
if not exist %HOME%/cache/$MODHASH.zip (
    gospace zip-vendor %HOME%/cache/$MODHASH.zip
)

rem unpack the dependencies into gopath
gospace unzip-vendor %HOME%/cache/$MODHASH.zip
gospace flatten-vendor

set TRAVIS_BUILD_DIR=%GOPATH%/src/storj.io/storj
cd %TRAVIS_BUILD_DIR%

set +x
