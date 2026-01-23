##@ Release

# Get the short commit hash
export GIT_COMMIT_HASH ?= $(shell git rev-parse --short HEAD)
# Check if the working directory has uncommitted changes
export GIT_DIRTY  ?= $(shell test -n "`git status --porcelain`" && echo "-dirty")
# The commit hash, with -dirty if applicable, for the buildCommitHash field.
export BUILD_COMMIT_HASH := $(GIT_COMMIT_HASH)$(GIT_DIRTY)
# The last commit timestamp.
export BUILD_TIMESTAMP ?= $(shell git log -1 --format=%ct)
# Get the branch name, replacing slashes with dashes
BRANCH_NAME ?= $(shell git rev-parse --abbrev-ref HEAD | sed "s!/!-!g")

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

# Select an appropriate version:
# 1. If the current tag is an exact version ("v1.9.0") and there are no changes, it will use that.
# 2. Otherwise it will fall back to something that looks like "v0.2512.1231-dev+71831760c".
#
# TODO: Should this just use the same pseudo version that Go uses by default?
export BUILD_VERSION ?= $(shell git describe --tags --exact-match --match "v[0-9]*.[0-9]*.[0-9]*" 2>/dev/null \
	|| echo "v0.$(BUILD_MINOR_VERSION).$(BUILD_MINUTES_IN_MONTH)-dev+$(GIT_COMMIT_HASH)")

# VERSION will be used for the building process.
export VERSION ?= $(BUILD_VERSION)

# Older image tag name logic.
BRANCH_NAME ?= $(shell git rev-parse --abbrev-ref HEAD | sed "s!/!-!g")
GIT_TAG := $(shell git rev-parse --short HEAD)
ifeq (${BRANCH_NAME},main)
TAG := ${GIT_TAG}
LATEST_TAG := latest
else
TAG := ${GIT_TAG}-${BRANCH_NAME}
ifneq (,$(findstring release-,$(BRANCH_NAME)))
LATEST_TAG := ${BRANCH_NAME}-latest
endif
endif
CUSTOMTAG ?=

.PHONY: release/binaries/version
release/binaries/version: ## Script for showing the version.
	@echo "$(VERSION)"

.PHONY: release/binaries/build
release/binaries/build: ## Cross-compile everything into release folder.
	@echo "Building release binaries"
	docker bake -f release.docker-bake.hcl binaries

.PHONY: release/binaries/check-release
release/binaries/check-release: ## Check that the built binaries are releases.
	@echo "Checking release binaries"
	./scripts/release/check-release-binaries.sh "release/$(VERSION)"

.PHONY: release/binaries/sign
release/binaries/sign: ## Sign the binaries for platforms that need it.
	@echo "Signing release binaries"
	./scripts/release/windows-sign-folder.sh "release/$(VERSION)/windows_amd64"

.PHONY: release/binaries/build-installers
release/binaries/build-installers: ## Build installers for platforms that need it.
	@echo "Building installers"
	# TODO: this needs to be invoked directly from a Windows machine at the moment.
	./installer/windows/buildrelease.bat

.PHONY: release/binaries/sign-installers
release/binaries/sign-installers: ## Sign installers for platforms that need it.
	@echo "Signing installers"
	storj-sign "release/$(VERSION)/windows_amd64/storagenode.msi"

.PHONY: release/binaries/compress
release/binaries/compress: ## Compress all components into a single archive for a given platform.
	@echo "Compressing artifacts"
	# TODO: ideally this would be already done inside build-binaries part to avoid image bloat.
	# however, Windows needs the binaries to be uncompressed for signing so it complicates things a bit.
	./scripts/release/compress-binaries.sh "release/$(VERSION)"

.PHONY: release/binaries/upload-to-google-storage
release/binaries/upload-to-google-storage: ## Upload binaries to Google Storage (jenkins)
	@echo "Uploading binaries to Google Storage"
	cd "release/$(VERSION)" \
		&& gsutil -m cp -r ./*.zip sha256sums "gs://storj-v3-alpha-builds/$(VERSION)/"

.PHONY: release/binaries/publish-to-github
release/binaries/publish-to-github: ## Publish the release to github.
	@echo "Publishing release to Github"
	scripts/release/publish-to-github.sh "$(BRANCH_NAME)" "release/$(VERSION)"

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
