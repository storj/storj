ARG DOCKER_ARCH
ARG DOCKER_PLATFORM

FROM --platform=${DOCKER_PLATFORM:-linux/amd64} storjlabs/storagenode-base:4a6c2e2a9-${DOCKER_ARCH:-amd64}
ARG TAG
ARG GOARCH
ARG VERSION_SERVER_URL
ENV GOARCH ${GOARCH:-amd64}
ENV VERSION_SERVER_URL ${VERSION_SERVER_URL:-https://version.storj.io}
EXPOSE 28967
EXPOSE 14002
# copy the files individually to avoid overriding the permissions on the folders
COPY cmd/storagenode/docker/entrypoint /entrypoint
COPY cmd/storagenode/docker/app/dashboard.sh /app/dashboard.sh
COPY cmd/storagenode/docker/bin/systemctl /bin/systemctl
WORKDIR /app
ENTRYPOINT ["/entrypoint"]

ENV ADDRESS="" \
    EMAIL="" \
    WALLET="" \
    STORAGE="2.0TB" \
    SETUP="false" \
    AUTO_UPDATE="true"
