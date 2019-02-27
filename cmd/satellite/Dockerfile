ARG CGO_ENABLED=1
ARG REPOSITORY=../storj.io/storj
ARG PACKAGE=storj.io/storj/cmd/satellite
FROM storjlabs/golang as build-env
#FROM storjlabs/satellite-ui:5f00274a-philip-satelliteUIDocker-go1.11 as satellite-ui

# Satellite UI static asset generation
FROM node:10.15.1 as satellite-ui
WORKDIR /app
COPY web/satellite/ /app
# Need to clean up (or ignore) local folders like node_modules, etc...
RUN npm install
RUN npm run build

# final stage
FROM alpine
ENV API_KEY= \
    CONF_PATH=/root/.local/share/storj/satellite \
    BOOTSTRAP_ADDR=bootstrap.storj.io:8080 \
    STORJ_CONSOLE_STATIC_DIR=/app/static
EXPOSE 7777
WORKDIR /app
VOLUME /root/.local/share/storj/satellite
COPY --from=build-env /app /app/satellite
COPY cmd/satellite/entrypoint /entrypoint
COPY --from=satellite-ui /app/dist /app/static
ENTRYPOINT ["/entrypoint"]
