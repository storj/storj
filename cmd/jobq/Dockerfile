FROM debian:trixie-slim

ARG TARGETOS
ARG TARGETARCH
EXPOSE 15781

WORKDIR /app

COPY --from=binaries /${TARGETOS}_${TARGETARCH}/jobq /app/jobq

ENTRYPOINT ["/app/jobq"]
