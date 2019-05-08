FROM scratch
ARG TAG
ARG GOARCH
ENV GOARCH ${GOARCH}
COPY resources/certs.pem /etc/ssl/certs/ca-certificates.crt
COPY release/${TAG}/watchtower_linux_${GOARCH:-amd64} /watchtower
LABEL com.centurylinklabs.watchtower=true
ENTRYPOINT ["/watchtower"]
