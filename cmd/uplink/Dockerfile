ARG CGO_ENABLED=1
ARG REPOSITORY=../storj.io/storj
ARG PACKAGE=storj.io/storj/cmd/uplink
FROM storjlabs/golang as build-env

# final stage
FROM alpine
ENV CONF_PATH=/root/.local/storj/uplink \
    API_KEY= \
    SATELLITE_ADDR=
EXPOSE 7777
WORKDIR /app
VOLUME /root/.local/storj/uplink
COPY --from=build-env /app /app/uplink
COPY cmd/uplink/entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
