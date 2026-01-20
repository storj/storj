FROM google/cloud-sdk:latest

ARG TARGETOS
ARG TARGETARCH

ENV PATH=$PATH:/app

WORKDIR /app

COPY --from=binaries /${TARGETOS}_${TARGETARCH}/segment-verify /app/segment-verify
COPY cmd/tools/segment-verify/entrypoint /app/entrypoint

ENTRYPOINT ["/app/entrypoint"]
