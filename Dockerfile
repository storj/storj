FROM debian:stretch

COPY bin/* /app/
COPY cmd/gateway/entrypoint     /entrypoint/gateway
COPY cmd/satellite/entrypoint   /entrypoint/satellite
COPY cmd/storagenode/entrypoint /entrypoint/storagenode
COPY cmd/uplink/entrypoint      /entrypoint/uplink
WORKDIR /app

RUN ls /app