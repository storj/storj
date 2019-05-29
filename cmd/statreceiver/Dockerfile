#statreceiver Dockerfile
ARG DOCKER_ARCH
FROM ${DOCKER_ARCH:-amd64}/alpine
ARG TAG
ARG GOARCH
ENV GOARCH ${GOARCH}
EXPOSE 9000
EXPOSE 2003
WORKDIR /app
COPY release/${TAG}/statreceiver_linux_${GOARCH:-amd64} /app/statreceiver
ENTRYPOINT /app/statreceiver --input /app/config/config.lua
