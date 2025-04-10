# you can push dev images with: ./scripts/bake.sh -f docker-bake.hcl -f docker-bake-dev.hcl storagenode-modular --push
target "storagenode-modular" {
  output = [
    {
      type = "image"
      name = "ghcr.io/storj/storagenode-modular:${BUILD_VERSION}-dev"
    }
  ]
}

target "storagenode-ui" {
  output = [
    {
      type = "image"
      name = "ghcr.io/storj/storagenode-ui:${BUILD_VERSION}-dev"
    }
  ]
}
