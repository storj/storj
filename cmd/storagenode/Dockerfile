ARG DOCKER_ARCH
# Storagenode UI static asset generation
FROM node:10.15.1 as ui
WORKDIR /app
COPY web/storagenode/ /app
# Need to clean up (or ignore) local folders like node_modules, etc...
RUN npm install
RUN npm run build
FROM ${DOCKER_ARCH:-amd64}/alpine
ARG TAG
ARG GOARCH
ENV GOARCH ${GOARCH}
EXPOSE 28967
WORKDIR /app
VOLUME /root/.local/share/storj/storagenode
COPY --from=ui /app/static /app/static
COPY --from=ui /app/dist /app/dist
COPY resources/certs.pem /etc/ssl/certs/ca-certificates.crt
COPY release/${TAG}/storagenode_linux_${GOARCH:-amd64} /app/storagenode
COPY cmd/storagenode/entrypoint /entrypoint
COPY cmd/storagenode/dashboard.sh /app/dashboard.sh
ENTRYPOINT ["/entrypoint"]

# Remove after the alpha
ENV ADDRESS="" \
    EMAIL="" \
    WALLET="" \
    BANDWIDTH="2.0TB" \
    STORAGE="2.0TB"
