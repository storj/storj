ARG CGO_ENABLED=1
ARG REPOSITORY=../storj.io/storj
ARG PACKAGE=storj.io/storj/cmd/uplink
FROM brimstone/golang as build-env

# final stage
FROM alpine
ENV API_KEY= \
    SATELLITE_ADDR=
EXPOSE 7777


WORKDIR /app
COPY --from=build-env /app /app/uplink
COPY cmd/uplink/entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
