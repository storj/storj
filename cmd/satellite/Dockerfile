ARG CGO_ENABLED=1
ARG REPOSITORY=../storj.io/storj
ARG PACKAGE=storj.io/storj/cmd/satellite
FROM brimstone/golang as build-env

# final stage
FROM alpine
ENV REDIS_ADDR=redis:6379 \
    REDIS_PASSWORD= \
    REDIS_DB=0 \
    IDENTITY_ADDR=:7777 \
    HTTP_PORT=:8080 \
    BOOTSTRAP_ADDR=bootstrap.storj.io:8080
WORKDIR /app
COPY --from=build-env /app /app/satellite
COPY cmd/satellite/entrypoint /entrypoint
EXPOSE 8081/udp \
    8080 \
    7070

ENTRYPOINT ["/entrypoint"]
