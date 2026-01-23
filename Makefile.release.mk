##@ Release

# Git branch name with slashes replaced by dashes (e.g., "feature/foo" -> "feature-foo").
export GIT_BRANCH_NAME ?= $(shell git rev-parse --abbrev-ref HEAD | sed "s!/!-!g")
# Short commit hash (e.g., "a1b2c3d").
export GIT_COMMIT_HASH ?= $(shell git rev-parse --short HEAD)
# Git tag if the current commit is an exact version tag (e.g., "v1.9.0"), empty otherwise.
export GIT_TAG ?= $(shell git describe --tags --exact-match --match "v[0-9]*.[0-9]*.[0-9]*" 2>/dev/null)
# "-dirty" suffix if working directory has uncommitted changes, empty otherwise.
export GIT_DIRTY ?= $(shell test -n "`git status --porcelain`" && echo "-dirty")

# Commit identifier with dirty suffix (e.g., "a1b2c3d" or "a1b2c3d-dirty").
export BUILD_COMMIT_HASH := $(GIT_COMMIT_HASH)$(GIT_DIRTY)
# Unix timestamp of the last commit.
export BUILD_TIMESTAMP ?= $(shell git log -1 --format=%ct)

# Calculate year, month and minutes in the month.
BUILD_YEAR := $(shell date -d @$(BUILD_TIMESTAMP) +%Y 2>/dev/null || date -r $(BUILD_TIMESTAMP) +%Y)
BUILD_MONTH := $(shell date -d @$(BUILD_TIMESTAMP) +%m 2>/dev/null || date -r $(BUILD_TIMESTAMP) +%m)
# Calculate minutes since the start of that month.
# This uses minutes instead of seconds, because Windows patch version is limited to 65k.
BUILD_MONTH_START := $(shell date -d "$(BUILD_YEAR)-$(BUILD_MONTH)-01" +%s 2>/dev/null || date -j -f "%Y-%m-%d" "$(BUILD_YEAR)-$(BUILD_MONTH)-01" +%s)
BUILD_MINUTES_IN_MONTH := $(shell echo $$((($(BUILD_TIMESTAMP) - $(BUILD_MONTH_START)) / 60)))
# Use compact year format (last 2 digits) + month for minor version to fit Windows 65k limit
BUILD_YEAR_SHORT := $(shell echo $(BUILD_YEAR) | cut -c 3-)
BUILD_MINOR_VERSION := $(BUILD_YEAR_SHORT)$(BUILD_MONTH)

# Build version:
#   - If GIT_TAG is set (exact version tag), uses that (e.g., "v1.9.0").
#   - Otherwise, generates a dev version like "v0.2512.1231-dev+a1b2c3d".
export BUILD_VERSION ?= $(or $(GIT_TAG),v0.$(BUILD_MINOR_VERSION).$(BUILD_MINUTES_IN_MONTH)-dev+$(GIT_COMMIT_HASH))

# Docker image tag configuration:
#   exact version tag: TAG="v1.9.0"                 LATEST_TAG=""
#   main branch:       TAG="a1b2c3d"                LATEST_TAG="latest"
#   release-* branch:  TAG="a1b2c3d-release-1.2"    LATEST_TAG="release-1.2-latest"
#   other branches:    TAG="a1b2c3d-feature-foo"    LATEST_TAG=""
ifneq (${GIT_TAG},)
  export TAG := ${GIT_TAG}
  export LATEST_TAG :=
else ifeq (${GIT_BRANCH_NAME},main)
  export TAG := ${GIT_COMMIT_HASH}
  export LATEST_TAG := latest
else
  export TAG := ${GIT_COMMIT_HASH}-${GIT_BRANCH_NAME}
  ifneq (,$(findstring release-,$(GIT_BRANCH_NAME)))
    export LATEST_TAG := ${GIT_BRANCH_NAME}-latest
  endif
endif

# Optional suffix appended to Docker image tags (e.g., "-debug", "-test").
export CUSTOMTAG ?=

