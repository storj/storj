ARG DOCKER_ARCH
FROM ${DOCKER_ARCH:-amd64}/alpine
ARG TAG
ARG GOARCH
ENV GOARCH ${GOARCH}
ENV CONF_PATH=/root/.local/share/storj/storagenode \
    IDENT_PATH=/root/.local/share/storj/identity/storagenode \
    CERT_DIFFICULTY=30 \
    SATELLITE_ADDR=
EXPOSE 28967
WORKDIR /app
VOLUME /root/.local/share/storj/storagenode
COPY resources/certs.pem /etc/ssl/certs/ca-certificates.crt
COPY release/${TAG}/storagenode_linux_${GOARCH:-amd64} /app/storagenode
COPY cmd/storagenode/entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
