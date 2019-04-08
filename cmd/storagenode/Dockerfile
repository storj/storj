ARG CGO_ENABLED=1
ARG REPOSITORY=../storj.io/storj
ARG PACKAGE=storj.io/storj/cmd/storagenode
FROM storjlabs/golang as storagenode-build-env

# final stage
FROM alpine
ENV CONF_PATH=/root/.local/share/storj/storagenode \
    IDENT_PATH=/root/.local/share/storj/identity/storagenode \
    CERT_DIFFICULTY=30 \
    SATELLITE_ADDR=
EXPOSE 28967
WORKDIR /app
VOLUME /root/.local/share/storj/storagenode
COPY resources/certs.pem /etc/ssl/certs/ca-certificates.crt
COPY --from=storagenode-build-env /app /app/storagenode
COPY cmd/storagenode/entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
