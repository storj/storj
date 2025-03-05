FROM ${DOCKER_ARCH:-amd64}/debian:bookworm-slim
ARG TAG
ARG GOARCH
ENV GOARCH ${GOARCH}
ENV PATH=$PATH:/app
EXPOSE 15781
WORKDIR /app
COPY release/${TAG}/jobq_linux_${GOARCH:-amd64} /app/jobq
ENTRYPOINT ["/app/jobq"]
