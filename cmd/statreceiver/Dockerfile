#Statreceiver Dockerfile
ARG DOCKER_ARCH
FROM golang:1.12.5-stretch
LABEL maintainer="Timothy Adams <tim@storj.io>"
ARG TAG
ARG GOARCH
ENV GOARCH ${GOARCH}

EXPOSE 9000
EXPOSE 2003
#Note: other ports seen in use on statreceiver still WIP
#9001,9002,2003,5432,42441

WORKDIR /app

# Install lua
RUN apt-get -y update && apt-get -y install lua5.3

# Add statreceiver
COPY release/${TAG}/statreceiver_linux_${GOARCH:-amd64} /app/statreceiver
COPY entrypoint ./entrypoint
COPY config.lua ./config.lua 

ENTRYPOINT ["/entrypoint"]
