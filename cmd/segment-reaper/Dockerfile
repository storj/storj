ARG DOCKER_ARCH
FROM ${DOCKER_ARCH:-amd64}/alpine
ARG TAG
ARG GOARCH
ENV GOARCH ${GOARCH}

WORKDIR /app
COPY release/${TAG}/segment-reaper_linux_${GOARCH:-amd64} /app/segment-reaper
ENTRYPOINT ["/app/segment-reaper"]
