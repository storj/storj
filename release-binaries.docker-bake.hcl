/*

This bake script builds all binaries for all platforms, and build the UI artifacts.
The result will be copied into `release/$VERSION`.

For each platform Dockerfile.release builds with the following steps:

  1. Downloading dependencies
  2. Building WASM artifacts
  3. Building UI artifacts
  4. Creating resource files for Windows
  5. Building binaries for all platforms

*/

variable "VERSION" {
  default = "v0.0.0+dev"
}

variable "BUILD_RELEASE" {
  default = "true"
}

variable "PLATFORMS" {
  default = {
    "linux/amd64" = {
      goos    = "linux"
      goarch  = "amd64"
      cgo     = "1"
      ldflags = "-linkmode external -extldflags \"-static\""
      cc      = "zig cc  -target x86_64-linux-musl"
      cpp     = "zig c++ -target x86_64-linux-musl"
      components = "./cmd/uplink ./cmd/identity ./cmd/multinode ./cmd/storagenode ./cmd/storagenode-updater ./cmd/certificates ./cmd/satellite ./cmd/versioncontrol ./cmd/tools/segment-verify ./cmd/jobq"
    }
    "linux/arm64" = {
      goos    = "linux"
      goarch  = "arm64"
      cgo     = "1"
      ldflags = "-linkmode external -extldflags \"-static\""
      cc      = "zig cc  -target aarch64-linux-musl"
      cpp     = "zig c++ -target aarch64-linux-musl"
      components = "./cmd/uplink ./cmd/identity ./cmd/multinode ./cmd/storagenode ./cmd/storagenode-updater ./cmd/satellite ./cmd/versioncontrol"
    }
    "linux/arm" = {
      goos    = "linux"
      goarch  = "arm"
      cgo     = "1"
      ldflags = "-linkmode external -extldflags \"-static\""
      cc      = "zig cc  -target arm-linux-musleabi"
      cpp     = "zig c++ -target arm-linux-musleabi"
      components = "./cmd/uplink ./cmd/identity ./cmd/multinode ./cmd/storagenode ./cmd/storagenode-updater"
    }
    "windows/amd64" = {
      goos    = "windows"
      goarch  = "amd64"
      cgo     = "1"
      ldflags = ""
      cc      = "zig cc  -target x86_64-windows-gnu"
      cpp     = "zig c++ -target x86_64-windows-gnu"
      components = "./cmd/uplink ./cmd/identity ./cmd/multinode ./cmd/storagenode ./cmd/storagenode-updater"
    }
    "freebsd/amd64" = {
      goos    = "freebsd"
      goarch  = "amd64"
      cgo     = "1"
      ldflags = ""
      cc      = "zig cc  -target x86_64-freebsd-none"
      cpp     = "zig c++ -target x86_64-freebsd-none"
      components = "./cmd/uplink ./cmd/identity ./cmd/multinode ./cmd/storagenode ./cmd/storagenode-updater"
    }
    "macos/amd64" = {
      goos    = "darwin"
      goarch  = "amd64"
      cgo     = "0"
      ldflags = ""
      cc      = "zig cc  -target x86_64-macos-none"
      cpp     = "zig c++ -target x86_64-macos-none"
      components = "./cmd/uplink ./cmd/identity"
    }
    "macos/arm64" = {
      goos    = "darwin"
      goarch  = "arm64"
      cgo     = "0"
      ldflags = ""
      cc      = "zig cc  -target aarch64-macos-none"
      cpp     = "zig c++ -target aarch64-macos-none"
      components = "./cmd/uplink ./cmd/identity"
    }
  }
}

target "default" {
  matrix = {
    item = keys(PLATFORMS)
  }

  name = "binaries-${replace(item, "/", "-")}" // e.g., binaries-linux-amd64
  platforms = [item]
  target = "export-binaries"

  contexts = {
    "web-storagenode" = "target:web-storagenode"
    "web-multinode"   = "target:web-multinode"

    "web-satellite-admin"         = "target:web-satellite-admin"
    "web-satellite-admin-legacy"  = "target:web-satellite-admin-legacy"
  }

  args = {
    "GOOS"        = PLATFORMS[item].goos
    "GOARCH"      = PLATFORMS[item].goarch
    "CGO_ENABLED" = PLATFORMS[item].cgo
    "CC"          = PLATFORMS[item].cc
    "CXX"         = PLATFORMS[item].cpp
    "GO_LDFLAGS"  = PLATFORMS[item].ldflags
    "COMPONENTS"  = PLATFORMS[item].components

    "VERSION"       = VERSION
    "BUILD_RELEASE" = BUILD_RELEASE
  }

  dockerfile = "release-binaries.Dockerfile"
  dockerignore = "release-binaries.Dockerfile.dockerignore"

  output = ["type=local,dest=./release/${VERSION}/${replace(item, "/", "_")}"]
}

/* UI Artifacts */

target "web-storagenode" {
  context    = "./web/storagenode"
  dockerfile = "Dockerfile"
  target = "export"
}

target "web-multinode" {
  context    = "./web/multinode"
  dockerfile = "Dockerfile"
  target = "export"
}

target "web-satellite-admin" {
  context    = "./satellite/admin/ui"
  dockerfile = "Dockerfile"
  target = "export"
}

target "web-satellite-admin-legacy" {
  context    = "./satellite/admin/legacy/ui"
  dockerfile = "Dockerfile"
  target = "export"
}

target "web-satellite" {
  context    = "."
  dockerfile = "release-binaries.Dockerfile"
  target = "web-satellite-export"
  output = []
}
