ARG DOCKER_ARCH
ARG TAG
ARG GOARCH

FROM google/cloud-sdk:latest
ENV GOARCH ${GOARCH}
ENV PATH=$PATH:/app
EXPOSE 5999
WORKDIR /app
COPY release/${TAG}/segment-verify_linux_${GOARCH:-amd64} /app/segment-verify
#COPY segment-verify /app/segment-verify
COPY entrypoint cmd/tools/segment-verify/entrypoint
ENTRYPOINT ["/app/entrypoint"]
