#!/usr/bin/env bash
set -eu
set -o pipefail

if [[ $# -eq 0 ]]; then
  echo "Usage: $0 [build|push] [options]"
  exit 1
fi

ACTION=$1
shift  # drop the first argument

BUILD_REPO="storjlabs/storagenode-modular"
PUSH_REPO=""
EXTRA_TAGS=()
SPECIFIED_VERSION=""
SELECTED_ARCHS=()

declare -A SUPPORTED_ARCHS=(
  ["amd64"]="linux/amd64 amd64 linux/amd64 amd64"
  ["arm64v8"]="linux/arm64/v8 arm64v8 linux/arm64/v8 arm64"
  ["arm32v5"]="linux/arm/v5 arm32v5 linux/arm/v7 arm"
)

# parse flags
while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo)
      if [[ "$ACTION" != "push" ]]; then
        echo "Error: --repo can only be used with push."
        exit 1
      fi
      PUSH_REPO="$2"
      shift 2
      ;;
    --tags)
      if [[ "$ACTION" != "push" ]]; then
        echo "Error: --tags can only be used with push."
        exit 1
      fi
      IFS=',' read -r -a EXTRA_TAGS <<< "$2"
      shift 2
      ;;
    --version)
      if [[ "$ACTION" != "push" ]]; then
        echo "Error: --version can only be used with push."
        exit 1
      fi
      SPECIFIED_VERSION="$2"
      shift 2
      ;;
    --arch)
      IFS=',' read -r -a SELECTED_ARCHS <<< "$2"
      shift 2
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

# check if the selected archs are supported
if [[ ${#SELECTED_ARCHS[@]} -gt 0 ]]; then
  for arch in "${SELECTED_ARCHS[@]}"; do
    if [[ -z "${SUPPORTED_ARCHS[$arch]+exists}" ]]; then
      echo "Error: Unsupported architecture '$arch'. Supported values: ${!SUPPORTED_ARCHS[*]}"
      exit 1
    fi
  done
else
  SELECTED_ARCHS=("${!SUPPORTED_ARCHS[@]}")  # Default: All architectures
fi

# ensure required flags are set
# TODO: make it optional, and push to dockerhub by default
if [[ "$ACTION" == "push" && -z "$PUSH_REPO" ]]; then
  echo "Error: --repo is required for push."
  exit 1
fi

echo -n "Build timestamp: "
TIMESTAMP=$(date +%s)
echo $TIMESTAMP

echo -n "Git commit: "
COMMIT=$(git rev-parse HEAD)
echo $COMMIT

# determine version (either from Git or user input)
if [[ -n "$SPECIFIED_VERSION" ]]; then
  VERSION="$SPECIFIED_VERSION"
  echo "Using specified version: $VERSION"
elif VERSION=$(git describe --tags --exact-match --match "v[0-9]*.[0-9]*.[0-9]*" 2>/dev/null); then
  echo "Using tagged version: $VERSION"
else
  VERSION=$(git show -s --date='format:%Y.%m' --format='v%cd.%ct-%h' HEAD)
  echo "Using commit-based version: $VERSION"
fi

# Build or push Docker images
for arch in "${SELECTED_ARCHS[@]}"; do
  IFS=" " read -r docker_platform docker_arch go_docker_platform _ <<< "${SUPPORTED_ARCHS[$arch]}"
  IMAGE_TAG="${VERSION}-${docker_arch}"
  BUILD_IMAGE="${BUILD_REPO}:${IMAGE_TAG}"

  if [[ "$ACTION" == "build" ]]; then
    echo "Building image for ${docker_platform}"

    docker buildx build --load --pull=true -t "${BUILD_IMAGE}" \
      --platform="${docker_platform}" \
      --build-arg=GO_DOCKER_PLATFORM="${go_docker_platform}" \
      --build-arg=DOCKER_PLATFORM="${docker_platform}" \
      --build-arg=DOCKER_ARCH="${docker_arch}" \
      --build-arg=BUILD_DATE="${TIMESTAMP}" \
      --build-arg=BUILD_COMMIT="${COMMIT}" \
      --build-arg=BUILD_VERSION="${VERSION}" \
      -f release/Dockerfile .

  elif [[ "$ACTION" == "push" ]]; then
    TARGET_IMAGE="${PUSH_REPO}:${IMAGE_TAG}"

    echo "Retagging ${BUILD_IMAGE} -> ${TARGET_IMAGE}"
    docker tag "${BUILD_IMAGE}" "${TARGET_IMAGE}"

    echo "Pushing ${TARGET_IMAGE}"
    docker push "${TARGET_IMAGE}"
  fi
done

if [[ "$ACTION" == "push" && ${#EXTRA_TAGS[@]} -gt 0 ]]; then
  echo "Creating and annotating manifest list..."
  for tag in "${EXTRA_TAGS[@]}"; do
    MANIFEST_TAG="${PUSH_REPO}:${tag}"
    echo "Creating manifest for ${MANIFEST_TAG}"
    docker manifest create --amend "${MANIFEST_TAG}" \
      $(for arch in "${SELECTED_ARCHS[@]}"; do
          IFS=" " read -r docker_platform docker_arch _ _<<< "${SUPPORTED_ARCHS[$arch]}"
          echo "${PUSH_REPO}:${VERSION}-${docker_arch}"
        done)

    for arch in "${SELECTED_ARCHS[@]}"; do
      IFS=" " read -r docker_platform docker_arch _ os_arch <<< "${SUPPORTED_ARCHS[$arch]}"
      echo "Annotating ${MANIFEST_TAG} for ${docker_platform}"
      docker manifest annotate "${MANIFEST_TAG}" "${PUSH_REPO}:${VERSION}-${docker_arch}" \
        --os linux --arch "${os_arch}"
    done

    echo "Pushing manifest ${MANIFEST_TAG}"
    docker manifest push "${MANIFEST_TAG}"
  done
fi