.PHONY: release/info
release/info: ## Script for showing the version.
	@echo "GIT_BRANCH_NAME:      $(GIT_BRANCH_NAME)"
	@echo "GIT_COMMIT_HASH:      $(GIT_COMMIT_HASH)"
	@echo "GIT_DIRTY:            $(GIT_DIRTY)"
	@echo "GIT_TAG:              $(GIT_TAG)"
	@echo "BUILD_COMMIT_HASH:    $(BUILD_COMMIT_HASH)"
	@echo "BUILD_TIMESTAMP:      $(BUILD_TIMESTAMP)"
	@echo "BUILD_VERSION:        $(BUILD_VERSION)"
	@echo "TAG:                  $(TAG)"
	@echo "LATEST_TAG:           $(LATEST_TAG)"
	@echo "CUSTOMTAG:            $(CUSTOMTAG)"

.PHONY: release/binaries/build
release/binaries/build: ## Cross-compile everything into release folder.
	@echo "Building release binaries"
	docker bake -f release.docker-bake.hcl binaries

.PHONY: release/binaries/check-release
release/binaries/check-release: ## Check that the built binaries are releases.
	@echo "Checking release binaries"
	./scripts/release/check-release-binaries.sh "release/$(BUILD_VERSION)"

.PHONY: release/binaries/sign
release/binaries/sign: ## Sign the binaries for platforms that need it.
	@echo "Signing release binaries"
	./scripts/release/windows-sign-folder.sh "release/$(BUILD_VERSION)/windows_amd64"

.PHONY: release/binaries/build-installers
release/binaries/build-installers: ## Build installers for platforms that need it.
	@echo "Building installers"
	# TODO: this needs to be invoked directly from a Windows machine at the moment.
	./installer/windows/buildrelease.bat

.PHONY: release/binaries/sign-installers
release/binaries/sign-installers: ## Sign installers for platforms that need it.
	@echo "Signing installers"
	storj-sign "release/$(BUILD_VERSION)/windows_amd64/storagenode.msi"

.PHONY: release/binaries/compress
release/binaries/compress: ## Compress all components into a single archive for a given platform.
	@echo "Compressing artifacts"
	# TODO: ideally this would be already done inside build-binaries part to avoid image bloat.
	# however, Windows needs the binaries to be uncompressed for signing so it complicates things a bit.
	./scripts/release/compress-binaries.sh "release/$(BUILD_VERSION)"

.PHONY: release/binaries/upload-to-google-storage
release/binaries/upload-to-google-storage: ## Upload binaries to Google Storage (jenkins)
	@echo "Uploading binaries to Google Storage"
	cd "release/$(BUILD_VERSION)" \
		&& gsutil -m cp -r ./*.zip sha256sums "gs://storj-v3-alpha-builds/$(BUILD_VERSION)/"

.PHONY: release/binaries/publish-to-github
release/binaries/publish-to-github: ## Publish the release to github.
	@echo "Publishing release to Github"
	scripts/release/publish-to-github.sh "$(GIT_BRANCH_NAME)" "release/$(BUILD_VERSION)"

.PHONY: release/binaries/clean
release/binaries/clean: ## Clean the release folder
	@echo "Cleaning the release folder"
	rm -rf release

.PHONY: release/images/build
release/images/build: ## Build images for important components
	docker bake -f release.docker-bake.hcl images

.PHONY: release/images/push
release/images/push: ## Push Docker images to Docker Hub (jenkins)
	docker bake -f release.docker-bake.hcl images --push

.PHONY: release/images/clean
release/images/clean: ## Remove all images
	-docker rmi storjlabs/segment-verify:${TAG}${CUSTOMTAG}
	-docker rmi storjlabs/jobq:${TAG}${CUSTOMTAG}
	-docker rmi storjlabs/multinode:${TAG}${CUSTOMTAG}
	-docker rmi storjlabs/satellite:${TAG}${CUSTOMTAG}
	-docker rmi storjlabs/versioncontrol:${TAG}${CUSTOMTAG}
	-docker rmi img.dev.storj.io/dev/storagenode:${TAG}${CUSTOMTAG}

##@ Clean

.PHONY: clean
clean: release/binaries/clean release/images/clean ## Clean docker test environment, local release binaries, and local Docker images
