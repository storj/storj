ARG CGO_ENABLED=1
ARG REPOSITORY=../storj.io/storj
ARG PACKAGE=storj.io/storj/cmd/gateway
FROM storjlabs/golang as build-env

# final stage
FROM alpine
ENV CONF_PATH=/root/.local/share/storj/gateway \
    API_KEY= \
    SATELLITE_ADDR=
EXPOSE 7777
WORKDIR /app
VOLUME /root/.local/share/storj/gateway
COPY --from=build-env /app /app/gateway
COPY cmd/gateway/entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
