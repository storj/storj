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
    STORJ_CONSOLE_STATIC_DIR=/app \
    STORJ_CONSOLE_ADDRESS=0.0.0.0:10100
EXPOSE 7777
EXPOSE 10100
WORKDIR /app
VOLUME /root/.local/share/storj/satellite
COPY resources/certs.pem /etc/ssl/certs/ca-certificates.crt
COPY --from=satellite-ui /app/ /app
COPY --from=build-env /app /app/satellite
COPY /cmd/satellite/entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
