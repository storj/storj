ARG CGO_ENABLED=1
ARG REPOSITORY=../storj.io/storj
ARG PACKAGE=storj.io/storj/cmd/storagenode
FROM brimstone/golang as build-env

# final stage
FROM alpine
ENV CONF_PATH= \
    SATELLITE_ADDR=
EXPOSE 7777

WORKDIR /app
COPY --from=build-env /app /app/storagenode
COPY cmd/storagenode/entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
