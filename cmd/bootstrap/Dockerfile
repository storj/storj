ARG CGO_ENABLED=1
ARG REPOSITORY=../storj.io/storj
ARG PACKAGE=storj.io/storj/cmd/bootstrap
FROM storjlabs/golang as build-env

# final stage
FROM alpine
ENV CONF_PATH=/root/.local/share/storj/bootstrap
EXPOSE 28967
WORKDIR /app
VOLUME /root/.local/share/storj/bootstrap
COPY --from=build-env /app /app/bootstrap
COPY cmd/bootstrap/entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
