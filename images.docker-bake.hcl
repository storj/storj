variable "TAG" {
  default = "dev"
}

variable "CUSTOMTAG" {
  default = ""
}

// virtual target for images.docker-bake.hcl so that a single
// binaries image can be used for multiple target platforms.
target "binaries-linux" {
  contexts = {
    linux_amd64 = "target:binaries-linux-amd64",
    linux_arm64 = "target:binaries-linux-arm64",
    linux_arm   = "target:binaries-linux-arm",
  }
  target = "combine-platforms"
  dockerfile = "release-binaries.Dockerfile"

  output = []
}

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

group "push-images" {
  targets = [
    "segment-verify-image-push",
    "jobq-image-push",
    "multinode-image-push",
    "uplink-image-push",
    "satellite-image-push",
    "versioncontrol-image-push",
    "storagenode-dev-image-push",
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
  tags       = ["storjlabs/jobq:${TAG}${CUSTOMTAG}"]
}

target "jobq-image-push" {
  inherits = ["jobq-image"]
  output   = ["type=registry"]
}

target "segment-verify-image" {
  inherits   = ["_base"]
  dockerfile = "cmd/tools/segment-verify/Dockerfile"
  platforms  = ["linux/amd64"]
  tags       = ["storjlabs/segment-verify:${TAG}${CUSTOMTAG}"]
}

target "segment-verify-image-push" {
  inherits = ["segment-verify-image"]
  output   = ["type=registry"]
}

target "uplink-image" {
  inherits   = ["_base"]
  dockerfile = "cmd/uplink/Dockerfile"
  tags       = ["storjlabs/uplink:${TAG}${CUSTOMTAG}"]
}

target "uplink-image-push" {
  inherits = ["uplink-image"]
  output   = ["type=registry"]
}

target "satellite-image" {
  inherits   = ["_base"]
  contexts   = {
    binaries = "target:binaries-linux"
    ui       = "target:web-satellite"
  }
  dockerfile = "cmd/satellite/Dockerfile"
  platforms  = ["linux/amd64", "linux/arm64"]
  tags       = ["storjlabs/satellite:${TAG}${CUSTOMTAG}"]
}

target "satellite-image-push" {
  inherits = ["satellite-image"]
  output   = ["type=registry"]
}

target "versioncontrol-image" {
  inherits   = ["_base"]
  dockerfile = "cmd/versioncontrol/Dockerfile"
  platforms  = ["linux/amd64", "linux/arm64"]
  tags       = ["storjlabs/versioncontrol:${TAG}${CUSTOMTAG}"]
}

target "versioncontrol-image-push" {
  inherits = ["versioncontrol-image"]
  output   = ["type=registry"]
}

target "multinode-image" {
  inherits   = ["_base"]
  dockerfile = "cmd/multinode/Dockerfile"
  tags       = ["storjlabs/multinode:${TAG}${CUSTOMTAG}"]
}

target "multinode-image-push" {
  inherits = ["multinode-image"]
  output   = ["type=registry"]
}

// THIS IS NOT THE PRODUCTION STORAGENODE!!! Only for testing.
target "storagenode-dev-image" {
  inherits   = ["_base"]
  contexts   = {
    binaries = "target:binaries-linux"
    ui       = "target:web-storagenode"
  }
  dockerfile = "cmd/storagenode/Dockerfile.dev"
  platforms  = ["linux/amd64", "linux/arm64"]
  tags       = ["img.dev.storj.io/dev/storagenode:${TAG}${CUSTOMTAG}"]
}

target "storagenode-dev-image-push" {
  inherits = ["storagenode-dev-image"]
  output   = ["type=registry"]
}
