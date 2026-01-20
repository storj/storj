# ca-cert downloads certificates in a separate stage to
# avoid needing to download it for different target platforms.
# Similarly, it avoids the apt cache.
FROM --platform=$BUILDPLATFORM debian:trixie-slim AS ca-cert

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates
RUN update-ca-certificates

FROM debian:trixie-slim

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

COPY --from=ca-cert /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=binaries /${TARGETOS}_${TARGETARCH}/uplink /app/uplink

ENTRYPOINT ["/app/uplink"]
