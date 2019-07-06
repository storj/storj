ARG DOCKER_ARCH
FROM ${DOCKER_ARCH:-amd64}/alpine
ARG TAG
ARG GOARCH
ENV GOARCH ${GOARCH}
EXPOSE 7777
WORKDIR /app
VOLUME /root/.local/share/storj/gateway
COPY release/${TAG}/gateway_linux_${GOARCH:-amd64} /app/gateway
COPY cmd/gateway/entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
ENV CONF_PATH=/root/.local/share/storj/gateway \
    API_KEY= \
    SATELLITE_ADDR=
