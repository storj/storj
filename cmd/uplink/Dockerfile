ARG DOCKER_ARCH

# Fetch ca-certificates file for arch independent builds below
FROM debian:bookworm-slim as ca-cert
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates
RUN update-ca-certificates

FROM ${DOCKER_ARCH:-amd64}/debian:bookworm-slim
ARG TAG
ARG GOARCH
ENV GOARCH ${GOARCH}
WORKDIR /app
COPY --from=ca-cert /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY release/${TAG}/uplink_linux_${GOARCH:-amd64} /app/uplink
ENTRYPOINT ["/app/uplink"]
