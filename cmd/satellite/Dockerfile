ARG DOCKER_ARCH
# Satellite UI static asset generation
FROM node:16.11.1 as ui
WORKDIR /app
COPY web/satellite/ /app
# Need to clean up (or ignore) local folders like node_modules, etc...
RUN npm install
RUN npm run build

# Fetch ca-certificates file for arch independent builds below
FROM debian:buster-slim as ca-cert
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates
RUN update-ca-certificates

FROM ${DOCKER_ARCH:-amd64}/debian:buster-slim
ARG TAG
ARG GOARCH
ENV GOARCH ${GOARCH}
ENV CONF_PATH=/root/.local/share/storj/satellite \
    STORJ_CONSOLE_STATIC_DIR=/app \
    STORJ_CONSOLE_ADDRESS=0.0.0.0:10100
EXPOSE 7777
EXPOSE 10100
WORKDIR /app
COPY --from=ui /app/static /app/static
COPY --from=ui /app/dist /app/dist
COPY --from=ca-cert /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY release/${TAG}/wasm/access.wasm /app/static/wasm/
COPY release/${TAG}/wasm/wasm_exec.js /app/static/wasm/
COPY release/${TAG}/wasm/access.wasm.br /app/static/wasm/
COPY release/${TAG}/wasm/wasm_exec.js.br /app/static/wasm/
COPY release/${TAG}/satellite_linux_${GOARCH:-amd64} /app/satellite
COPY cmd/satellite/entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
