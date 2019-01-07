ARG CGO_ENABLED=1
ARG REPOSITORY=../storj.io/storj
ARG PACKAGE=storj.io/storj/cmd/satellite
FROM storjlabs/golang as build-env

# final stage
FROM alpine
ENV API_KEY= \
    CONF_PATH=/root/.local/share/storj/satellite \
    BOOTSTRAP_ADDR=bootstrap.storj.io:8080
EXPOSE 7777
WORKDIR /app
VOLUME /root/.local/share/storj/satellite
COPY --from=build-env /app /app/satellite
COPY cmd/satellite/entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
