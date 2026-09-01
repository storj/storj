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

// BUILD_VERSION is the semantic version used for binary embedding and release paths.
// Example: "v1.120.8" for releases, "v0.2601.30974-dev+e90c239c1" for dev builds.
variable "BUILD_VERSION" {
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
  }

  args = {
    "GOOS"        = PLATFORMS[item].goos
    "GOARCH"      = PLATFORMS[item].goarch
    "CGO_ENABLED" = PLATFORMS[item].cgo
    "CC"          = PLATFORMS[item].cc
    "CXX"         = PLATFORMS[item].cpp
    "GO_LDFLAGS"  = PLATFORMS[item].ldflags
    "COMPONENTS"  = PLATFORMS[item].components

    "BUILD_VERSION" = BUILD_VERSION
    "BUILD_RELEASE" = BUILD_RELEASE
  }

  dockerfile = "release.Dockerfile"
  dockerignore = "release.Dockerfile.dockerignore"

  output = ["type=local,dest=./release/${BUILD_VERSION}/${replace(item, "/", "_")}"]
}

// windows-installer builds the Windows storagenode MSI from the windows/amd64 binaries.
target "windows-installer" {
  inherits = ["_signing"]

  contexts = {
    windows_amd64 = "target:binaries-windows-amd64"
  }
  target     = "export-windows-installer"
  dockerfile = "release.Dockerfile"
  dockerignore = "release.Dockerfile.dockerignore"

  args = {
    "BUILD_VERSION" = BUILD_VERSION
  }

  output = ["type=local,dest=./release/${BUILD_VERSION}/windows_amd64"]
}

// SIGN_ATTEMPT busts the cache of the signing steps, which is otherwise unaware
// of the credentials being available. It defaults to the current time, so that
// signing is retried however the target is invoked.
variable "SIGN_ATTEMPT" {
  default = timestamp()
}

// ALLOW_UNSIGNED builds unsigned Windows artifacts, under a name that says so,
// when there are no signing credentials. Without it, missing credentials fail
// the build, so that a release cannot silently end up without them.
variable "ALLOW_UNSIGNED" {
  default = ""
}

// _signing carries the credentials for signing Windows artifacts.
target "_signing" {
  args = {
    "SIGN_ATTEMPT"   = SIGN_ATTEMPT
    "ALLOW_UNSIGNED" = ALLOW_UNSIGNED
  }

  secret = [
    "type=env,id=azure_tenant_id,env=AZURE_TENANT_ID",
    "type=env,id=azure_client_id,env=AZURE_CLIENT_ID",
    "type=env,id=azure_client_secret,env=AZURE_CLIENT_SECRET",
    "type=env,id=sign_keystore,env=SIGN_KEYSTORE",
    "type=env,id=sign_alias,env=SIGN_ALIAS",
  ]
}

// finalized-binaries builds every binary, signs the Windows artifacts, verifies
// that the binaries are release builds and compresses them for publishing.
// Signing needs AZURE_TENANT_ID, AZURE_CLIENT_ID, AZURE_CLIENT_SECRET,
// SIGN_KEYSTORE and SIGN_ALIAS in the environment; without them the build fails,
// unless ALLOW_UNSIGNED is set.
target "finalized-binaries" {
  inherits = ["_signing"]

  contexts = {
    linux_amd64   = "target:binaries-linux-amd64"
    linux_arm64   = "target:binaries-linux-arm64"
    linux_arm     = "target:binaries-linux-arm"
    windows_amd64 = "target:binaries-windows-amd64"
    freebsd_amd64 = "target:binaries-freebsd-amd64"
    macos_amd64   = "target:binaries-macos-amd64"
    macos_arm64   = "target:binaries-macos-arm64"
  }

  target     = "export-finalized-binaries"
  dockerfile = "release.Dockerfile"
  dockerignore = "release.Dockerfile.dockerignore"

  args = {
    "BUILD_VERSION" = BUILD_VERSION
  }

  output = ["type=local,dest=./release/${BUILD_VERSION}"]
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

target "web-satellite" {
  context    = "./web/satellite"
  dockerfile = "Dockerfile"
  target = "ui"
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
