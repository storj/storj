FROM google/cloud-sdk:latest
ARG TAG
ARG GOARCH
ENV GOARCH ${GOARCH}
ENV PATH=$PATH:/app
WORKDIR /app
COPY release/*/segment-verify_linux_${GOARCH:-amd64} /app/segment-verify
COPY cmd/tools/segment-verify/entrypoint /app/entrypoint
ENTRYPOINT ["/app/entrypoint"]
