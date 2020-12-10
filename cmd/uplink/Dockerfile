# Fetch ca-certificates file for arch independent builds below
FROM alpine as ca-cert
RUN apk -U add ca-certificates

ARG DOCKER_ARCH
FROM ${DOCKER_ARCH:-amd64}/alpine
ARG TAG
ARG GOARCH
ENV GOARCH ${GOARCH}

ENV CONF_PATH=/root/.local/storj/uplink \
    API_KEY= \
    SATELLITE_ADDR=
WORKDIR /app
VOLUME /root/.local/storj/uplink
COPY --from=ca-cert /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY release/${TAG}/uplink_linux_${GOARCH:-amd64} /app/uplink
COPY cmd/uplink/entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
