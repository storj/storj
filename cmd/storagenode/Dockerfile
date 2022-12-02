ARG DOCKER_ARCH
ARG DOCKER_PLATFORM

FROM --platform=${DOCKER_PLATFORM:-linux/amd64} storjlabs/storagenode-base:70e276ecb-${DOCKER_ARCH:-amd64}
ARG TAG
ARG GOARCH
ARG VERSION_SERVER_URL
ARG SUPERVISOR_SERVER
ENV GOARCH ${GOARCH:-amd64}
ENV VERSION_SERVER_URL ${VERSION_SERVER_URL:-https://version.storj.io}
ENV SUPERVISOR_SERVER ${SUPERVISOR_SERVER:-unix}
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
    AUTO_UPDATE="true" \
    LOG_LEVEL=""
