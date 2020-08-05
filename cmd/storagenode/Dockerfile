ARG DOCKER_ARCH
ARG APK_ARCH

# Fetch ca-certificates file and supervisor apk pkg with dependecies for arch independent builds below
FROM alpine as alpine
RUN apk -U add ca-certificates
RUN echo ${APK_ARCH:-x86_64} > /etc/apk/arch
RUN mkdir /tmp/apk-supervisor
RUN apk fetch --no-cache -R -o /tmp/apk-supervisor supervisor

FROM ${DOCKER_ARCH:-amd64}/alpine
ARG TAG
ARG GOARCH
ARG VERSION_SERVER_URL
ENV GOARCH ${GOARCH:-amd64}
ENV VERSION_SERVER_URL ${VERSION_SERVER_URL:-https://version.storj.io}
EXPOSE 28967
EXPOSE 14002
COPY --from=alpine /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=alpine /tmp/apk-supervisor/* /var/lib/apk-supervisor/
COPY cmd/storagenode/entrypoint /entrypoint
COPY cmd/storagenode/dashboard.sh /app/dashboard.sh
COPY cmd/storagenode/supervisord.conf /etc/supervisord.conf
COPY cmd/storagenode/systemctl /bin/systemctl
WORKDIR /app
ENTRYPOINT ["/entrypoint"]

ENV ADDRESS="" \
    EMAIL="" \
    WALLET="" \
    STORAGE="2.0TB" \
    SETUP="false" \
    AUTO_UPDATE="true"
