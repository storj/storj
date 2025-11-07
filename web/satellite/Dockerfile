FROM debian:bookworm AS build
ENV NODE_MAJOR=20
RUN apt-get update && apt-get install -y ca-certificates curl gnupg
RUN mkdir -p /etc/apt/keyrings
RUN curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key | gpg --dearmor -o /etc/apt/keyrings/nodesource.gpg
ENV NODE_MAJOR=24
RUN echo "deb [signed-by=/etc/apt/keyrings/nodesource.gpg] https://deb.nodesource.com/node_$NODE_MAJOR.x nodistro main" | tee /etc/apt/sources.list.d/nodesource.list
RUN apt-get update && DEBIAN_FRONTEND="noninteractive" apt-get install -y brotli unzip libuv1-dev libjson-c-dev nettle-dev nodejs git-restore-mtime
ADD web/satellite /work
WORKDIR /work
RUN npm ci --prefer-offline --no-audit --loglevel verbose && npm run build

FROM scratch as ui
COPY --from=build /work/dist /var/www/html
COPY --from=build /work/static /var/www/static