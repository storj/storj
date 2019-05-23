ARG DOCKER_ARCH
FROM ${DOCKER_ARCH:-amd64}/alpine
ARG TAG
ARG GOARCH
ENV GOARCH ${GOARCH}
EXPOSE 28967
WORKDIR /app
COPY release/${TAG}/bootstrap_linux_${GOARCH:-amd64} /app/bootstrap
COPY cmd/bootstrap/entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
