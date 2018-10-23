$xhome = $env:USERPROFILE

Set-Location $xhome

New-Item "$xhome/bin" -Force -ItemType "directory"
New-Item "$xhome/cache" -Force -ItemType "directory"

$env:PATH=$env:PATH + ";$xhome/bin"

$env:GOSPACE_ROOT="$env:GOPATH"
$env:GOSPACE_PKG="storj.io/storj"
$env:GOSPACE_REPO="git@github.com:storj/storj.git"

$LockingProcess = CMD /C "openfiles /query /fo table | find /I ""$env:GOPATH/src/github.com/storj/storj"""
Write-Host $LockingProcess

New-Item "$env:GOPATH/src/storj.io" -Force -ItemType "directory"
Copy-Item -Recurse -Path "$env:GOPATH/src/github.com/storj/storj" -Destination "$env:GOPATH/src/storj.io/storj" 

# setup gospace
[Net.ServicePointManager]::SecurityProtocol = "tls12, tls11, tls"
Invoke-WebRequest -Uri "https://github.com/storj/gospace/releases/download/v0.0.1/gospace_windows_amd64.exe" -OutFile "$xhome/bin/gospace.exe"

# find module dependency hash
$modhash = gospace hash

# download dependencies, if we don't have them in cache
if (!(Test-Path $xhome/cache/$modhash.zip)) {
    gospace zip-vendor $xhome/cache/$modhash.zip
}

# unpack the dependencies into gopath
gospace unzip-vendor $xhome/cache/$modhash.zip
gospace flatten-vendor

$env:TRAVIS_BUILD_DIR="$env:GOPATH/src/storj.io/storj"
Set-Location $env:TRAVIS_BUILD_DIR
