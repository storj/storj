/*

This bake script defines targets for:

1. cross compiling binaries
2. building UI artifacts
3. building release images

For each platform release.Dockerfile builds with the following steps:

  1. Downloading dependencies
  2. Building WASM artifacts
  3. Building UI artifacts
  4. Creating resource files for Windows
  5. Building binaries for all platforms

*/

// VERSION is the semantic version used for binary embedding and release paths.
// Example: "v1.120.8" for releases, "v0.2601.30974-dev+e90c239c1" for dev builds.
variable "VERSION" {
  default = "v0.0.0+dev"
}

// BUILD_RELEASE controls whether binaries are marked as release builds.
// When "true", enables release-specific behavior in the built binaries.
variable "BUILD_RELEASE" {
  default = "true"
}

// TAG is the Docker image tag suffix used for published images.
// Example: "v1.120.8" for releases, "dev" for development builds.
variable "TAG" {
  default = "dev"
}

// CUSTOMTAG is an optional additional tag suffix appended to image tags.
// Useful for distinguishing variant builds (e.g., "-debug", "-test").
variable "CUSTOMTAG" {
  default = ""
}

// LATEST_TAG controls whether to also tag images as "latest".
// Set to "-latest" to add the latest tag, or "" to skip it.
variable "LATEST_TAG" {
  default = ""
}

// PLATFORMS is a map of supported platforms and what components are built for each platform.
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

// binaries target does a cross-compilation of all binaries.
target "binaries" {
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

  dockerfile = "release.Dockerfile"
  dockerignore = "release.Dockerfile.dockerignore"

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
  dockerfile = "release.Dockerfile"
  target = "web-satellite-export"
  output = []
}

/* Images building */

group "images" {
  targets = [
    "segment-verify-image",
    "jobq-image",
    "multinode-image",
    "uplink-image",
    "satellite-image",
    "versioncontrol-image",
    "storagenode-dev-image",
  ]
}

/* Development binaries for images. */

target "storj-up" {
  context    = "."
  dockerfile = "release.Dockerfile"
  target     = "storj-up-binaries"
  output     = []
}

target "delve" {
  context    = "."
  dockerfile = "release.Dockerfile"
  target     = "delve-binaries"
  output     = []
}

// virtual target for images so that a single binaries image
// can be used for multiple target platforms.
target "binaries-linux" {
  contexts = {
    linux_amd64 = "target:binaries-linux-amd64",
    linux_arm64 = "target:binaries-linux-arm64",
    linux_arm   = "target:binaries-linux-arm",
  }
  target     = "combine-platforms"
  dockerfile = "release.Dockerfile"

  output = []
}

// image_tags is a function that returns a list of image tags for a given image name.
// It automatically adds "latest" tag when LATEST_TAG is not empty.
function "image_tags" {
  params = [name]
  result = LATEST_TAG != "" ? [
    "storjlabs/${name}:${TAG}${CUSTOMTAG}",
    "storjlabs/${name}:${LATEST_TAG}"
    ] : [
    "storjlabs/${name}:${TAG}${CUSTOMTAG}"
  ]
}

target "_base" {
  context   = "."
  platforms = ["linux/amd64", "linux/arm64", "linux/arm"]
  contexts  = { binaries = "target:binaries-linux" }
  pull      = true
}

target "jobq-image" {
  inherits   = ["_base"]
  dockerfile = "cmd/jobq/Dockerfile"
  platforms  = ["linux/amd64"]
  tags       = image_tags("jobq")
}

target "segment-verify-image" {
  inherits   = ["_base"]
  dockerfile = "cmd/tools/segment-verify/Dockerfile"
  platforms  = ["linux/amd64"]
  tags       = image_tags("segment-verify")
}

target "uplink-image" {
  inherits   = ["_base"]
  dockerfile = "cmd/uplink/Dockerfile"
  tags       = image_tags("uplink")
}


target "satellite-image" {
  inherits = ["_base"]
  contexts = {
    binaries = "target:binaries-linux"
    ui       = "target:web-satellite"
    storj-up = "target:storj-up"
    delve    = "target:delve"
  }
  dockerfile = "cmd/satellite/Dockerfile"
  platforms  = ["linux/amd64", "linux/arm64"]
  tags       = image_tags("satellite")
}

target "versioncontrol-image" {
  inherits   = ["_base"]
  dockerfile = "cmd/versioncontrol/Dockerfile"
  platforms  = ["linux/amd64", "linux/arm64"]
  tags       = image_tags("versioncontrol")
}

target "multinode-image" {
  inherits   = ["_base"]
  dockerfile = "cmd/multinode/Dockerfile"
  tags       = image_tags("multinode")
}

// THIS IS NOT THE PRODUCTION STORAGENODE!!! Only for testing.
target "storagenode-dev-image" {
  inherits = ["_base"]
  contexts = {
    binaries = "target:binaries-linux"
    ui       = "target:web-storagenode"
    storj-up = "target:storj-up"
    delve    = "target:delve"
  }
  dockerfile = "cmd/storagenode/Dockerfile.dev"
  platforms  = ["linux/amd64", "linux/arm64"]
  tags       = ["img.dev.storj.io/dev/storagenode:${TAG}${CUSTOMTAG}"]
}
