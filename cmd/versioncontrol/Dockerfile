FROM debian:trixie-slim

ARG TARGETOS
ARG TARGETARCH
EXPOSE 8080

WORKDIR /app

COPY --from=binaries /${TARGETOS}_${TARGETARCH}/versioncontrol /app/versioncontrol
COPY cmd/versioncontrol/entrypoint /app/entrypoint

ENTRYPOINT ["/app/entrypoint"]
