#Statreceiver Dockerfile
ARG DOCKER_ARCH
FROM golang:1.12.5-stretch
LABEL maintainer="Timothy Adams <tim@storj.io>"
ARG TAG
ARG GOARCH
ENV GOARCH ${GOARCH}

EXPOSE 9000
EXPOSE 9001
EXPOSE 9002
EXPOSE 2003
EXPOSE 5432
EXPOSE 42441

WORKDIR /app
VOLUME /root/.local/share/storj/statreceiver

# Install lua
RUN apt-get -y update && apt-get -y install lua5.3

# Add statreceiver
COPY release/${TAG}/storagenode_linux_${GOARCH:-amd64} /app/storagenode
COPY cmd/statreceiver/entrypoint /entrypoint
COPY cmd/statreceiver/config.lua /config.lua 

ENTRYPOINT ["/entrypoint"]
