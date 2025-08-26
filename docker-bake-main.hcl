# This file enhance the normal docker-bake.hcl (docker can combine the two) with publishing images, and publishing build cached.
# Should be used only for main builds.

target "storagenode-modular" {
  cache-to = [
    {
      type = "registry",
      mode = "min",
      ref  = "ghcr.io/storj/storagenode-modular-cache:main"
    }
  ]
  output = [
    {
      type = "image"
      name = "ghcr.io/storj/storagenode-modular:${BUILD_VERSION}"
    }
  ]
}


target "satellite-modular" {
  cache-to = [
    {
      type = "registry",
      mode = "min",
      ref  = "ghcr.io/storj/satellite-modular-cache:main"
    }
  ]
  output = [
    {
      type = "image"
      name = "ghcr.io/storj/satellite-modular:${BUILD_VERSION}"
    }
  ]
}

target "storagenode-ui" {
  cache-to = [
    {
      type = "registry",
      mode = "min",
      ref  = "ghcr.io/storj/storagenode-ui-cache:main"
    }
  ]
  output = [
    {
      type = "image"
      name = "ghcr.io/storj/storagenode-ui:${BUILD_VERSION}"
    }
  ]
}
