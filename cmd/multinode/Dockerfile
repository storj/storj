ARG DOCKER_ARCH

# Fetch ca-certificates file for arch independent builds below
FROM debian:buster-slim as ca-cert
RUN echo "deb http://archive.debian.org/debian buster main\ndeb http://archive.debian.org/debian-security buster/updates main\ndeb http://archive.debian.org/debian buster-updates main" > /etc/apt/sources.list
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates
RUN update-ca-certificates

FROM ${DOCKER_ARCH:-amd64}/debian:buster-slim
ARG TAG
ARG GOARCH
ENV GOARCH ${GOARCH}
EXPOSE 15002
WORKDIR /app
COPY --from=ca-cert /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY release/${TAG}/multinode_linux_${GOARCH:-amd64} /app/multinode
COPY cmd/multinode/entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
