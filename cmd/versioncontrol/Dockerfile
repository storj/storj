ARG DOCKER_ARCH
FROM ${DOCKER_ARCH:-amd64}/debian:buster-slim
ARG TAG
ARG GOARCH
ENV GOARCH ${GOARCH}
EXPOSE 8080
WORKDIR /app
COPY release/${TAG}/versioncontrol_linux_${GOARCH:-amd64} /app/versioncontrol
COPY cmd/versioncontrol/entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
