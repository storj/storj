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
COPY release/${TAG}/uplink_linux_${GOARCH:-amd64} /app/uplink
COPY cmd/uplink/entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
