variable "TAG" {
  default = "dev"
}

variable "CUSTOMTAG" {
  default = ""
}

variable "LATEST_TAG" {
  default = ""
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

// virtual target for images.docker-bake.hcl so that a single
// binaries image can be used for multiple target platforms.
target "binaries-linux" {
  contexts = {
    linux_amd64 = "target:binaries-linux-amd64",
    linux_arm64 = "target:binaries-linux-arm64",
    linux_arm   = "target:binaries-linux-arm",
  }
  target     = "combine-platforms"
  dockerfile = "release-binaries.Dockerfile"

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
  }
  dockerfile = "cmd/storagenode/Dockerfile.dev"
  platforms  = ["linux/amd64", "linux/arm64"]
  tags       = ["img.dev.storj.io/dev/storagenode:${TAG}${CUSTOMTAG}"]
}
