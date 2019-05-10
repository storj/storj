#Statreceiver Dockerfile
ARG DOCKER_ARCH
FROM golang:1.12.5-stretch
LABEL maintainer="Timothy Adams <tim@storj.io>"
ARG TAG
ARG GOARCH
ENV GOARCH ${GOARCH}

EXPOSE 9000
#Note: other ports seen in use on statreceiver
#9001,9002,2003,5432,42441

WORKDIR /app
VOLUME /root/.local/share/storj/statreceiver

# Install lua
RUN apt-get -y update && apt-get -y install lua5.3

# Add statreceiver
#COPY release/${TAG}/storagenode_linux_${GOARCH:-amd64} /app/storagenode
COPY entrypoint /entrypoint
COPY config.lua /config.lua 

ENTRYPOINT ["/entrypoint"]
