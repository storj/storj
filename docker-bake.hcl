# This is a standard docker-bake.hcl file.
# You can call any target with `docker buildx bake <target>`.
# But because we need versions, it's better to use `./scripts/bake.sh <target>`.

variable "BUILD_COMMIT" {
  default = "head"
}
variable "BUILD_DATE" {
  default = "1970-01-01T00:00:00.0Z"
}
variable "BUILD_VERSION" {
  default = "dev"
}

target "storagenode-modular" {
  args = {
    BUILD_COMMIT  = "${BUILD_COMMIT}"
    BUILD_VERSION = "${BUILD_VERSION}"
    BUILD_DATE    = "${BUILD_DATE}"
  }

  dockerfile = "./storagenode/storagenode/Dockerfile"
  context    = "."
  contexts = {
    webui = "target:storagenode-ui"
  }

  target = "build"
  cache-from = [
    {
      type = "registry",
      ref  = "ghcr.io/storj/storagenode-modular-cache:main"
    }
  ]

  tags = [
   "ghcr.io/storj/storagenode-modular:${BUILD_VERSION}"
  ]
}

target "storagenode-ui" {
  args = {
    BUILD_COMMIT  = "${BUILD_COMMIT}"
    BUILD_VERSION = "${BUILD_VERSION}"
    BUILD_DATE    = "${BUILD_DATE}"
  }

  dockerfile = "./web/storagenode/Dockerfile"
  context    = "."
  target = "ui"
  cache-from = [
    {
      type = "registry",
      ref  = "ghcr.io/storj/storagenode-ui-cache:main"
    }
  ]

  tags = [
    "ghcr.io/storj/storagenode-ui:${BUILD_VERSION}"
  ]
}

target "satellite-modular" {
  args = {
    BUILD_COMMIT  = "${BUILD_COMMIT}"
    BUILD_VERSION = "${BUILD_VERSION}"
    BUILD_DATE    = "${BUILD_DATE}"
  }

  dockerfile = "./satellite/satellite/Dockerfile"
  context    = "."
  contexts = {
    webui = "target:satellite-ui"
  }
  target = "build"
  cache-from = [
    {
      type = "registry",
      ref  = "ghcr.io/storj/satellite-modular-cache:main"
    }
  ]

  tags = [
   "ghcr.io/storj/satellite-modular:${BUILD_VERSION}"
  ]
}

target "satellite-ui" {
  args = {
    BUILD_COMMIT  = "${BUILD_COMMIT}"
    BUILD_VERSION = "${BUILD_VERSION}"
    BUILD_DATE    = "${BUILD_DATE}"
  }

  dockerfile = "./web/satellite/Dockerfile"
  context    = "."
  target = "ui"
  cache-from = [
    {
      type = "registry",
      ref  = "ghcr.io/storj/satellite-ui-cache:main"
    }
  ]

  tags = [
    "ghcr.io/storj/satellite-ui:${BUILD_VERSION}"
  ]
}

target "storagenode-modular-all-platform" {
  inherits = ["storagenode-modular"]
  platforms = ["linux/arm64", "linux/amd64"]
}