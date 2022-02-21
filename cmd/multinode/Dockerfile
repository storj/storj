ARG DOCKER_ARCH

# Fetch ca-certificates file for arch independent builds below
FROM alpine as ca-cert
RUN apk -U add ca-certificates

FROM ${DOCKER_ARCH:-amd64}/alpine
ARG TAG
ARG GOARCH
ENV GOARCH ${GOARCH}
EXPOSE 15002
WORKDIR /app
COPY --from=ca-cert /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY release/${TAG}/multinode_linux_${GOARCH:-amd64} /app/multinode
COPY cmd/multinode/entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
