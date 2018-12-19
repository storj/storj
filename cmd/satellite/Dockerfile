FROM storjlabs-binaries
ENV API_KEY= \
    CONF_PATH=/root/.local/share/storj/satellite \
    BOOTSTRAP_ADDR=bootstrap.storj.io:8080
EXPOSE 7776/udp \
       7777 \
       8080

WORKDIR /app
ENTRYPOINT ["/entrypoint/satellite"]
