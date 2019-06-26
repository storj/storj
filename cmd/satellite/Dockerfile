# Satellite UI static asset generation
FROM node:10.15.1 as satellite-ui
WORKDIR /app
COPY web/satellite/ /app
COPY web/marketing/ /app/marketing
# Need to clean up (or ignore) local folders like node_modules, etc...
RUN npm install
RUN npm run build

FROM alpine as ca-cert
RUN apk -U add ca-certificates

ARG DOCKER_ARCH
FROM ${DOCKER_ARCH:-amd64}/alpine
ARG TAG
ARG GOARCH
ENV GOARCH ${GOARCH}
ENV API_KEY= \
    CONF_PATH=/root/.local/share/storj/satellite \
    STORJ_CONSOLE_STATIC_DIR=/app \
    STORJ_CONSOLE_ADDRESS=0.0.0.0:10100
EXPOSE 7777
EXPOSE 10100
WORKDIR /app
COPY --from=satellite-ui /app/static /app/static
COPY --from=satellite-ui /app/dist /app/dist
COPY --from=satellite-ui /app/marketing /app/marketing
COPY --from=ca-cert /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY release/${TAG}/satellite_linux_${GOARCH:-amd64} /app/satellite
COPY cmd/satellite/entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
