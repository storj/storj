ARG DOCKER_ARCH

FROM ${DOCKER_ARCH:-amd64}/alpine as alpine-arch

# Fetch ca-certificates file and supervisor apk pkg with dependecies for arch independent builds below
FROM alpine as alpine-generic
ARG APK_ARCH=x86_64
RUN apk -U add ca-certificates && cat /etc/apk/arch
RUN echo $APK_ARCH > /etc/apk/arch && cat /etc/apk/arch
RUN mkdir /tmp/apk-supervisor
RUN rm -rf /etc/apk/keys/*
COPY --from=alpine-arch /etc/apk/keys /etc/apk/keys
RUN apk fetch --no-cache -R -o /tmp/apk-supervisor supervisor

FROM alpine-arch
ARG TAG
ARG GOARCH
ARG VERSION_SERVER_URL
ENV GOARCH ${GOARCH:-amd64}
ENV VERSION_SERVER_URL ${VERSION_SERVER_URL:-https://version.storj.io}
EXPOSE 28967
EXPOSE 14002
COPY --from=alpine-generic /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=alpine-generic /tmp/apk-supervisor/* /var/lib/apk-supervisor/
COPY cmd/storagenode/docker/ /
WORKDIR /app
ENTRYPOINT ["/entrypoint"]

ENV ADDRESS="" \
    EMAIL="" \
    WALLET="" \
    STORAGE="2.0TB" \
    SETUP="false" \
    AUTO_UPDATE="true"
