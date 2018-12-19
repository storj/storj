FROM storjlabs-binaries

ENV CONF_PATH=/root/.local/share/storj/uplink \
    API_KEY= \
    SATELLITE_ADDR=
EXPOSE 7776/udp \
       7777

WORKDIR /app
RUN ls /app
ENTRYPOINT ["/entrypoint/gateway"]
